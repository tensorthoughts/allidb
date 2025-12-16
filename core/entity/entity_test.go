package entity

import (
	"sync"
	"testing"
	"time"
)

func TestTextChunk_Size(t *testing.T) {
	chunk := &TextChunk{
		ChunkID:  ChunkID("chunk1"),
		Text:     "This is a test chunk",
		Metadata: map[string]string{
			"source": "test",
			"type":   "paragraph",
		},
	}

	size := chunk.Size()
	if size <= 0 {
		t.Errorf("Expected positive size, got %d", size)
	}

	// Size should increase with more data
	chunk.Text = "This is a much longer test chunk with more text content"
	newSize := chunk.Size()
	if newSize <= size {
		t.Errorf("Expected larger size with more text, got %d vs %d", newSize, size)
	}
}

func TestTextChunk_ToBytes_FromBytes(t *testing.T) {
	original := &TextChunk{
		ChunkID:  ChunkID("chunk1"),
		Text:     "This is a test chunk",
		Metadata: map[string]string{
			"source": "test",
			"type":   "paragraph",
		},
	}

	// Serialize
	data := original.ToBytes()
	if len(data) == 0 {
		t.Fatal("ToBytes returned empty data")
	}

	// Deserialize
	decoded := &TextChunk{}
	err := decoded.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	// Verify
	if decoded.ChunkID != original.ChunkID {
		t.Errorf("Expected ChunkID %s, got %s", original.ChunkID, decoded.ChunkID)
	}
	if decoded.Text != original.Text {
		t.Errorf("Expected Text %s, got %s", original.Text, decoded.Text)
	}
	if len(decoded.Metadata) != len(original.Metadata) {
		t.Errorf("Expected %d metadata entries, got %d", len(original.Metadata), len(decoded.Metadata))
	}
	for k, v := range original.Metadata {
		if decoded.Metadata[k] != v {
			t.Errorf("Expected metadata[%s] = %s, got %s", k, v, decoded.Metadata[k])
		}
	}
}

func TestTextChunk_EmptyMetadata(t *testing.T) {
	chunk := &TextChunk{
		ChunkID:  ChunkID("chunk1"),
		Text:     "Test",
		Metadata: make(map[string]string),
	}

	data := chunk.ToBytes()
	decoded := &TextChunk{}
	err := decoded.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	if len(decoded.Metadata) != 0 {
		t.Errorf("Expected empty metadata, got %d entries", len(decoded.Metadata))
	}
}

func TestTextChunk_ToJSON_FromJSON(t *testing.T) {
	original := &TextChunk{
		ChunkID:  ChunkID("chunk1"),
		Text:     "This is a test chunk",
		Metadata: map[string]string{
			"source": "test",
		},
	}

	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	decoded := &TextChunk{}
	err = decoded.FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if decoded.ChunkID != original.ChunkID {
		t.Errorf("Expected ChunkID %s, got %s", original.ChunkID, decoded.ChunkID)
	}
	if decoded.Text != original.Text {
		t.Errorf("Expected Text %s, got %s", original.Text, decoded.Text)
	}
}

func TestGraphEdge_ToBytes_FromBytes(t *testing.T) {
	now := time.Now()
	original := &GraphEdge{
		FromEntity: EntityID("entity1"),
		ToEntity:   EntityID("entity2"),
		Relation:   Relation("connects"),
		Weight:     0.75,
		Timestamp:  now,
	}

	// Serialize
	data := original.ToBytes()
	if len(data) == 0 {
		t.Fatal("ToBytes returned empty data")
	}

	// Deserialize
	decoded := &GraphEdge{}
	err := decoded.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	// Verify
	if decoded.FromEntity != original.FromEntity {
		t.Errorf("Expected FromEntity %s, got %s", original.FromEntity, decoded.FromEntity)
	}
	if decoded.ToEntity != original.ToEntity {
		t.Errorf("Expected ToEntity %s, got %s", original.ToEntity, decoded.ToEntity)
	}
	if decoded.Relation != original.Relation {
		t.Errorf("Expected Relation %s, got %s", original.Relation, decoded.Relation)
	}
	if decoded.Weight != original.Weight {
		t.Errorf("Expected Weight %f, got %f", original.Weight, decoded.Weight)
	}
	// Timestamp comparison (within 1 second due to precision)
	if decoded.Timestamp.Unix() != original.Timestamp.Unix() {
		t.Errorf("Expected Timestamp %v, got %v", original.Timestamp, decoded.Timestamp)
	}
}

func TestGraphEdge_ToJSON_FromJSON(t *testing.T) {
	now := time.Now()
	original := &GraphEdge{
		FromEntity: EntityID("entity1"),
		ToEntity:   EntityID("entity2"),
		Relation:   Relation("connects"),
		Weight:     0.75,
		Timestamp:  now,
	}

	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	decoded := &GraphEdge{}
	err = decoded.FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if decoded.FromEntity != original.FromEntity {
		t.Errorf("Expected FromEntity %s, got %s", original.FromEntity, decoded.FromEntity)
	}
	if decoded.Weight != original.Weight {
		t.Errorf("Expected Weight %f, got %f", original.Weight, decoded.Weight)
	}
}

func TestUnifiedEntity_NewUnifiedEntity(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	if entity.GetEntityID() != EntityID("entity1") {
		t.Errorf("Expected EntityID entity1, got %s", entity.GetEntityID())
	}
	if entity.GetTenantID() != "tenant1" {
		t.Errorf("Expected TenantID tenant1, got %s", entity.GetTenantID())
	}
	if entity.GetEntityType() != EntityType("person") {
		t.Errorf("Expected EntityType person, got %s", entity.GetEntityType())
	}
	if len(entity.GetEmbeddings()) != 0 {
		t.Error("Expected empty embeddings")
	}
	if len(entity.GetIncomingEdges()) != 0 {
		t.Error("Expected empty incoming edges")
	}
	if len(entity.GetOutgoingEdges()) != 0 {
		t.Error("Expected empty outgoing edges")
	}
	if len(entity.GetChunks()) != 0 {
		t.Error("Expected empty chunks")
	}
}

func TestUnifiedEntity_AddEmbedding(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	vector := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	entity.AddEmbedding(vector)

	embeddings := entity.GetEmbeddings()
	if len(embeddings) != 1 {
		t.Fatalf("Expected 1 embedding, got %d", len(embeddings))
	}

	emb := embeddings[0]
	if emb.Version != 1 {
		t.Errorf("Expected version 1, got %d", emb.Version)
	}
	if len(emb.Vector) != len(vector) {
		t.Errorf("Expected vector length %d, got %d", len(vector), len(emb.Vector))
	}
	for i, v := range vector {
		if emb.Vector[i] != v {
			t.Errorf("Expected vector[%d] = %f, got %f", i, v, emb.Vector[i])
		}
	}

	// Add another embedding
	entity.AddEmbedding([]float32{0.6, 0.7, 0.8})
	embeddings = entity.GetEmbeddings()
	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}
	if embeddings[1].Version != 2 {
		t.Errorf("Expected version 2, got %d", embeddings[1].Version)
	}
}

func TestUnifiedEntity_GetLatestEmbedding(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	// No embeddings
	latest := entity.GetLatestEmbedding()
	if latest != nil {
		t.Error("Expected nil for no embeddings")
	}

	// Add embeddings
	entity.AddEmbedding([]float32{0.1, 0.2})
	entity.AddEmbedding([]float32{0.3, 0.4})
	entity.AddEmbedding([]float32{0.5, 0.6})

	latest = entity.GetLatestEmbedding()
	if latest == nil {
		t.Fatal("Expected latest embedding, got nil")
	}
	if latest.Version != 3 {
		t.Errorf("Expected version 3, got %d", latest.Version)
	}
	if latest.Vector[0] != 0.5 {
		t.Errorf("Expected latest vector[0] = 0.5, got %f", latest.Vector[0])
	}
}

func TestUnifiedEntity_AddEdges(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	// Add incoming edge
	incoming := GraphEdge{
		FromEntity: EntityID("entity0"),
		ToEntity:   EntityID("entity1"),
		Relation:   Relation("follows"),
		Weight:     0.8,
		Timestamp:  time.Now(),
	}
	entity.AddIncomingEdge(incoming)

	incomingEdges := entity.GetIncomingEdges()
	if len(incomingEdges) != 1 {
		t.Fatalf("Expected 1 incoming edge, got %d", len(incomingEdges))
	}
	if incomingEdges[0].FromEntity != EntityID("entity0") {
		t.Errorf("Expected FromEntity entity0, got %s", incomingEdges[0].FromEntity)
	}

	// Add outgoing edge
	outgoing := GraphEdge{
		FromEntity: EntityID("entity1"),
		ToEntity:   EntityID("entity2"),
		Relation:   Relation("connects"),
		Weight:     0.9,
		Timestamp:  time.Now(),
	}
	entity.AddOutgoingEdge(outgoing)

	outgoingEdges := entity.GetOutgoingEdges()
	if len(outgoingEdges) != 1 {
		t.Fatalf("Expected 1 outgoing edge, got %d", len(outgoingEdges))
	}
	if outgoingEdges[0].ToEntity != EntityID("entity2") {
		t.Errorf("Expected ToEntity entity2, got %s", outgoingEdges[0].ToEntity)
	}
}

func TestUnifiedEntity_AddChunk(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	chunk := TextChunk{
		ChunkID:  ChunkID("chunk1"),
		Text:     "This is a test chunk",
		Metadata: map[string]string{"source": "test"},
	}
	entity.AddChunk(chunk)

	chunks := entity.GetChunks()
	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].ChunkID != ChunkID("chunk1") {
		t.Errorf("Expected ChunkID chunk1, got %s", chunks[0].ChunkID)
	}
	if chunks[0].Text != "This is a test chunk" {
		t.Errorf("Expected text 'This is a test chunk', got '%s'", chunks[0].Text)
	}
}

func TestUnifiedEntity_SetImportance(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	if entity.GetImportance() != 0.0 {
		t.Errorf("Expected initial importance 0.0, got %f", entity.GetImportance())
	}

	entity.SetImportance(0.85)
	if entity.GetImportance() != 0.85 {
		t.Errorf("Expected importance 0.85, got %f", entity.GetImportance())
	}
}

func TestUnifiedEntity_Timestamps(t *testing.T) {
	before := time.Now()
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))
	after := time.Now()

	createdAt := entity.GetCreatedAt()
	if createdAt.Before(before) || createdAt.After(after) {
		t.Errorf("CreatedAt %v not in expected range [%v, %v]", createdAt, before, after)
	}

	updatedAt := entity.GetUpdatedAt()
	if updatedAt.Before(before) || updatedAt.After(after) {
		t.Errorf("UpdatedAt %v not in expected range [%v, %v]", updatedAt, before, after)
	}

	// Update should change UpdatedAt
	time.Sleep(10 * time.Millisecond)
	entity.AddEmbedding([]float32{0.1, 0.2})
	newUpdatedAt := entity.GetUpdatedAt()
	if !newUpdatedAt.After(updatedAt) {
		t.Error("UpdatedAt should increase after update")
	}
}

func TestUnifiedEntity_AppendOnly(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	// Add multiple items
	entity.AddEmbedding([]float32{0.1})
	entity.AddEmbedding([]float32{0.2})
	entity.AddEmbedding([]float32{0.3})

	embeddings := entity.GetEmbeddings()
	if len(embeddings) != 3 {
		t.Fatalf("Expected 3 embeddings, got %d", len(embeddings))
	}

	// Verify order is preserved
	if embeddings[0].Vector[0] != 0.1 {
		t.Error("First embedding should be 0.1")
	}
	if embeddings[1].Vector[0] != 0.2 {
		t.Error("Second embedding should be 0.2")
	}
	if embeddings[2].Vector[0] != 0.3 {
		t.Error("Third embedding should be 0.3")
	}
}

func TestUnifiedEntity_ThreadSafeReads(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	// Add some data
	entity.AddEmbedding([]float32{0.1, 0.2})
	entity.AddChunk(TextChunk{ChunkID: "chunk1", Text: "test"})

	// Concurrent reads
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = entity.GetEntityID()
			_ = entity.GetEmbeddings()
			_ = entity.GetChunks()
			_ = entity.GetImportance()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Error during concurrent read: %v", err)
		}
	}
}

func TestUnifiedEntity_ThreadSafeWrites(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	// Concurrent writes
	var wg sync.WaitGroup
	goroutines := 10
	writesPerGoroutine := 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				entity.AddEmbedding([]float32{float32(id), float32(j)})
				entity.AddChunk(TextChunk{
					ChunkID: ChunkID("chunk"),
					Text:    "test",
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify all writes were applied
	embeddings := entity.GetEmbeddings()
	expectedCount := goroutines * writesPerGoroutine
	if len(embeddings) != expectedCount {
		t.Errorf("Expected %d embeddings, got %d", expectedCount, len(embeddings))
	}

	chunks := entity.GetChunks()
	if len(chunks) != expectedCount {
		t.Errorf("Expected %d chunks, got %d", expectedCount, len(chunks))
	}
}

func TestUnifiedEntity_ToBytes_FromBytes(t *testing.T) {
	original := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))
	original.AddEmbedding([]float32{0.1, 0.2, 0.3})
	original.AddEmbedding([]float32{0.4, 0.5, 0.6})
	original.AddIncomingEdge(GraphEdge{
		FromEntity: EntityID("entity0"),
		ToEntity:   EntityID("entity1"),
		Relation:   Relation("follows"),
		Weight:     0.8,
		Timestamp:  time.Now(),
	})
	original.AddOutgoingEdge(GraphEdge{
		FromEntity: EntityID("entity1"),
		ToEntity:   EntityID("entity2"),
		Relation:   Relation("connects"),
		Weight:     0.9,
		Timestamp:  time.Now(),
	})
	original.AddChunk(TextChunk{
		ChunkID:  ChunkID("chunk1"),
		Text:     "Test chunk",
		Metadata: map[string]string{"source": "test"},
	})
	original.SetImportance(0.75)

	// Serialize
	data := original.ToBytes()
	if len(data) == 0 {
		t.Fatal("ToBytes returned empty data")
	}

	// Deserialize
	decoded := NewUnifiedEntity(EntityID(""), "", EntityType(""))
	err := decoded.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	// Verify
	if decoded.GetEntityID() != original.GetEntityID() {
		t.Errorf("Expected EntityID %s, got %s", original.GetEntityID(), decoded.GetEntityID())
	}
	if decoded.GetTenantID() != original.GetTenantID() {
		t.Errorf("Expected TenantID %s, got %s", original.GetTenantID(), decoded.GetTenantID())
	}

	embeddings := decoded.GetEmbeddings()
	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}

	incomingEdges := decoded.GetIncomingEdges()
	if len(incomingEdges) != 1 {
		t.Fatalf("Expected 1 incoming edge, got %d", len(incomingEdges))
	}

	outgoingEdges := decoded.GetOutgoingEdges()
	if len(outgoingEdges) != 1 {
		t.Fatalf("Expected 1 outgoing edge, got %d", len(outgoingEdges))
	}

	chunks := decoded.GetChunks()
	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(chunks))
	}

	if decoded.GetImportance() != 0.75 {
		t.Errorf("Expected importance 0.75, got %f", decoded.GetImportance())
	}
}

func TestUnifiedEntity_ToJSON_FromJSON(t *testing.T) {
	original := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))
	original.AddEmbedding([]float32{0.1, 0.2})
	original.AddChunk(TextChunk{
		ChunkID: ChunkID("chunk1"),
		Text:    "Test",
	})

	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	decoded := NewUnifiedEntity(EntityID(""), "", EntityType(""))
	err = decoded.FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if decoded.GetEntityID() != original.GetEntityID() {
		t.Errorf("Expected EntityID %s, got %s", original.GetEntityID(), decoded.GetEntityID())
	}

	embeddings := decoded.GetEmbeddings()
	if len(embeddings) != 1 {
		t.Fatalf("Expected 1 embedding, got %d", len(embeddings))
	}
}

func TestUnifiedEntity_EmptyEntity(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	data := entity.ToBytes()
	decoded := NewUnifiedEntity(EntityID(""), "", EntityType(""))
	err := decoded.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	if decoded.GetEntityID() != entity.GetEntityID() {
		t.Errorf("Expected EntityID %s, got %s", entity.GetEntityID(), decoded.GetEntityID())
	}
	if len(decoded.GetEmbeddings()) != 0 {
		t.Error("Expected empty embeddings")
	}
}

func TestUnifiedEntity_LargeVector(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	// Create a large vector (e.g., 1536 dimensions like OpenAI embeddings)
	largeVector := make([]float32, 1536)
	for i := range largeVector {
		largeVector[i] = float32(i) / 1000.0
	}

	entity.AddEmbedding(largeVector)

	embeddings := entity.GetEmbeddings()
	if len(embeddings) != 1 {
		t.Fatalf("Expected 1 embedding, got %d", len(embeddings))
	}

	if len(embeddings[0].Vector) != 1536 {
		t.Fatalf("Expected vector length 1536, got %d", len(embeddings[0].Vector))
	}

	// Verify serialization works
	data := entity.ToBytes()
	decoded := NewUnifiedEntity(EntityID(""), "", EntityType(""))
	err := decoded.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	decodedEmbeddings := decoded.GetEmbeddings()
	if len(decodedEmbeddings[0].Vector) != 1536 {
		t.Fatalf("Expected decoded vector length 1536, got %d", len(decodedEmbeddings[0].Vector))
	}
}

func TestUnifiedEntity_MultipleChunks(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	chunks := []TextChunk{
		{ChunkID: "chunk1", Text: "First chunk"},
		{ChunkID: "chunk2", Text: "Second chunk"},
		{ChunkID: "chunk3", Text: "Third chunk"},
	}

	for _, chunk := range chunks {
		entity.AddChunk(chunk)
	}

	retrieved := entity.GetChunks()
	if len(retrieved) != 3 {
		t.Fatalf("Expected 3 chunks, got %d", len(retrieved))
	}

	for i, chunk := range chunks {
		if retrieved[i].ChunkID != chunk.ChunkID {
			t.Errorf("Expected ChunkID %s at index %d, got %s", chunk.ChunkID, i, retrieved[i].ChunkID)
		}
	}
}

func TestUnifiedEntity_ConcurrentReadWrite(t *testing.T) {
	entity := NewUnifiedEntity(EntityID("entity1"), "tenant1", EntityType("person"))

	var wg sync.WaitGroup

	// Start writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				entity.AddEmbedding([]float32{float32(id), float32(j)})
			}
		}(i)
	}

	// Start readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = entity.GetEmbeddings()
				_ = entity.GetEntityID()
			}
		}()
	}

	wg.Wait()

	// Verify final state
	embeddings := entity.GetEmbeddings()
	if len(embeddings) != 50 {
		t.Errorf("Expected 50 embeddings, got %d", len(embeddings))
	}
}

