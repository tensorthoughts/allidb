package entity

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// EntityID represents an entity identifier.
type EntityID string

// Relation represents a relationship type between entities.
type Relation string

// GraphEdge represents a directed edge in the graph.
type GraphEdge struct {
	FromEntity EntityID
	ToEntity   EntityID
	Relation   Relation
	Weight     float64
	Timestamp  time.Time
}

// ToBytes serializes the GraphEdge to binary format.
// Format:
//   - FromEntityLen (4 bytes): uint32
//   - FromEntity (variable): []byte
//   - ToEntityLen (4 bytes): uint32
//   - ToEntity (variable): []byte
//   - RelationLen (4 bytes): uint32
//   - Relation (variable): []byte
//   - Weight (8 bytes): float64
//   - Timestamp (8 bytes): int64 (Unix nanoseconds)
func (e *GraphEdge) ToBytes() []byte {
	fromBytes := []byte(e.FromEntity)
	toBytes := []byte(e.ToEntity)
	relationBytes := []byte(e.Relation)
	
	size := 4 + len(fromBytes) + // FromEntity
		4 + len(toBytes) + // ToEntity
		4 + len(relationBytes) + // Relation
		8 + // Weight
		8 // Timestamp
	
	buf := make([]byte, size)
	offset := 0
	
	// FromEntityLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(fromBytes)))
	offset += 4
	
	// FromEntity
	copy(buf[offset:offset+len(fromBytes)], fromBytes)
	offset += len(fromBytes)
	
	// ToEntityLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(toBytes)))
	offset += 4
	
	// ToEntity
	copy(buf[offset:offset+len(toBytes)], toBytes)
	offset += len(toBytes)
	
	// RelationLen
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(relationBytes)))
	offset += 4
	
	// Relation
	copy(buf[offset:offset+len(relationBytes)], relationBytes)
	offset += len(relationBytes)
	
	// Weight (encode as IEEE 754 float64)
	binary.LittleEndian.PutUint64(buf[offset:offset+8], math.Float64bits(e.Weight))
	offset += 8
	
	// Timestamp
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(e.Timestamp.UnixNano()))
	offset += 8
	
	return buf
}

// FromBytes deserializes a GraphEdge from binary format.
func (e *GraphEdge) FromBytes(data []byte) error {
	offset := 0
	
	// FromEntityLen
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for from entity length")
	}
	fromLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	
	if offset+fromLen > len(data) {
		return fmt.Errorf("insufficient data for from entity")
	}
	e.FromEntity = EntityID(data[offset : offset+fromLen])
	offset += fromLen
	
	// ToEntityLen
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for to entity length")
	}
	toLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	
	if offset+toLen > len(data) {
		return fmt.Errorf("insufficient data for to entity")
	}
	e.ToEntity = EntityID(data[offset : offset+toLen])
	offset += toLen
	
	// RelationLen
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for relation length")
	}
	relationLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	
	if offset+relationLen > len(data) {
		return fmt.Errorf("insufficient data for relation")
	}
	e.Relation = Relation(data[offset : offset+relationLen])
	offset += relationLen
	
	// Weight
	if offset+8 > len(data) {
		return fmt.Errorf("insufficient data for weight")
	}
	bits := binary.LittleEndian.Uint64(data[offset : offset+8])
	e.Weight = math.Float64frombits(bits)
	offset += 8
	
	// Timestamp
	if offset+8 > len(data) {
		return fmt.Errorf("insufficient data for timestamp")
	}
	timestampNanos := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	e.Timestamp = time.Unix(0, timestampNanos)
	offset += 8
	
	return nil
}

// ToJSON serializes the GraphEdge to JSON format.
func (e *GraphEdge) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes a GraphEdge from JSON format.
func (e *GraphEdge) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

