package sstable

import (
	"encoding/binary"
	"fmt"
)

// EntityID represents an entity identifier.
type EntityID string

// RowType represents the type of a row in the SSTable.
type RowType uint8

const (
	// RowTypeVector represents a vector row.
	RowTypeVector RowType = 0x01
	// RowTypeEdge represents an edge row.
	RowTypeEdge RowType = 0x02
	// RowTypeChunk represents a chunk row.
	RowTypeChunk RowType = 0x03
	// RowTypeMeta represents a metadata row.
	RowTypeMeta RowType = 0x04
)

// Magic number for SSTable files (ASCII: "SSTB")
const Magic = uint32(0x53535442)

// Version of the SSTable format.
const Version = uint16(1)

// HeaderSize is the size of the SSTable header in bytes.
const HeaderSize = 16

// FooterSize is the size of the SSTable footer in bytes.
const FooterSize = 24

// IndexEntrySize is the size of a single index entry in bytes.
const IndexEntrySize = 20 // 4 bytes entityID len + variable entityID + 8 bytes offset

// Header represents the SSTable file header.
// Layout (16 bytes):
//   - Magic (4 bytes): uint32
//   - Version (2 bytes): uint16
//   - Reserved (2 bytes): padding
//   - DataOffset (8 bytes): int64 - offset to start of data section
type Header struct {
	Magic      uint32
	Version    uint16
	Reserved   uint16 // padding
	DataOffset int64  // offset to start of data section
}

// EncodeHeader encodes a header to bytes.
func EncodeHeader(h *Header) []byte {
	buf := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	binary.LittleEndian.PutUint16(buf[6:8], h.Reserved)
	binary.LittleEndian.PutUint64(buf[8:16], uint64(h.DataOffset))
	return buf
}

// DecodeHeader decodes a header from bytes.
func DecodeHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("header data too short: %d bytes", len(data))
	}

	h := &Header{
		Magic:      binary.LittleEndian.Uint32(data[0:4]),
		Version:    binary.LittleEndian.Uint16(data[4:6]),
		Reserved:   binary.LittleEndian.Uint16(data[6:8]),
		DataOffset: int64(binary.LittleEndian.Uint64(data[8:16])),
	}

	if h.Magic != Magic {
		return nil, fmt.Errorf("invalid magic number: 0x%x", h.Magic)
	}

	if h.Version != Version {
		return nil, fmt.Errorf("unsupported version: %d", h.Version)
	}

	return h, nil
}

// Row represents a row in the SSTable.
// Layout:
//   - Type (1 byte): RowType
//   - EntityIDLen (4 bytes): uint32 - length of entity ID
//   - EntityID (variable): []byte - entity ID bytes
//   - DataLen (4 bytes): uint32 - length of row data
//   - Data (variable): []byte - row data
//   - CRC32 (4 bytes): uint32 - CRC32 checksum of (Type + EntityIDLen + EntityID + DataLen + Data)
type Row struct {
	Type     RowType
	EntityID EntityID
	Data     []byte
	CRC32    uint32
}

// RowSize returns the encoded size of a row in bytes.
func (r *Row) RowSize() int {
	return 1 + // Type
		4 + // EntityIDLen
		len(r.EntityID) + // EntityID
		4 + // DataLen
		len(r.Data) + // Data
		4 // CRC32
}

// Footer represents the SSTable file footer.
// Layout (24 bytes):
//   - IndexOffset (8 bytes): int64 - offset to start of index section
//   - IndexSize (8 bytes): int64 - size of index section in bytes
//   - RowCount (8 bytes): int64 - total number of rows
type Footer struct {
	IndexOffset int64 // offset to start of index section
	IndexSize   int64 // size of index section in bytes
	RowCount    int64 // total number of rows
}

// EncodeFooter encodes a footer to bytes.
func EncodeFooter(f *Footer) []byte {
	buf := make([]byte, FooterSize)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(f.IndexOffset))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(f.IndexSize))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(f.RowCount))
	return buf
}

// DecodeFooter decodes a footer from bytes.
func DecodeFooter(data []byte) (*Footer, error) {
	if len(data) < FooterSize {
		return nil, fmt.Errorf("footer data too short: %d bytes", len(data))
	}

	f := &Footer{
		IndexOffset: int64(binary.LittleEndian.Uint64(data[0:8])),
		IndexSize:   int64(binary.LittleEndian.Uint64(data[8:16])),
		RowCount:    int64(binary.LittleEndian.Uint64(data[16:24])),
	}

	return f, nil
}

// IndexEntry represents an entry in the index block.
// Layout (variable):
//   - EntityIDLen (4 bytes): uint32 - length of entity ID
//   - EntityID (variable): []byte - entity ID bytes
//   - Offset (8 bytes): int64 - offset to first row for this entity
type IndexEntry struct {
	EntityID EntityID
	Offset   int64
}

// IndexEntrySize returns the encoded size of an index entry in bytes.
func (e *IndexEntry) IndexEntrySize() int {
	return 4 + // EntityIDLen
		len(e.EntityID) + // EntityID
		8 // Offset
}

// EncodeIndexEntry encodes an index entry to bytes.
func EncodeIndexEntry(e *IndexEntry) []byte {
	entityIDBytes := []byte(e.EntityID)
	size := 4 + len(entityIDBytes) + 8
	buf := make([]byte, size)
	
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(entityIDBytes)))
	copy(buf[4:4+len(entityIDBytes)], entityIDBytes)
	binary.LittleEndian.PutUint64(buf[4+len(entityIDBytes):4+len(entityIDBytes)+8], uint64(e.Offset))
	
	return buf
}

// DecodeIndexEntry decodes an index entry from bytes at the given offset.
func DecodeIndexEntry(data []byte, offset int) (*IndexEntry, int, error) {
	if offset+4 > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for entity ID length")
	}

	entityIDLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	if offset+entityIDLen > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for entity ID")
	}

	entityID := EntityID(data[offset : offset+entityIDLen])
	offset += entityIDLen

	if offset+8 > len(data) {
		return nil, 0, fmt.Errorf("insufficient data for offset")
	}

	rowOffset := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	return &IndexEntry{
		EntityID: entityID,
		Offset:   rowOffset,
	}, offset, nil
}

