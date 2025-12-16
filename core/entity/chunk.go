package entity

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// ChunkID represents a unique identifier for a text chunk.
type ChunkID string

// TextChunk represents a chunk of text with associated metadata.
type TextChunk struct {
	ChunkID  ChunkID
	Text     string
	Metadata map[string]string
}

// Size returns an estimate of the memory size of the chunk in bytes.
func (c *TextChunk) Size() int {
	size := 0
	size += len(c.ChunkID) + 8 // ChunkID string overhead
	size += len(c.Text) + 8    // Text string overhead
	
	// Metadata size
	size += 24 // map overhead
	for k, v := range c.Metadata {
		size += len(k) + len(v) + 16 // key + value + overhead
	}
	
	return size
}

// ToBytes serializes the TextChunk to binary format.
// Format:
//   - ChunkIDLen (4 bytes): uint32
//   - ChunkID (variable): []byte
//   - TextLen (4 bytes): uint32
//   - Text (variable): []byte
//   - MetadataCount (4 bytes): uint32
//   - For each metadata entry:
//     - KeyLen (4 bytes): uint32
//     - Key (variable): []byte
//     - ValueLen (4 bytes): uint32
//     - Value (variable): []byte
func (c *TextChunk) ToBytes() []byte {
	chunkIDBytes := []byte(c.ChunkID)
	textBytes := []byte(c.Text)
	
	// Calculate size
	size := 4 + len(chunkIDBytes) + // ChunkID
		4 + len(textBytes) + // Text
		4 // MetadataCount
	
	// Add metadata size
	for k, v := range c.Metadata {
		size += 4 + len(k) + 4 + len(v)
	}
	
	buf := make([]byte, size)
	offset := 0
	
	// ChunkIDLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(chunkIDBytes)))
	offset += 4
	
	// ChunkID
	copy(buf[offset:offset+len(chunkIDBytes)], chunkIDBytes)
	offset += len(chunkIDBytes)
	
	// TextLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(textBytes)))
	offset += 4
	
	// Text
	copy(buf[offset:offset+len(textBytes)], textBytes)
	offset += len(textBytes)
	
	// MetadataCount
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(c.Metadata)))
	offset += 4
	
	// Metadata entries
	for k, v := range c.Metadata {
		keyBytes := []byte(k)
		valueBytes := []byte(v)
		
		// KeyLen
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(keyBytes)))
		offset += 4
		
		// Key
		copy(buf[offset:offset+len(keyBytes)], keyBytes)
		offset += len(keyBytes)
		
		// ValueLen
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(valueBytes)))
		offset += 4
		
		// Value
		copy(buf[offset:offset+len(valueBytes)], valueBytes)
		offset += len(valueBytes)
	}
	
	return buf
}

// FromBytes deserializes a TextChunk from binary format.
func (c *TextChunk) FromBytes(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for chunk ID length")
	}
	
	offset := 0
	
	// ChunkIDLen
	chunkIDLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	
	if offset+chunkIDLen > len(data) {
		return fmt.Errorf("insufficient data for chunk ID")
	}
	c.ChunkID = ChunkID(data[offset : offset+chunkIDLen])
	offset += chunkIDLen
	
	// TextLen
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for text length")
	}
	textLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	
	if offset+textLen > len(data) {
		return fmt.Errorf("insufficient data for text")
	}
	c.Text = string(data[offset : offset+textLen])
	offset += textLen
	
	// MetadataCount
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for metadata count")
	}
	metadataCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	
	c.Metadata = make(map[string]string, metadataCount)
	
	// Metadata entries
	for i := 0; i < metadataCount; i++ {
		// KeyLen
		if offset+4 > len(data) {
			return fmt.Errorf("insufficient data for metadata key length")
		}
		keyLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		
		if offset+keyLen > len(data) {
			return fmt.Errorf("insufficient data for metadata key")
		}
		key := string(data[offset : offset+keyLen])
		offset += keyLen
		
		// ValueLen
		if offset+4 > len(data) {
			return fmt.Errorf("insufficient data for metadata value length")
		}
		valueLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		
		if offset+valueLen > len(data) {
			return fmt.Errorf("insufficient data for metadata value")
		}
		value := string(data[offset : offset+valueLen])
		offset += valueLen
		
		c.Metadata[key] = value
	}
	
	return nil
}

// ToJSON serializes the TextChunk to JSON format.
func (c *TextChunk) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON deserializes a TextChunk from JSON format.
func (c *TextChunk) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

