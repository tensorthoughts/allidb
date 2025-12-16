package entity

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// EntityType represents the type of an entity.
type EntityType string

// VectorEmbedding represents a versioned vector embedding.
type VectorEmbedding struct {
	Version   int64     // Version number
	Vector    []float32 // The embedding vector
	Timestamp time.Time // When this embedding was created
}

// UnifiedEntity represents a unified entity with all its associated data.
// It supports append-only updates and thread-safe reads.
type UnifiedEntity struct {
	mu sync.RWMutex // Protects all fields for thread-safe reads

	EntityID      EntityID
	TenantID      string
	EntityType    EntityType
	Embeddings    []VectorEmbedding // Versioned vector embeddings
	IncomingEdges []GraphEdge       // Incoming graph edges
	OutgoingEdges []GraphEdge       // Outgoing graph edges
	Chunks        []TextChunk       // Text chunks
	Importance    float64           // Importance score
	CreatedAt     time.Time         // Creation timestamp
	UpdatedAt     time.Time         // Last update timestamp
}

// NewUnifiedEntity creates a new UnifiedEntity with the given ID, tenant, and type.
func NewUnifiedEntity(entityID EntityID, tenantID string, entityType EntityType) *UnifiedEntity {
	now := time.Now()
	return &UnifiedEntity{
		EntityID:      entityID,
		TenantID:      tenantID,
		EntityType:    entityType,
		Embeddings:    make([]VectorEmbedding, 0),
		IncomingEdges: make([]GraphEdge, 0),
		OutgoingEdges: make([]GraphEdge, 0),
		Chunks:        make([]TextChunk, 0),
		Importance:    0.0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// AddEmbedding appends a new versioned embedding (append-only).
func (e *UnifiedEntity) AddEmbedding(vector []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()

	version := int64(len(e.Embeddings)) + 1
	embedding := VectorEmbedding{
		Version:   version,
		Vector:    make([]float32, len(vector)),
		Timestamp: time.Now(),
	}
	copy(embedding.Vector, vector)

	e.Embeddings = append(e.Embeddings, embedding)
	e.UpdatedAt = time.Now()
}

// AddIncomingEdge appends a new incoming edge (append-only).
func (e *UnifiedEntity) AddIncomingEdge(edge GraphEdge) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.IncomingEdges = append(e.IncomingEdges, edge)
	e.UpdatedAt = time.Now()
}

// AddOutgoingEdge appends a new outgoing edge (append-only).
func (e *UnifiedEntity) AddOutgoingEdge(edge GraphEdge) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.OutgoingEdges = append(e.OutgoingEdges, edge)
	e.UpdatedAt = time.Now()
}

// AddChunk appends a new text chunk (append-only).
func (e *UnifiedEntity) AddChunk(chunk TextChunk) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Chunks = append(e.Chunks, chunk)
	e.UpdatedAt = time.Now()
}

// SetImportance sets the importance score.
func (e *UnifiedEntity) SetImportance(score float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Importance = score
	e.UpdatedAt = time.Now()
}

// GetEntityID returns the entity ID (thread-safe read).
func (e *UnifiedEntity) GetEntityID() EntityID {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.EntityID
}

// GetTenantID returns the tenant ID (thread-safe read).
func (e *UnifiedEntity) GetTenantID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.TenantID
}

// GetEntityType returns the entity type (thread-safe read).
func (e *UnifiedEntity) GetEntityType() EntityType {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.EntityType
}

// GetEmbeddings returns a copy of all embeddings (thread-safe read).
func (e *UnifiedEntity) GetEmbeddings() []VectorEmbedding {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]VectorEmbedding, len(e.Embeddings))
	for i, emb := range e.Embeddings {
		result[i] = VectorEmbedding{
			Version:   emb.Version,
			Vector:    make([]float32, len(emb.Vector)),
			Timestamp: emb.Timestamp,
		}
		copy(result[i].Vector, emb.Vector)
	}
	return result
}

// GetLatestEmbedding returns the latest (highest version) embedding (thread-safe read).
func (e *UnifiedEntity) GetLatestEmbedding() *VectorEmbedding {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.Embeddings) == 0 {
		return nil
	}

	latest := e.Embeddings[len(e.Embeddings)-1]
	result := &VectorEmbedding{
		Version:   latest.Version,
		Vector:    make([]float32, len(latest.Vector)),
		Timestamp: latest.Timestamp,
	}
	copy(result.Vector, latest.Vector)
	return result
}

// GetIncomingEdges returns a copy of all incoming edges (thread-safe read).
func (e *UnifiedEntity) GetIncomingEdges() []GraphEdge {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]GraphEdge, len(e.IncomingEdges))
	copy(result, e.IncomingEdges)
	return result
}

// GetOutgoingEdges returns a copy of all outgoing edges (thread-safe read).
func (e *UnifiedEntity) GetOutgoingEdges() []GraphEdge {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]GraphEdge, len(e.OutgoingEdges))
	copy(result, e.OutgoingEdges)
	return result
}

// GetChunks returns a copy of all text chunks (thread-safe read).
func (e *UnifiedEntity) GetChunks() []TextChunk {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]TextChunk, len(e.Chunks))
	copy(result, e.Chunks)
	return result
}

// GetImportance returns the importance score (thread-safe read).
func (e *UnifiedEntity) GetImportance() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Importance
}

// GetCreatedAt returns the creation timestamp (thread-safe read).
func (e *UnifiedEntity) GetCreatedAt() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.CreatedAt
}

// GetUpdatedAt returns the last update timestamp (thread-safe read).
func (e *UnifiedEntity) GetUpdatedAt() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.UpdatedAt
}

// ToBytes serializes the UnifiedEntity to binary format.
func (e *UnifiedEntity) ToBytes() []byte {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Calculate size
	size := 0
	size += 4 + len(e.EntityID) + // EntityID
		4 + len(e.TenantID) + // TenantID
		4 + len(e.EntityType) + // EntityType
		4 + // Embeddings count
		4 + // IncomingEdges count
		4 + // OutgoingEdges count
		4 + // Chunks count
		8 + // Importance
		8 + // CreatedAt
		8 // UpdatedAt

	// Embeddings size
	for _, emb := range e.Embeddings {
		size += 8 + // Version
			8 + // Timestamp
			4 + // Vector length
			len(emb.Vector)*4 // Vector data (float32 = 4 bytes)
	}

	// Edges size
	for _, edge := range e.IncomingEdges {
		size += len(edge.ToBytes())
	}
	for _, edge := range e.OutgoingEdges {
		size += len(edge.ToBytes())
	}

	// Chunks size
	for _, chunk := range e.Chunks {
		size += len(chunk.ToBytes())
	}

	buf := make([]byte, size)
	offset := 0

	// EntityID
	entityIDBytes := []byte(e.EntityID)
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(entityIDBytes)))
	offset += 4
	copy(buf[offset:offset+len(entityIDBytes)], entityIDBytes)
	offset += len(entityIDBytes)

	// TenantID
	tenantIDBytes := []byte(e.TenantID)
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(tenantIDBytes)))
	offset += 4
	copy(buf[offset:offset+len(tenantIDBytes)], tenantIDBytes)
	offset += len(tenantIDBytes)

	// EntityType
	entityTypeBytes := []byte(e.EntityType)
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(entityTypeBytes)))
	offset += 4
	copy(buf[offset:offset+len(entityTypeBytes)], entityTypeBytes)
	offset += len(entityTypeBytes)

	// Embeddings
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(e.Embeddings)))
	offset += 4
	for _, emb := range e.Embeddings {
		binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(emb.Version))
		offset += 8
		binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(emb.Timestamp.UnixNano()))
		offset += 8
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(emb.Vector)))
		offset += 4
		for _, v := range emb.Vector {
			// Encode float32 using IEEE 754
			binary.LittleEndian.PutUint32(buf[offset:offset+4], math.Float32bits(v))
			offset += 4
		}
	}

	// IncomingEdges
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(e.IncomingEdges)))
	offset += 4
	for _, edge := range e.IncomingEdges {
		edgeBytes := edge.ToBytes()
		copy(buf[offset:offset+len(edgeBytes)], edgeBytes)
		offset += len(edgeBytes)
	}

	// OutgoingEdges
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(e.OutgoingEdges)))
	offset += 4
	for _, edge := range e.OutgoingEdges {
		edgeBytes := edge.ToBytes()
		copy(buf[offset:offset+len(edgeBytes)], edgeBytes)
		offset += len(edgeBytes)
	}

	// Chunks
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(e.Chunks)))
	offset += 4
	for _, chunk := range e.Chunks {
		chunkBytes := chunk.ToBytes()
		copy(buf[offset:offset+len(chunkBytes)], chunkBytes)
		offset += len(chunkBytes)
	}

	// Importance (encode as IEEE 754 float64)
	binary.LittleEndian.PutUint64(buf[offset:offset+8], math.Float64bits(e.Importance))
	offset += 8

	// CreatedAt
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(e.CreatedAt.UnixNano()))
	offset += 8

	// UpdatedAt
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(e.UpdatedAt.UnixNano()))
	offset += 8

	return buf
}

// FromBytes deserializes a UnifiedEntity from binary format.
func (e *UnifiedEntity) FromBytes(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	offset := 0

	// EntityID
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for entity ID length")
	}
	entityIDLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+entityIDLen > len(data) {
		return fmt.Errorf("insufficient data for entity ID")
	}
	e.EntityID = EntityID(data[offset : offset+entityIDLen])
	offset += entityIDLen

	// TenantID
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for tenant ID length")
	}
	tenantIDLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+tenantIDLen > len(data) {
		return fmt.Errorf("insufficient data for tenant ID")
	}
	e.TenantID = string(data[offset : offset+tenantIDLen])
	offset += tenantIDLen

	// EntityType
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for entity type length")
	}
	entityTypeLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+entityTypeLen > len(data) {
		return fmt.Errorf("insufficient data for entity type")
	}
	e.EntityType = EntityType(data[offset : offset+entityTypeLen])
	offset += entityTypeLen

	// Embeddings
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for embeddings count")
	}
	embeddingsCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	e.Embeddings = make([]VectorEmbedding, 0, embeddingsCount)
	for i := 0; i < embeddingsCount; i++ {
		if offset+8 > len(data) {
			return fmt.Errorf("insufficient data for embedding version")
		}
		version := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
		offset += 8

		if offset+8 > len(data) {
			return fmt.Errorf("insufficient data for embedding timestamp")
		}
		timestampNanos := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
		offset += 8

		if offset+4 > len(data) {
			return fmt.Errorf("insufficient data for vector length")
		}
		vectorLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+vectorLen*4 > len(data) {
			return fmt.Errorf("insufficient data for vector")
		}
		vector := make([]float32, vectorLen)
		for j := 0; j < vectorLen; j++ {
			bits := binary.LittleEndian.Uint32(data[offset : offset+4])
			vector[j] = math.Float32frombits(bits)
			offset += 4
		}

		e.Embeddings = append(e.Embeddings, VectorEmbedding{
			Version:   version,
			Vector:    vector,
			Timestamp: time.Unix(0, timestampNanos),
		})
	}

	// IncomingEdges
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for incoming edges count")
	}
	incomingCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	e.IncomingEdges = make([]GraphEdge, 0, incomingCount)
	for i := 0; i < incomingCount; i++ {
		edge := GraphEdge{}
		// We need to determine the size of the edge first
		// For simplicity, we'll try to decode it
		// This is a bit tricky since edges have variable length
		// Let's use a helper that reads until it finds the end
		edgeStart := offset
		if err := edge.FromBytes(data[offset:]); err != nil {
			return fmt.Errorf("failed to decode incoming edge %d: %w", i, err)
		}
		// Re-encode to find the size
		edgeBytes := edge.ToBytes()
		offset += len(edgeBytes)
		e.IncomingEdges = append(e.IncomingEdges, edge)
		_ = edgeStart
	}

	// OutgoingEdges
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for outgoing edges count")
	}
	outgoingCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	e.OutgoingEdges = make([]GraphEdge, 0, outgoingCount)
	for i := 0; i < outgoingCount; i++ {
		edge := GraphEdge{}
		if err := edge.FromBytes(data[offset:]); err != nil {
			return fmt.Errorf("failed to decode outgoing edge %d: %w", i, err)
		}
		edgeBytes := edge.ToBytes()
		offset += len(edgeBytes)
		e.OutgoingEdges = append(e.OutgoingEdges, edge)
	}

	// Chunks
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for chunks count")
	}
	chunksCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	e.Chunks = make([]TextChunk, 0, chunksCount)
	for i := 0; i < chunksCount; i++ {
		chunk := TextChunk{}
		if err := chunk.FromBytes(data[offset:]); err != nil {
			return fmt.Errorf("failed to decode chunk %d: %w", i, err)
		}
		chunkBytes := chunk.ToBytes()
		offset += len(chunkBytes)
		e.Chunks = append(e.Chunks, chunk)
	}

	// Importance
	if offset+8 > len(data) {
		return fmt.Errorf("insufficient data for importance")
	}
	bits := binary.LittleEndian.Uint64(data[offset : offset+8])
	e.Importance = math.Float64frombits(bits)
	offset += 8

	// CreatedAt
	if offset+8 > len(data) {
		return fmt.Errorf("insufficient data for created at")
	}
	createdNanos := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	e.CreatedAt = time.Unix(0, createdNanos)
	offset += 8

	// UpdatedAt
	if offset+8 > len(data) {
		return fmt.Errorf("insufficient data for updated at")
	}
	updatedNanos := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	e.UpdatedAt = time.Unix(0, updatedNanos)
	offset += 8

	return nil
}

// ToJSON serializes the UnifiedEntity to JSON format.
func (e *UnifiedEntity) ToJSON() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return json.Marshal(e)
}

// FromJSON deserializes a UnifiedEntity from JSON format.
func (e *UnifiedEntity) FromJSON(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return json.Unmarshal(data, e)
}
