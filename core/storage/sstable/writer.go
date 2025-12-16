package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
)

// Writer writes an immutable SSTable to a file.
// Writes are sequential only - no seeking is allowed.
type Writer struct {
	writer io.Writer

	// Current position in the file
	offset int64

	// Index entries (built as we write rows)
	index []IndexEntry

	// Current entity being written (for grouping rows by entity)
	currentEntity EntityID
	entityStartOffset int64

	// Statistics
	rowCount int64
}

// NewWriter creates a new SSTable writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: w,
		offset: 0,
		index:  make([]IndexEntry, 0),
		rowCount: 0,
	}
}

// WriteHeader writes the SSTable header.
// The data offset is set to immediately after the header.
func (w *Writer) WriteHeader() error {
	dataOffset := int64(HeaderSize)
	header := &Header{
		Magic:      Magic,
		Version:    Version,
		Reserved:   0,
		DataOffset: dataOffset,
	}

	data := EncodeHeader(header)
	n, err := w.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if n != HeaderSize {
		return fmt.Errorf("incomplete header write: %d bytes", n)
	}

	w.offset = int64(n)
	return nil
}

// WriteRow writes a row to the SSTable.
// Rows should be written in sorted order by entity ID for optimal performance.
func (w *Writer) WriteRow(row *Row) error {
	// Check if this is a new entity (for index building)
	if row.EntityID != w.currentEntity {
		// Save previous entity's start offset if it exists
		if w.currentEntity != "" {
			w.index = append(w.index, IndexEntry{
				EntityID: w.currentEntity,
				Offset:   w.entityStartOffset,
			})
		}
		// Start tracking new entity
		w.currentEntity = row.EntityID
		w.entityStartOffset = w.offset
	}

	// Calculate CRC32
	crc := w.calculateRowCRC(row)

	// Encode row
	rowData := w.encodeRow(row, crc)

	// Write row
	n, err := w.writer.Write(rowData)
	if err != nil {
		return fmt.Errorf("failed to write row: %w", err)
	}

	w.offset += int64(n)
	w.rowCount++

	return nil
}

// encodeRow encodes a row to bytes.
func (w *Writer) encodeRow(row *Row, crc uint32) []byte {
	entityIDBytes := []byte(row.EntityID)
	dataLen := 1 + // Type
		4 + // EntityIDLen
		len(entityIDBytes) + // EntityID
		4 + // DataLen
		len(row.Data) + // Data
		4 // CRC32

	buf := make([]byte, dataLen)
	offset := 0

	// Type
	buf[offset] = byte(row.Type)
	offset++

	// EntityIDLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(entityIDBytes)))
	offset += 4

	// EntityID
	copy(buf[offset:offset+len(entityIDBytes)], entityIDBytes)
	offset += len(entityIDBytes)

	// DataLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(row.Data)))
	offset += 4

	// Data
	copy(buf[offset:offset+len(row.Data)], row.Data)
	offset += len(row.Data)

	// CRC32
	binary.LittleEndian.PutUint32(buf[offset:offset+4], crc)

	return buf
}

// calculateRowCRC calculates the CRC32 checksum for a row.
// CRC covers: Type + EntityIDLen + EntityID + DataLen + Data
func (w *Writer) calculateRowCRC(row *Row) uint32 {
	entityIDBytes := []byte(row.EntityID)
	
	// Build data for CRC calculation
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

// WriteIndex writes the index block.
// The index must be sorted by entity ID.
func (w *Writer) WriteIndex() error {
	// Add the last entity if we have one
	if w.currentEntity != "" {
		w.index = append(w.index, IndexEntry{
			EntityID: w.currentEntity,
			Offset:   w.entityStartOffset,
		})
	}

	// Sort index by entity ID
	sort.Slice(w.index, func(i, j int) bool {
		return w.index[i].EntityID < w.index[j].EntityID
	})

	indexOffset := w.offset

	// Write all index entries
	for _, entry := range w.index {
		entryData := EncodeIndexEntry(&entry)
		n, err := w.writer.Write(entryData)
		if err != nil {
			return fmt.Errorf("failed to write index entry: %w", err)
		}
		w.offset += int64(n)
	}

	indexSize := w.offset - indexOffset

	// Write footer
	footer := &Footer{
		IndexOffset: indexOffset,
		IndexSize:   indexSize,
		RowCount:    w.rowCount,
	}

	footerData := EncodeFooter(footer)
	n, err := w.writer.Write(footerData)
	if err != nil {
		return fmt.Errorf("failed to write footer: %w", err)
	}
	if n != FooterSize {
		return fmt.Errorf("incomplete footer write: %d bytes", n)
	}

	return nil
}

// Close finalizes the SSTable by writing the index and footer.
// This method must be called after all rows have been written.
func (w *Writer) Close() error {
	return w.WriteIndex()
}

// RowCount returns the number of rows written so far.
func (w *Writer) RowCount() int64 {
	return w.rowCount
}

