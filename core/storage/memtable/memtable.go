package memtable

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// EntityID represents an entity identifier.
type EntityID string

// Row represents a row of data in the memtable.
// Implementations should provide methods to get the entity ID and estimate memory usage.
type Row interface {
	// EntityID returns the entity identifier for this row.
	EntityID() EntityID
	// Size returns the approximate memory size of the row in bytes.
	Size() int
}

// memtableData holds the actual data structure.
// This is copied on each write to enable lock-free reads.
type memtableData struct {
	data map[EntityID][]Row
	size int64 // approximate memory usage in bytes
}

// Memtable is an in-memory table with append-only semantics,
// concurrent writes, and lock-free reads.
type Memtable struct {
	// Pointer to the current memtableData, accessed atomically for lock-free reads
	dataPtr unsafe.Pointer

	// Mutex for writes (only held during write operations)
	writeMu sync.Mutex

	// Configuration
	maxSize int64 // maximum size before flush is triggered
}

// New creates a new memtable with the given maximum size threshold.
// If maxSize is 0, a default of 64MB is used.
func New(maxSize int64) *Memtable {
	if maxSize <= 0 {
		maxSize = 64 * 1024 * 1024 // 64MB default
	}

	mt := &Memtable{
		maxSize: maxSize,
	}

	// Initialize with empty data
	initialData := &memtableData{
		data: make(map[EntityID][]Row),
		size: 0,
	}
	atomic.StorePointer(&mt.dataPtr, unsafe.Pointer(initialData))

	return mt
}

// getData atomically loads the current memtableData pointer.
// This provides lock-free read access.
func (mt *Memtable) getData() *memtableData {
	ptr := atomic.LoadPointer(&mt.dataPtr)
	return (*memtableData)(ptr)
}

// Put appends a row to the memtable.
// This method is safe to call from multiple goroutines concurrently.
func (mt *Memtable) Put(row Row) {
	mt.writeMu.Lock()
	defer mt.writeMu.Unlock()

	// Get current data
	current := mt.getData()

	// Get entity ID from the row
	entityID := row.EntityID()

	// Create a new copy of the data map
	newData := &memtableData{
		data: make(map[EntityID][]Row, len(current.data)+1),
		size: current.size,
	}

	// Copy all existing entries
	for k, v := range current.data {
		// Copy the slice to ensure we have a new backing array
		rowsCopy := make([]Row, len(v))
		copy(rowsCopy, v)
		newData.data[k] = rowsCopy
	}

	// Append the new row
	rowSize := int64(row.Size())
	newData.data[entityID] = append(newData.data[entityID], row)
	newData.size += rowSize

	// Atomically update the pointer
	atomic.StorePointer(&mt.dataPtr, unsafe.Pointer(newData))
}

// Get retrieves all rows for the given entity ID.
// This method provides lock-free read access and is safe to call
// from multiple goroutines concurrently.
func (mt *Memtable) Get(entityID EntityID) []Row {
	data := mt.getData()
	rows := data.data[entityID]

	// Return a copy to prevent external modification
	if len(rows) == 0 {
		return nil
	}

	result := make([]Row, len(rows))
	copy(result, rows)
	return result
}

// ShouldFlush returns true if the memtable has exceeded its size threshold.
// This method provides lock-free read access.
func (mt *Memtable) ShouldFlush() bool {
	data := mt.getData()
	return data.size >= mt.maxSize
}

// Reset clears the memtable and resets its size counter.
// This method is safe to call from multiple goroutines, but should typically
// be called when no other operations are in progress (e.g., after flushing).
func (mt *Memtable) Reset() {
	mt.writeMu.Lock()
	defer mt.writeMu.Unlock()

	// Create new empty data
	newData := &memtableData{
		data: make(map[EntityID][]Row),
		size: 0,
	}

	// Atomically update the pointer
	atomic.StorePointer(&mt.dataPtr, unsafe.Pointer(newData))
}

// Size returns the approximate memory usage of the memtable in bytes.
// This method provides lock-free read access.
func (mt *Memtable) Size() int64 {
	data := mt.getData()
	return data.size
}

// Count returns the total number of rows in the memtable.
// This method provides lock-free read access.
func (mt *Memtable) Count() int {
	data := mt.getData()
	count := 0
	for _, rows := range data.data {
		count += len(rows)
	}
	return count
}

// EntityCount returns the number of unique entity IDs in the memtable.
// This method provides lock-free read access.
func (mt *Memtable) EntityCount() int {
	data := mt.getData()
	return len(data.data)
}

// GetAllEntities returns all entity IDs in the memtable.
// This method provides lock-free read access.
func (mt *Memtable) GetAllEntities() []EntityID {
	data := mt.getData()
	entities := make([]EntityID, 0, len(data.data))
	for entityID := range data.data {
		entities = append(entities, entityID)
	}
	return entities
}

// GetAllData returns a snapshot of all data in the memtable.
// This method provides lock-free read access and returns a copy of the data.
// Use with caution as it may be expensive for large memtables.
func (mt *Memtable) GetAllData() map[EntityID][]Row {
	data := mt.getData()
	result := make(map[EntityID][]Row, len(data.data))
	for entityID, rows := range data.data {
		// Copy the slice
		rowsCopy := make([]Row, len(rows))
		copy(rowsCopy, rows)
		result[entityID] = rowsCopy
	}
	return result
}

