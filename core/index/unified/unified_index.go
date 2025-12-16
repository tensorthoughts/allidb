package unified

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/index/graph"
	"github.com/tensorthoughts25/allidb/core/index/hnsw"
	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

// EntityID is an alias for entity.EntityID
type EntityID = entity.EntityID

// UnifiedIndex is a unified vector + graph index that combines:
// - ANN search using HNSW
// - Graph-based candidate expansion
// - Entity caching
// - Hot reload on SSTable updates
type UnifiedIndex struct {
	// Core components
	hnswIndex  *hnsw.HNSW
	graphCache *graph.GraphCache
	entityCache *entityCache

	// ID mapping: EntityID <-> HNSW node ID (uint64)
	idMapping map[EntityID]uint64
	reverseMapping map[uint64]EntityID
	idMappingMu sync.RWMutex

	// SSTable management
	sstableDir    string
	sstablePrefix string
	sstableReaders []*sstable.Reader
	mergeReader   *sstable.MergeReader

	// Hot reload
	reloadMu      sync.RWMutex
	lastReload    time.Time
	reloadInterval time.Duration
	stopReload    chan struct{}
	reloadWg      sync.WaitGroup

	// Configuration
	config Config
}

// entityCache caches UnifiedEntity objects with LRU eviction.
type entityCache struct {
	cache map[EntityID]*entity.UnifiedEntity
	mu    sync.RWMutex
	maxSize int
}

// Config holds configuration for the unified index.
type Config struct {
	// HNSW configuration
	HNSWConfig hnsw.Config

	// Graph cache configuration
	GraphCacheSize int // Maximum entities in graph cache

	// Entity cache configuration
	EntityCacheSize int // Maximum entities in entity cache

	// SSTable configuration
	SSTableDir    string // Directory containing SSTable files
	SSTablePrefix string // Prefix for SSTable files (default: "sstable")

	// Hot reload configuration
	ReloadInterval time.Duration // How often to check for SSTable updates (default: 5s)
}

// DefaultConfig returns a default unified index configuration.
func DefaultConfig(sstableDir string) Config {
	return Config{
		HNSWConfig:      hnsw.DefaultConfig(),
		GraphCacheSize:  10000,
		EntityCacheSize: 1000,
		SSTableDir:      sstableDir,
		SSTablePrefix:   "sstable",
		ReloadInterval:  5 * time.Second,
	}
}

// New creates a new unified index with the given configuration.
func New(config Config) (*UnifiedIndex, error) {
	// Create HNSW index
	hnswIndex := hnsw.New(config.HNSWConfig)

	// Create graph cache
	graphCache := graph.NewGraphCache(config.GraphCacheSize)

	// Create entity cache
	entityCache := &entityCache{
		cache:   make(map[EntityID]*entity.UnifiedEntity),
		maxSize: config.EntityCacheSize,
	}

	idx := &UnifiedIndex{
		hnswIndex:      hnswIndex,
		graphCache:     graphCache,
		entityCache:    entityCache,
		idMapping:      make(map[EntityID]uint64),
		reverseMapping: make(map[uint64]EntityID),
		sstableDir:     config.SSTableDir,
		sstablePrefix:  config.SSTablePrefix,
		sstableReaders: make([]*sstable.Reader, 0),
		lastReload:     time.Time{},
		reloadInterval: config.ReloadInterval,
		stopReload:     make(chan struct{}),
		config:         config,
	}

	// Initial load
	if err := idx.reloadSSTables(); err != nil {
		return nil, fmt.Errorf("failed to load initial SSTables: %w", err)
	}

	// Start hot reload goroutine
	idx.reloadWg.Add(1)
	go idx.hotReloadLoop()

	return idx, nil
}

// reloadSSTables reloads all SSTable files from disk.
func (idx *UnifiedIndex) reloadSSTables() error {
	idx.reloadMu.Lock()
	defer idx.reloadMu.Unlock()

	// Find all SSTable files
	pattern := filepath.Join(idx.sstableDir, idx.sstablePrefix+"-*.sst")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob SSTable files: %w", err)
	}

	// Sort by filename (which should include sequence number)
	sort.Strings(matches)

	// Load SSTable files
	newReaders := make([]*sstable.Reader, 0, len(matches))
	for _, filePath := range matches {
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Log error but continue with other files
			continue
		}

		reader, err := sstable.NewReader(data)
		if err != nil {
			// Log error but continue with other files
			continue
		}

		newReaders = append(newReaders, reader)
	}

	// Update readers (newest first for LSM-tree semantics)
	idx.sstableReaders = newReaders
	if len(newReaders) > 0 {
		// Reverse to get newest first
		for i, j := 0, len(newReaders)-1; i < j; i, j = i+1, j-1 {
			newReaders[i], newReaders[j] = newReaders[j], newReaders[i]
		}
		idx.mergeReader = sstable.NewMergeReader(newReaders...)
	} else {
		idx.mergeReader = nil
	}

	// Rebuild HNSW index and graph cache from SSTables
	if err := idx.rebuildIndexes(); err != nil {
		return fmt.Errorf("failed to rebuild indexes: %w", err)
	}

	idx.lastReload = time.Now()
	return nil
}

// rebuildIndexes rebuilds the HNSW index and graph cache from SSTables.
func (idx *UnifiedIndex) rebuildIndexes() error {
	if idx.mergeReader == nil {
		return nil
	}

	// Create new HNSW index
	idx.hnswIndex = hnsw.New(idx.config.HNSWConfig)

	// Clear graph cache
	idx.graphCache.Clear()

	// Clear ID mappings
	idx.idMappingMu.Lock()
	idx.idMapping = make(map[EntityID]uint64)
	idx.reverseMapping = make(map[uint64]EntityID)
	idx.idMappingMu.Unlock()

	// Clear entity cache
	idx.entityCache.mu.Lock()
	idx.entityCache.cache = make(map[EntityID]*entity.UnifiedEntity)
	idx.entityCache.mu.Unlock()

	// Get all entity IDs from SSTables
	entitySet := make(map[EntityID]bool)
	for _, reader := range idx.sstableReaders {
		ids := reader.GetAllEntityIDs()
		for _, id := range ids {
			entitySet[EntityID(id)] = true
		}
	}

	// Load entities and build indexes
	for entityID := range entitySet {
		rows, err := idx.mergeReader.Get(sstable.EntityID(entityID))
		if err != nil {
			continue
		}

		// Process rows to extract vectors and edges
		var latestVector []float32
		var vectorID uint64

		for _, row := range rows {
			switch row.Type {
			case sstable.RowTypeVector:
				// Decode vector (assuming it's stored as float32 array)
				vector, id, err := idx.decodeVector(row.Data)
				if err == nil {
					latestVector = vector
					vectorID = id
				}

			case sstable.RowTypeEdge:
				// Decode edge
				edge, err := idx.decodeEdge(row.Data)
				if err == nil {
					idx.graphCache.AddEdge(edge)
				}
			}
		}

		// Add vector to HNSW if we have one
		if latestVector != nil && vectorID != 0 {
			idx.hnswIndex.Add(vectorID, latestVector)
			// Update ID mapping
			idx.idMappingMu.Lock()
			idx.idMapping[entityID] = vectorID
			idx.reverseMapping[vectorID] = entityID
			idx.idMappingMu.Unlock()
		}
	}

	return nil
}

// decodeVector decodes a vector from SSTable row data.
// Format: [8 bytes ID][4 bytes vector len][vector data as float32]
func (idx *UnifiedIndex) decodeVector(data []byte) ([]float32, uint64, error) {
	if len(data) < 12 {
		return nil, 0, fmt.Errorf("insufficient data for vector")
	}

	id := binary.LittleEndian.Uint64(data[0:8])
	vectorLen := int(binary.LittleEndian.Uint32(data[8:12]))

	if len(data) < 12+vectorLen*4 {
		return nil, 0, fmt.Errorf("insufficient data for vector content")
	}

	vector := make([]float32, vectorLen)
	for i := 0; i < vectorLen; i++ {
		offset := 12 + i*4
		bits := binary.LittleEndian.Uint32(data[offset : offset+4])
		vector[i] = math.Float32frombits(bits)
	}

	return vector, id, nil
}

// decodeEdge decodes a graph edge from SSTable row data.
func (idx *UnifiedIndex) decodeEdge(data []byte) (entity.GraphEdge, error) {
	edge := entity.GraphEdge{}
	err := edge.FromBytes(data)
	return edge, err
}

// hotReloadLoop periodically checks for SSTable updates and reloads.
func (idx *UnifiedIndex) hotReloadLoop() {
	defer idx.reloadWg.Done()

	ticker := time.NewTicker(idx.reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-idx.stopReload:
			return
		case <-ticker.C:
			// Check if SSTables have been updated
			if idx.shouldReload() {
				if err := idx.reloadSSTables(); err != nil {
					// Log error but continue
					_ = err
				}
			}
		}
	}
}

// shouldReload checks if SSTables need to be reloaded.
func (idx *UnifiedIndex) shouldReload() bool {
	// Check if any SSTable files have been modified since last reload
	pattern := filepath.Join(idx.sstableDir, idx.sstablePrefix+"-*.sst")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return false
	}

	for _, filePath := range matches {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		if info.ModTime().After(idx.lastReload) {
			return true
		}
	}

	return false
}

// Search performs a unified search: ANN first, then graph-based expansion.
// Returns entity IDs sorted by relevance.
func (idx *UnifiedIndex) Search(query []float32, k int, expandFactor int) ([]EntityID, error) {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()

	// Step 1: ANN search using HNSW
	annCandidates := idx.hnswIndex.Search(query, k*expandFactor)
	if len(annCandidates) == 0 {
		return nil, nil
	}

	// Step 2: Graph-based candidate expansion
	expandedCandidates := make(map[EntityID]bool)

	// Add ANN candidates (convert HNSW node IDs to EntityIDs)
	idx.idMappingMu.RLock()
	for _, candidateID := range annCandidates {
		if entityID, exists := idx.reverseMapping[candidateID]; exists {
			expandedCandidates[entityID] = true

			// Expand via graph neighbors
			neighbors := idx.graphCache.GetNeighbors(entityID)
			for _, neighbor := range neighbors {
				expandedCandidates[neighbor] = true
			}
		}
	}
	idx.idMappingMu.RUnlock()

	// Step 3: Score and rank candidates
	type scoredCandidate struct {
		entityID EntityID
		score    float64
	}

	candidates := make([]scoredCandidate, 0, len(expandedCandidates))
	for entityID := range expandedCandidates {
		// Get entity to compute score
		ent := idx.getEntity(entityID)
		if ent == nil {
			continue
		}

		// Get latest embedding
		embedding := ent.GetLatestEmbedding()
		if embedding == nil {
			continue
		}

		// Compute cosine distance
		distance := hnsw.CosineDistance(query, embedding.Vector)
		score := 1.0 / (1.0 + float64(distance)) // Convert distance to score

		// Weight by importance
		importance := ent.GetImportance()
		score *= (1.0 + importance)

		candidates = append(candidates, scoredCandidate{
			entityID: entityID,
			score:    score,
		})
	}

	// Sort by score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Return top k
	result := make([]EntityID, 0, k)
	for i := 0; i < len(candidates) && i < k; i++ {
		result = append(result, candidates[i].entityID)
	}

	return result, nil
}

// getEntity retrieves an entity, using cache if available.
func (idx *UnifiedIndex) getEntity(entityID EntityID) *entity.UnifiedEntity {
	// Check cache first
	idx.entityCache.mu.RLock()
	if cached, exists := idx.entityCache.cache[entityID]; exists {
		idx.entityCache.mu.RUnlock()
		return cached
	}
	idx.entityCache.mu.RUnlock()

	// Load from SSTables
	if idx.mergeReader == nil {
		return nil
	}

	rows, err := idx.mergeReader.Get(sstable.EntityID(entityID))
	if err != nil || len(rows) == 0 {
		return nil
	}

	// Reconstruct entity from rows
	ent := idx.reconstructEntity(entityID, rows)
	if ent == nil {
		return nil
	}

	// Cache entity
	idx.entityCache.mu.Lock()
	if len(idx.entityCache.cache) >= idx.entityCache.maxSize {
		// Simple eviction: remove a random entry (could use LRU)
		for k := range idx.entityCache.cache {
			delete(idx.entityCache.cache, k)
			break
		}
	}
	idx.entityCache.cache[entityID] = ent
	idx.entityCache.mu.Unlock()

	return ent
}

// reconstructEntity reconstructs a UnifiedEntity from SSTable rows.
func (idx *UnifiedIndex) reconstructEntity(entityID EntityID, rows []*sstable.Row) *entity.UnifiedEntity {
	// Group rows by type; prefer metadata rows to seed tenant/type/importance.
	ent := entity.NewUnifiedEntity(entityID, "", "")

	// Process rows in order (newest first from merge reader)
	for _, row := range rows {
		switch row.Type {
		case sstable.RowTypeVector:
			vector, _, err := idx.decodeVector(row.Data)
			if err == nil {
				ent.AddEmbedding(vector)
			}

		case sstable.RowTypeEdge:
			edge, err := idx.decodeEdge(row.Data)
			if err == nil {
				if edge.ToEntity == entityID {
					ent.AddIncomingEdge(edge)
				} else if edge.FromEntity == entityID {
					ent.AddOutgoingEdge(edge)
				}
			}

		case sstable.RowTypeChunk:
			chunk := entity.TextChunk{}
			if err := chunk.FromBytes(row.Data); err == nil {
				ent.AddChunk(chunk)
			}

		case sstable.RowTypeMeta:
			// Decode full entity metadata (tenant, type, importance, timestamps, cached vectors/edges/chunks).
			var decoded entity.UnifiedEntity
			if err := decoded.FromBytes(row.Data); err == nil {
				ent = &decoded
			}
		}
	}

	return ent
}

// Close stops the hot reload loop and cleans up resources.
// This method is safe to call multiple times.
func (idx *UnifiedIndex) Close() error {
	select {
	case <-idx.stopReload:
		// Already closed
		return nil
	default:
		close(idx.stopReload)
	}
	idx.reloadWg.Wait()
	return nil
}

// Size returns the current number of entities in the index.
func (idx *UnifiedIndex) Size() int64 {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	return idx.hnswIndex.Size()
}

// GetEntity retrieves an entity by ID.
func (idx *UnifiedIndex) GetEntity(entityID EntityID) *entity.UnifiedEntity {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	return idx.getEntity(entityID)
}

// GetNeighbors returns all neighbors (incoming + outgoing) for an entity.
func (idx *UnifiedIndex) GetNeighbors(entityID EntityID) []EntityID {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	return idx.graphCache.GetNeighbors(entityID)
}

// GetOutgoingNeighbors returns entities that this entity points to.
func (idx *UnifiedIndex) GetOutgoingNeighbors(entityID EntityID) []EntityID {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	return idx.graphCache.GetOutgoingNeighbors(entityID)
}

// GetIncomingNeighbors returns entities that point to this entity.
func (idx *UnifiedIndex) GetIncomingNeighbors(entityID EntityID) []EntityID {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	return idx.graphCache.GetIncomingNeighbors(entityID)
}

// GetOutgoingEdges returns all outgoing edges for an entity.
func (idx *UnifiedIndex) GetOutgoingEdges(entityID EntityID) []entity.GraphEdge {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	return idx.graphCache.GetOutgoingEdges(entityID)
}

// GetAllEntityIDs returns all entity IDs from SSTables.
// This method is useful for building Merkle trees and other operations that need
// to enumerate all entities.
func (idx *UnifiedIndex) GetAllEntityIDs() []EntityID {
	idx.reloadMu.RLock()
	defer idx.reloadMu.RUnlock()
	
	entitySet := make(map[EntityID]bool)
	
	// Get entity IDs from all SSTable readers
	for _, reader := range idx.sstableReaders {
		ids := reader.GetAllEntityIDs()
		for _, id := range ids {
			entitySet[EntityID(id)] = true
		}
	}
	
	// Convert set to slice
	result := make([]EntityID, 0, len(entitySet))
	for entityID := range entitySet {
		result = append(result, entityID)
	}
	
	return result
}

// AddEntity adds an entity to the index immediately (for real-time indexing).
// This is useful for demo purposes and ensures entities are searchable right after write.
// In production, entities are typically indexed via SSTable hot reload.
func (idx *UnifiedIndex) AddEntity(ent *entity.UnifiedEntity) error {
	idx.reloadMu.Lock()
	defer idx.reloadMu.Unlock()

	entityID := EntityID(ent.GetEntityID())

	// Get latest embedding
	embedding := ent.GetLatestEmbedding()
	if embedding == nil || len(embedding.Vector) == 0 {
		// No vector to index, but we can still cache the entity and add edges
	} else {
		// Generate a unique HNSW node ID from entity ID (using hash)
		// This ensures consistent mapping between entity ID and HNSW node ID
		vectorID := idx.hashEntityID(entityID)

		// Check if already indexed
		idx.idMappingMu.RLock()
		_, exists := idx.idMapping[entityID]
		idx.idMappingMu.RUnlock()

		if !exists {
			// Add vector to HNSW
			idx.hnswIndex.Add(vectorID, embedding.Vector)

			// Update ID mapping
			idx.idMappingMu.Lock()
			idx.idMapping[entityID] = vectorID
			idx.reverseMapping[vectorID] = entityID
			idx.idMappingMu.Unlock()
		}
	}

	// Add edges to graph cache
	for _, edge := range ent.GetOutgoingEdges() {
		idx.graphCache.AddEdge(edge)
	}
	for _, edge := range ent.GetIncomingEdges() {
		idx.graphCache.AddEdge(edge)
	}

	// Cache entity
	idx.entityCache.mu.Lock()
	if len(idx.entityCache.cache) >= idx.entityCache.maxSize {
		// Simple eviction: remove a random entry
		for k := range idx.entityCache.cache {
			delete(idx.entityCache.cache, k)
			break
		}
	}
	idx.entityCache.cache[entityID] = ent
	idx.entityCache.mu.Unlock()

	return nil
}

// hashEntityID generates a consistent uint64 hash from an EntityID.
// This is used to map EntityID to HNSW node ID.
func (idx *UnifiedIndex) hashEntityID(entityID EntityID) uint64 {
	// Simple hash function (FNV-1a style)
	hash := uint64(2166136261)
	for _, b := range []byte(entityID) {
		hash ^= uint64(b)
		hash *= 16777619
	}
	return hash
}

