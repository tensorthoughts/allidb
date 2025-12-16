// +build grpc

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/security/auth"
	"github.com/tensorthoughts25/allidb/core/storage"
	"github.com/tensorthoughts25/allidb/core/storage/memtable"
	"github.com/tensorthoughts25/allidb/core/storage/sstable"
	"github.com/tensorthoughts25/allidb/core/storage/wal"
)

// StorageService provides a unified interface for storage operations.
// It integrates WAL, memtable, tenant guard, and SSTable flushing.
type StorageService struct {
	wal         *wal.WAL
	memtable    *memtable.Memtable
	tenantGuard *storage.TenantGuard
	sstableDir  string

	// Flush management
	flushMu sync.Mutex
}

// NewStorageService creates a new storage service.
func NewStorageService(wal *wal.WAL, memtable *memtable.Memtable, tenantGuard *storage.TenantGuard, sstableDir string) *StorageService {
	return &StorageService{
		wal:         wal,
		memtable:    memtable,
		tenantGuard: tenantGuard,
		sstableDir:  sstableDir,
	}
}

// PutEntity stores an entity with tenant isolation and WAL logging.
func (s *StorageService) PutEntity(ctx context.Context, ent *entity.UnifiedEntity) error {
	// Validate tenant
	_, err := s.tenantGuard.RequireTenant(ctx)
	if err != nil {
		return fmt.Errorf("tenant validation failed: %w", err)
	}

	// Ensure entity tenant matches context tenant
	if err := s.tenantGuard.ValidateEntityTenant(ctx, ent.GetTenantID()); err != nil {
		return fmt.Errorf("tenant mismatch: %w", err)
	}

	// Create WAL entry
	walEntry := &EntityWALEntry{
		Entity: ent,
	}

	// Write to WAL first (for durability)
	if err := s.wal.Append(walEntry); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Write to memtable
	row := &EntityRow{
		Entity: ent,
	}
	s.memtable.Put(row)

	// Check if memtable should be flushed
	if s.memtable.ShouldFlush() {
		// FlushMemtable is an internal operation that doesn't require tenant validation
		// Use background context to avoid tenant requirement
		if err := s.FlushMemtable(context.Background()); err != nil {
			return fmt.Errorf("failed to flush memtable: %w", err)
		}
	}

	return nil
}

// GetEntity retrieves an entity by ID with tenant validation.
func (s *StorageService) GetEntity(ctx context.Context, entityID entity.EntityID) (*entity.UnifiedEntity, error) {
	// Validate tenant
	tenantID, err := s.tenantGuard.RequireTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("tenant validation failed: %w", err)
	}

	// Check memtable first
	memEnt, memTS := s.latestFromMemtable(memtable.EntityID(entityID), tenantID)

	// Check SSTables (newest first)
	sstEnt, sstTS, err := s.latestFromSSTables(memtable.EntityID(entityID), tenantID)
	if err != nil {
		return nil, fmt.Errorf("sstable lookup failed: %w", err)
	}

	// Choose the newest between memtable and SSTables
	if memEnt != nil && memTS >= sstTS {
		return memEnt, nil
	}
	if sstEnt != nil {
		return sstEnt, nil
	}
	return nil, nil
}

// FlushMemtable flushes the memtable to an SSTable.
// This is a production-ready implementation that writes all entities to an SSTable.
func (s *StorageService) FlushMemtable(ctx context.Context) error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()

	// Get all data from memtable
	data := s.memtable.GetAllData()
	if len(data) == 0 {
		// Nothing to flush
		s.memtable.Reset()
		return nil
	}

	// Create SSTable file with timestamp-based sequence
	seq := time.Now().UnixNano()
	sstablePath := filepath.Join(s.sstableDir, fmt.Sprintf("sstable-%d.sst", seq))
	file, err := os.Create(sstablePath)
	if err != nil {
		return fmt.Errorf("failed to create SSTable file: %w", err)
	}
	defer file.Close()

	// Write SSTable
	writer := sstable.NewWriter(file)
	if err := writer.WriteHeader(); err != nil {
		return fmt.Errorf("failed to write SSTable header: %w", err)
	}

	// Sort entity IDs for consistent ordering
	entityIDs := make([]memtable.EntityID, 0, len(data))
	for entityID := range data {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i, j int) bool {
		return entityIDs[i] < entityIDs[j]
	})

	// Write all rows, sorted by entity ID
	for _, entityID := range entityIDs {
		rows := data[entityID]
		for _, row := range rows {
			if entityRow, ok := row.(*EntityRow); ok {
				// Convert entity to SSTable row
				entityBytes := entityRow.Entity.ToBytes()
				sstableRow := &sstable.Row{
					Type:     sstable.RowTypeMeta,
					EntityID: sstable.EntityID(entityID),
					Data:     entityBytes,
				}
				if err := writer.WriteRow(sstableRow); err != nil {
					return fmt.Errorf("failed to write row for entity %s: %w", entityID, err)
				}
			}
		}
	}

	// Close writer (writes index and footer)
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close SSTable writer: %w", err)
	}

	// Reset memtable only after successful flush
	s.memtable.Reset()

	return nil
}

// RangeScan returns entities whose IDs are between start (inclusive) and end (exclusive).
// If end is empty, it scans to the end. If tenantID is empty, tenant filtering is skipped.
func (s *StorageService) RangeScan(ctx context.Context, start, end string, tenantID string) (map[string]*entity.UnifiedEntity, error) {
	result := make(map[string]*entity.UnifiedEntity)

	// Scan memtable
	for _, eid := range s.memtable.GetAllEntities() {
		key := string(eid)
		if !inRange(key, start, end) {
			continue
		}
		rows := s.memtable.Get(eid)
		if len(rows) == 0 {
			continue
		}
		if erow, ok := rows[len(rows)-1].(*EntityRow); ok {
			if tenantID != "" && erow.Entity.GetTenantID() != tenantID {
				continue
			}
			result[key] = erow.Entity
		}
	}

	// Scan SSTables newest-first
	pattern := filepath.Join(s.sstableDir, "sstable-*.sst")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return result, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i] > files[j] })

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		reader, err := sstable.NewReader(data)
		if err != nil {
			continue
		}
		ids := reader.GetAllEntityIDs()
		for _, id := range ids {
			key := string(id)
			if !inRange(key, start, end) {
				continue
			}
			// Only set if not already present (prefer newer files)
			if _, exists := result[key]; exists {
				continue
			}
			rows, err := reader.Get(id)
			if err != nil || len(rows) == 0 {
				continue
			}
			row := rows[len(rows)-1]
			ent := &entity.UnifiedEntity{}
			if err := ent.FromBytes(row.Data); err != nil {
				continue
			}
			if tenantID != "" && ent.GetTenantID() != tenantID {
				continue
			}
			result[key] = ent
		}
	}

	return result, nil
}

// ReplicaRead returns raw bytes for replication.
func (s *StorageService) ReplicaRead(ctx context.Context, key, tenantID string) ([]byte, error) {
	// Key format expected as tenantID:entityID
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid key format")
	}
	if tenantID != "" && parts[0] != tenantID {
		return nil, fmt.Errorf("tenant mismatch")
	}
	// Use tenantID from key if not provided
	if tenantID == "" {
		tenantID = parts[0]
	}
	// Inject tenant into context for GetEntity
	ctx = auth.WithTenantID(ctx, tenantID)
	entityID := entity.EntityID(parts[1])
	ent, err := s.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, nil
	}
	return ent.ToBytes(), nil
}

// ReplicaWrite writes raw bytes for replication.
func (s *StorageService) ReplicaWrite(ctx context.Context, key, tenantID string, value []byte) error {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid key format")
	}
	if tenantID != "" && parts[0] != tenantID {
		return fmt.Errorf("tenant mismatch")
	}
	// Use tenantID from key if not provided
	if tenantID == "" {
		tenantID = parts[0]
	}
	ent := &entity.UnifiedEntity{}
	if err := ent.FromBytes(value); err != nil {
		return err
	}
	// Ensure tenant matches key prefix
	if ent.GetTenantID() != parts[0] {
		return fmt.Errorf("tenant mismatch between key and entity")
	}
	// Inject tenant into context for validation (though we skip WAL for replica writes)
	ctx = auth.WithTenantID(ctx, tenantID)
	row := &EntityRow{Entity: ent}
	// No WAL append on replica write (assumes primary already logged)
	// Direct memtable write without tenant guard validation since this is replication
	s.memtable.Put(row)
	return nil
}

func inRange(key, start, end string) bool {
	if start != "" && key < start {
		return false
	}
	if end != "" && key >= end {
		return false
	}
	return true
}

// latestFromMemtable returns latest entity and timestamp from memtable for the given ID and tenant.
func (s *StorageService) latestFromMemtable(id memtable.EntityID, tenantID string) (*entity.UnifiedEntity, int64) {
	rows := s.memtable.Get(id)
	if len(rows) == 0 {
		return nil, 0
	}
	if entityRow, ok := rows[len(rows)-1].(*EntityRow); ok {
		ent := entityRow.Entity
		if ent.GetTenantID() != tenantID {
			return nil, 0
		}
		ts := extractEntityTimestamp(ent)
		return ent, ts
	}
	return nil, 0
}

// latestFromSSTables scans SSTables newest-first and returns the newest matching entity.
func (s *StorageService) latestFromSSTables(id memtable.EntityID, tenantID string) (*entity.UnifiedEntity, int64, error) {
	pattern := filepath.Join(s.sstableDir, "sstable-*.sst")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, 0, err
	}
	if len(files) == 0 {
		return nil, 0, nil
	}
	// Sort descending by filename (timestamp-based)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	var bestEnt *entity.UnifiedEntity
	var bestTS int64

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // skip unreadable
		}
		reader, err := sstable.NewReader(data)
		if err != nil {
			continue
		}
		rows, err := reader.Get(sstable.EntityID(id))
		if err != nil || len(rows) == 0 {
			continue
		}
		// Take the newest row in this SSTable (last written)
		row := rows[len(rows)-1]
		ent := &entity.UnifiedEntity{}
		if err := ent.FromBytes(row.Data); err != nil {
			continue
		}
		if ent.GetTenantID() != tenantID {
			continue
		}
		ts := extractRowTimestamp(row.Data)
		if ts >= bestTS {
			bestTS = ts
			bestEnt = ent
		}
		// Since files are newest-first, we can early-exit if we found one with a timestamp
	}

	return bestEnt, bestTS, nil
}

// extractRowTimestamp reads an int64 timestamp from the first 8 bytes if present.
func extractRowTimestamp(data []byte) int64 {
	if len(data) < 8 {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(data[:8]))
}

// extractEntityTimestamp attempts to read timestamp from entity's internal modified time.
// Falls back to zero if unavailable.
func extractEntityTimestamp(ent *entity.UnifiedEntity) int64 {
	if ent == nil {
		return 0
	}
	return ent.GetUpdatedAt().UnixNano()
}
// EntityRow implements memtable.Row for UnifiedEntity.
type EntityRow struct {
	Entity *entity.UnifiedEntity
}

func (r *EntityRow) EntityID() memtable.EntityID {
	return memtable.EntityID(r.Entity.GetEntityID())
}

func (r *EntityRow) Size() int {
	// Approximate size: entity ID + tenant ID + embeddings + edges + chunks
	size := len(r.Entity.GetEntityID()) + len(r.Entity.GetTenantID())
	for _, emb := range r.Entity.GetEmbeddings() {
		size += len(emb.Vector) * 4 // float32 = 4 bytes
	}
	size += len(r.Entity.GetIncomingEdges()) * 100 // rough estimate
	size += len(r.Entity.GetOutgoingEdges()) * 100
	for _, chunk := range r.Entity.GetChunks() {
		size += len(chunk.Text) + 50 // rough estimate
	}
	return size
}

// EntityWALEntry implements wal.WALEntry for UnifiedEntity.
type EntityWALEntry struct {
	Entity *entity.UnifiedEntity
}

func (e *EntityWALEntry) ToBytes() []byte {
	return e.Entity.ToBytes()
}

func (e *EntityWALEntry) FromBytes(data []byte) error {
	ent := &entity.UnifiedEntity{}
	if err := ent.FromBytes(data); err != nil {
		return err
	}
	e.Entity = ent
	return nil
}

