package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sort"
	"sync"
)

// Reader reads from an SSTable file.
// The reader is mmap-friendly and supports concurrent reads.
type Reader struct {
	data []byte // mmap-friendly: entire file in memory

	header *Header
	footer *Footer

	// Index loaded into memory for fast lookups
	index []IndexEntry

	// Mutex for thread-safe access (only needed for index building)
	mu sync.RWMutex
}

// NewReader creates a new SSTable reader from a byte slice.
// The data should be the entire SSTable file contents (e.g., from mmap).
func NewReader(data []byte) (*Reader, error) {
	if len(data) < HeaderSize+FooterSize {
		return nil, fmt.Errorf("file too small: %d bytes", len(data))
	}

	// Read header
	header, err := DecodeHeader(data[0:HeaderSize])
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	// Read footer (last FooterSize bytes)
	footerStart := len(data) - FooterSize
	footer, err := DecodeFooter(data[footerStart:])
	if err != nil {
		return nil, fmt.Errorf("failed to decode footer: %w", err)
	}

	// Validate footer
	if footer.IndexOffset < 0 || footer.IndexOffset >= int64(len(data)) {
		return nil, fmt.Errorf("invalid index offset: %d", footer.IndexOffset)
	}
	if footer.IndexSize < 0 || footer.IndexOffset+footer.IndexSize > int64(len(data)) {
		return nil, fmt.Errorf("invalid index size: %d", footer.IndexSize)
	}

	// Load index
	indexData := data[footer.IndexOffset : footer.IndexOffset+footer.IndexSize]
	index, err := loadIndex(indexData)
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	return &Reader{
		data:    data,
		header:  header,
		footer:  footer,
		index:   index,
	}, nil
}

// loadIndex loads the index from the index section.
func loadIndex(indexData []byte) ([]IndexEntry, error) {
	index := make([]IndexEntry, 0)
	offset := 0

	for offset < len(indexData) {
		entry, newOffset, err := DecodeIndexEntry(indexData, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode index entry at offset %d: %w", offset, err)
		}
		index = append(index, *entry)
		offset = newOffset
	}

	return index, nil
}

// Get retrieves all rows for the given entity ID.
// This method is safe to call from multiple goroutines concurrently.
// Note: For optimal performance, rows should be written in sorted order by entity ID.
// If rows for the same entity are non-contiguous, only rows starting from the
// first occurrence will be returned.
func (r *Reader) Get(entityID EntityID) ([]*Row, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the entity in the index using binary search
	idx := sort.Search(len(r.index), func(i int) bool {
		return r.index[i].EntityID >= entityID
	})

	if idx >= len(r.index) || r.index[idx].EntityID != entityID {
		// Entity not found in index
		return nil, nil
	}

	// Get the offset for this entity
	offset := r.index[idx].Offset

	// Determine the end offset (start of next entity, or start of index)
	var endOffset int64
	if idx+1 < len(r.index) {
		endOffset = r.index[idx+1].Offset
	} else {
		endOffset = r.footer.IndexOffset
	}

	// Read all rows for this entity (they should be contiguous if written in sorted order)
	rows := make([]*Row, 0)
	currentOffset := offset

	for currentOffset < endOffset {
		row, nextOffset, err := r.readRowAt(currentOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to read row at offset %d: %w", currentOffset, err)
		}

		// Verify entity ID matches (safety check - stop if we hit a different entity)
		if row.EntityID != entityID {
			break // Reached next entity (shouldn't happen if data is sorted)
		}

		rows = append(rows, row)
		currentOffset = nextOffset
	}

	return rows, nil
}

// readRowAt reads a single row starting at the given offset.
// Returns the row and the offset of the next row.
func (r *Reader) readRowAt(offset int64) (*Row, int64, error) {
	if offset < 0 || offset >= int64(len(r.data)) {
		return nil, 0, fmt.Errorf("offset out of bounds: %d", offset)
	}

	data := r.data[offset:]
	if len(data) < 1+4 {
		return nil, 0, fmt.Errorf("insufficient data for row header")
	}

	row := &Row{}
	currentOffset := 0

	// Type
	row.Type = RowType(data[currentOffset])
	currentOffset++

	// EntityIDLen
	if currentOffset+4 > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for entity ID length")
	}
	entityIDLen := int(binary.LittleEndian.Uint32(data[currentOffset : currentOffset+4]))
	currentOffset += 4

	// EntityID
	if currentOffset+entityIDLen > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for entity ID")
	}
	row.EntityID = EntityID(data[currentOffset : currentOffset+entityIDLen])
	currentOffset += entityIDLen

	// DataLen
	if currentOffset+4 > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for data length")
	}
	dataLen := int(binary.LittleEndian.Uint32(data[currentOffset : currentOffset+4]))
	currentOffset += 4

	// Data
	if currentOffset+dataLen > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for row data")
	}
	row.Data = make([]byte, dataLen)
	copy(row.Data, data[currentOffset:currentOffset+dataLen])
	currentOffset += dataLen

	// CRC32
	if currentOffset+4 > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for CRC32")
	}
	row.CRC32 = binary.LittleEndian.Uint32(data[currentOffset : currentOffset+4])
	currentOffset += 4

	// Verify CRC32
	expectedCRC := r.calculateRowCRC(row)
	if row.CRC32 != expectedCRC {
		return nil, 0, fmt.Errorf("CRC32 mismatch: expected %d, got %d", expectedCRC, row.CRC32)
	}

	nextOffset := offset + int64(currentOffset)
	return row, nextOffset, nil
}

// calculateRowCRC calculates the CRC32 checksum for a row.
func (r *Reader) calculateRowCRC(row *Row) uint32 {
	entityIDBytes := []byte(row.EntityID)
	
	crcData := make([]byte, 1+4+len(entityIDBytes)+4+len(row.Data))
	offset := 0

	crcData[offset] = byte(row.Type)
	offset++

	binary.LittleEndian.PutUint32(crcData[offset:offset+4], uint32(len(entityIDBytes)))
	offset += 4

	copy(crcData[offset:offset+len(entityIDBytes)], entityIDBytes)
	offset += len(entityIDBytes)

	binary.LittleEndian.PutUint32(crcData[offset:offset+4], uint32(len(row.Data)))
	offset += 4

	copy(crcData[offset:offset+len(row.Data)], row.Data)

	return crc32.ChecksumIEEE(crcData)
}

// RowCount returns the total number of rows in the SSTable.
func (r *Reader) RowCount() int64 {
	return r.footer.RowCount
}

// EntityCount returns the number of unique entities in the SSTable.
func (r *Reader) EntityCount() int {
	return len(r.index)
}

// GetAllEntityIDs returns all entity IDs in the SSTable.
func (r *Reader) GetAllEntityIDs() []EntityID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]EntityID, len(r.index))
	for i, entry := range r.index {
		ids[i] = entry.EntityID
	}
	return ids
}

// MergeReader represents a merged view of multiple SSTable readers.
// It provides a unified interface for reading from multiple SSTables.
type MergeReader struct {
	readers []*Reader
	mu      sync.RWMutex
}

// NewMergeReader creates a new merge reader from multiple SSTable readers.
// Readers should be ordered from newest to oldest (for LSM-tree semantics).
func NewMergeReader(readers ...*Reader) *MergeReader {
	return &MergeReader{
		readers: readers,
	}
}

// Get retrieves all rows for the given entity ID from all SSTables.
// Rows are returned in order: newest first, then older.
// This method is safe to call from multiple goroutines concurrently.
func (mr *MergeReader) Get(entityID EntityID) ([]*Row, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	allRows := make([]*Row, 0)

	// Read from all readers (newest first)
	for _, reader := range mr.readers {
		rows, err := reader.Get(entityID)
		if err != nil {
			return nil, fmt.Errorf("failed to read from SSTable: %w", err)
		}
		allRows = append(allRows, rows...)
	}

	return allRows, nil
}

// RowCount returns the total number of rows across all SSTables.
func (mr *MergeReader) RowCount() int64 {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	total := int64(0)
	for _, reader := range mr.readers {
		total += reader.RowCount()
	}
	return total
}

// EntityCount returns the number of unique entities across all SSTables.
// Note: This is an approximation as entities may appear in multiple SSTables.
func (mr *MergeReader) EntityCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	entitySet := make(map[EntityID]bool)
	for _, reader := range mr.readers {
		ids := reader.GetAllEntityIDs()
		for _, id := range ids {
			entitySet[id] = true
		}
	}
	return len(entitySet)
}

