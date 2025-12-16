package hnsw

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Node represents a node in the HNSW graph.
type Node struct {
	ID       uint64
	Vector   []float32
	Level    int // Level in the hierarchy (0 = bottom layer)
	Neighbors [][]uint64 // Neighbors at each level
}

// nodeData holds the current graph state for lock-free reads.
type nodeData struct {
	nodes map[uint64]*Node
	entryPoint uint64 // Entry point node ID
	maxLevel int // Maximum level in the graph
}

// HNSW is a Hierarchical Navigable Small World graph index.
type HNSW struct {
	// Configuration
	M         int // Maximum number of connections per node
	ef        int // Search width parameter
	efConstruction int // ef parameter for construction
	ml        float64 // Level generation parameter (1/ln(2))

	// Sharded mutexes for writes (reduces lock contention)
	shardCount int
	shards     []*shard

	// Lock-free read data (copy-on-write)
	dataPtr unsafe.Pointer // *nodeData

	// Statistics
	nodeCount int64 // Atomic counter
}

// shard represents a shard of the graph for write operations.
type shard struct {
	mu    sync.RWMutex
	nodes map[uint64]*Node
}

// Config holds configuration for the HNSW index.
type Config struct {
	M              int     // Maximum connections per node (default: 16)
	Ef             int     // Search width (default: 200)
	EfConstruction int     // Construction search width (default: 200)
	ShardCount     int     // Number of shards for write locks (default: 16)
}

// DefaultConfig returns a default HNSW configuration.
func DefaultConfig() Config {
	return Config{
		M:              16,
		Ef:             200,
		EfConstruction: 200,
		ShardCount:     16,
	}
}

// New creates a new HNSW index with the given configuration.
func New(config Config) *HNSW {
	if config.M <= 0 {
		config.M = 16
	}
	if config.Ef <= 0 {
		config.Ef = 200
	}
	if config.EfConstruction <= 0 {
		config.EfConstruction = 200
	}
	if config.ShardCount <= 0 {
		config.ShardCount = 16
	}

	ml := 1.0 / math.Log(2.0)

	hnsw := &HNSW{
		M:              config.M,
		ef:             config.Ef,
		efConstruction: config.EfConstruction,
		ml:             ml,
		shardCount:     config.ShardCount,
		shards:         make([]*shard, config.ShardCount),
	}

	// Initialize shards
	for i := 0; i < config.ShardCount; i++ {
		hnsw.shards[i] = &shard{
			nodes: make(map[uint64]*Node),
		}
	}

	// Initialize lock-free read data
	initialData := &nodeData{
		nodes:     make(map[uint64]*Node),
		entryPoint: 0,
		maxLevel:   -1,
	}
	atomic.StorePointer(&hnsw.dataPtr, unsafe.Pointer(initialData))

	return hnsw
}

// getShard returns the shard for a given node ID.
func (h *HNSW) getShard(nodeID uint64) *shard {
	return h.shards[nodeID%uint64(h.shardCount)]
}

// getData atomically loads the current nodeData pointer for lock-free reads.
func (h *HNSW) getData() *nodeData {
	ptr := atomic.LoadPointer(&h.dataPtr)
	return (*nodeData)(ptr)
}

// updateData atomically updates the nodeData pointer (copy-on-write).
func (h *HNSW) updateData(newData *nodeData) {
	atomic.StorePointer(&h.dataPtr, unsafe.Pointer(newData))
}

// randomLevel generates a random level for a new node using the exponential distribution.
func (h *HNSW) randomLevel() int {
	level := 0
	for rand.Float64() < math.Exp(-float64(level)/h.ml) && level < 16 {
		level++
	}
	return level
}

// Add inserts a new node into the HNSW index.
// This method is thread-safe and uses sharded locks for writes.
func (h *HNSW) Add(id uint64, vector []float32) {
	level := h.randomLevel()
	node := &Node{
		ID:        id,
		Vector:    make([]float32, len(vector)),
		Level:     level,
		Neighbors: make([][]uint64, level+1),
	}
	copy(node.Vector, vector)

	// Initialize neighbor lists
	for i := 0; i <= level; i++ {
		node.Neighbors[i] = make([]uint64, 0, h.M)
	}

	// Get shard for this node
	shard := h.getShard(id)

	// Lock shard for write
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// If this is the first node, set as entry point
	data := h.getData()
	if len(data.nodes) == 0 {
		shard.nodes[id] = node
		h.updateDataWithNode(node, true)
		atomic.AddInt64(&h.nodeCount, 1)
		return
	}

	// Search for nearest neighbors at each level
	currentClosest := data.entryPoint
	ep := []uint64{currentClosest}

	// Search from top level down to level+1
	for lc := data.maxLevel; lc > level; lc-- {
		ep = h.searchLayer(data, ep, vector, 1, lc)
		if len(ep) > 0 {
			currentClosest = ep[0]
		}
	}

	// Search and connect at each level from level down to 0
	for lc := min(level, data.maxLevel); lc >= 0; lc-- {
		candidates := h.searchLayer(data, ep, vector, h.efConstruction, lc)
		neighbors := h.selectNeighbors(candidates, h.M)

		// Add node to shard
		shard.nodes[id] = node

		// Connect to neighbors (need to update neighbors in their shards)
		// Sort neighbors by shard index to avoid deadlocks
		type neighborUpdate struct {
			neighborID uint64
			shardIdx   int
		}
		updates := make([]neighborUpdate, len(neighbors))
		for i, neighborID := range neighbors {
			updates[i] = neighborUpdate{
				neighborID: neighborID,
				shardIdx:   int(neighborID % uint64(h.shardCount)),
			}
		}
		// Sort by shard index to ensure consistent lock ordering
		sort.Slice(updates, func(i, j int) bool {
			return updates[i].shardIdx < updates[j].shardIdx
		})

		// Track which shards we've locked (by index)
		lockedShardIndices := make(map[int]bool)

		for _, update := range updates {
			node.Neighbors[lc] = append(node.Neighbors[lc], update.neighborID)
			
			// Lock neighbor's shard if not already locked
			neighborShard := h.shards[update.shardIdx]
			currentShardIdx := int(id % uint64(h.shardCount))
			if update.shardIdx != currentShardIdx {
				if !lockedShardIndices[update.shardIdx] {
					neighborShard.mu.Lock()
					lockedShardIndices[update.shardIdx] = true
				}
			}
			
			// Get neighbor node from shard (more up-to-date than data)
			neighbor := neighborShard.nodes[update.neighborID]
			if neighbor == nil {
				// Fallback to data if not in shard yet
				neighborData := h.getData()
				neighbor = neighborData.nodes[update.neighborID]
			}
			
			if neighbor != nil {
				// Add bidirectional connection
				if lc < len(neighbor.Neighbors) {
					neighbor.Neighbors[lc] = append(neighbor.Neighbors[lc], id)
					
					// Prune if too many connections
					if len(neighbor.Neighbors[lc]) > h.M {
						// Create candidates with distances for pruning
						pruneCandidates := make([]candidate, len(neighbor.Neighbors[lc]))
						for i, nid := range neighbor.Neighbors[lc] {
							neighborData := h.getData()
							if n, exists := neighborData.nodes[nid]; exists {
								pruneCandidates[i] = candidate{
									id:       nid,
									distance: CosineDistance(neighbor.Vector, n.Vector),
								}
							} else {
								pruneCandidates[i] = candidate{
									id:       nid,
									distance: math.MaxFloat32,
								}
							}
						}
						neighbor.Neighbors[lc] = h.selectNeighborsWithDistances(data, pruneCandidates, neighbor.Vector, h.M)
					}
				}
			}
		}

		// Unlock all shards we locked
		for shardIdx, locked := range lockedShardIndices {
			if locked {
				h.shards[shardIdx].mu.Unlock()
			}
		}

		ep = neighbors
	}

	// Update entry point if this node is at a higher level
	if level > data.maxLevel {
		h.updateDataWithNode(node, true)
	} else {
		h.updateDataWithNode(node, false)
	}

	atomic.AddInt64(&h.nodeCount, 1)
}

// updateDataWithNode updates the lock-free read data with a new node (copy-on-write).
func (h *HNSW) updateDataWithNode(node *Node, updateEntryPoint bool) {
	oldData := h.getData()
	newData := &nodeData{
		nodes:      make(map[uint64]*Node, len(oldData.nodes)+1),
		entryPoint: oldData.entryPoint,
		maxLevel:   oldData.maxLevel,
	}

	// Copy all existing nodes
	for k, v := range oldData.nodes {
		newData.nodes[k] = v
	}

	// Add new node
	newData.nodes[node.ID] = node

	// Update entry point and max level if needed
	if updateEntryPoint || len(oldData.nodes) == 0 {
		newData.entryPoint = node.ID
	}
	if node.Level > newData.maxLevel {
		newData.maxLevel = node.Level
	}

	h.updateData(newData)
}

// searchLayer searches for the nearest neighbors in a specific layer.
func (h *HNSW) searchLayer(data *nodeData, entryPoints []uint64, query []float32, ef int, level int) []uint64 {
	if len(entryPoints) == 0 {
		return nil
	}

	visited := make(map[uint64]bool)
	candidates := make([]candidate, 0, ef*2)

	// Initialize with entry points
	for _, ep := range entryPoints {
		if node, exists := data.nodes[ep]; exists && node.Level >= level {
			dist := CosineDistance(query, node.Vector)
			candidates = append(candidates, candidate{id: ep, distance: dist})
			visited[ep] = true
		}
	}

	// Sort candidates by distance
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	// Keep only the best ef candidates
	if len(candidates) > ef {
		candidates = candidates[:ef]
	}

	// Explore neighbors
	changed := true
	for changed && len(candidates) > 0 {
		changed = false
		currentBest := candidates[0].distance

		// Check all current candidates' neighbors
		for i := 0; i < len(candidates); i++ {
			cand := candidates[i]
			if cand.distance > currentBest {
				break // No need to check further
			}

			node, exists := data.nodes[cand.id]
			if !exists || node.Level < level {
				continue
			}

			neighbors := node.Neighbors[level]
			for _, neighborID := range neighbors {
				if visited[neighborID] {
					continue
				}
				visited[neighborID] = true

				neighbor, exists := data.nodes[neighborID]
				if !exists {
					continue
				}

				dist := CosineDistance(query, neighbor.Vector)
				if dist < currentBest || len(candidates) < ef {
					candidates = append(candidates, candidate{id: neighborID, distance: dist})
					changed = true
				}
			}
		}

		// Sort and keep best ef
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].distance < candidates[j].distance
		})
		if len(candidates) > ef {
			candidates = candidates[:ef]
		}
		if len(candidates) > 0 {
			currentBest = candidates[0].distance
		}
	}

	// Extract node IDs
	result := make([]uint64, len(candidates))
	for i, cand := range candidates {
		result[i] = cand.id
	}
	return result
}

// candidate represents a candidate node with its distance.
type candidate struct {
	id       uint64
	distance float32
}

// selectNeighbors selects the best M neighbors from candidates.
// Candidates should already be sorted by distance.
func (h *HNSW) selectNeighbors(candidates []uint64, M int) []uint64 {
	if len(candidates) <= M {
		return candidates
	}
	return candidates[:M]
}

// selectNeighborsWithDistances selects the best M neighbors from candidates with distances.
func (h *HNSW) selectNeighborsWithDistances(data *nodeData, candidates []candidate, query []float32, M int) []uint64 {
	// Sort by distance
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	// Select top M
	if len(candidates) > M {
		candidates = candidates[:M]
	}

	result := make([]uint64, len(candidates))
	for i, cand := range candidates {
		result[i] = cand.id
	}
	return result
}

// Search finds the k nearest neighbors to the query vector.
// This method is lock-free and thread-safe for concurrent reads.
func (h *HNSW) Search(query []float32, k int) []uint64 {
	data := h.getData()

	if len(data.nodes) == 0 {
		return nil
	}

	// Start from entry point
	ep := []uint64{data.entryPoint}

	// Search from top level down to level 0
	for lc := data.maxLevel; lc > 0; lc-- {
		ep = h.searchLayer(data, ep, query, 1, lc)
		if len(ep) == 0 {
			return nil
		}
	}

	// Final search at level 0
	candidates := h.searchLayer(data, ep, query, h.ef, 0)

	// Return top k
	if len(candidates) > k {
		candidates = candidates[:k]
	}

	return candidates
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Size returns the current number of nodes in the index.
func (h *HNSW) Size() int64 {
	return atomic.LoadInt64(&h.nodeCount)
}

