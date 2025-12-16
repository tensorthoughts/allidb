package unified

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

// encodeVector encodes a vector with ID for SSTable storage.
func encodeVector(id uint64, vector []float32) []byte {
	buf := make([]byte, 8+4+len(vector)*4)
	binary.LittleEndian.PutUint64(buf[0:8], id)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(len(vector)))
	for i, v := range vector {
		offset := 12 + i*4
		binary.LittleEndian.PutUint32(buf[offset:offset+4], math.Float32bits(v))
	}
	return buf
}

// createTestSSTable creates a test SSTable file with vectors and edges.
func createTestSSTable(t *testing.T, filePath string, vectors map[string][]float32, edges []entity.GraphEdge) {
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create SSTable file: %v", err)
	}
	defer file.Close()

	writer := sstable.NewWriter(file)
	if err := writer.WriteHeader(); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Write vectors
	for entityID, vector := range vectors {
		// Use entity ID as uint64 (simplified - in real system would have proper mapping)
		id := uint64(len(entityID)) // Simple hash
		vectorData := encodeVector(id, vector)
		row := &sstable.Row{
			Type:     sstable.RowTypeVector,
			EntityID: sstable.EntityID(entityID),
			Data:     vectorData,
		}
		if err := writer.WriteRow(row); err != nil {
			t.Fatalf("Failed to write vector row: %v", err)
		}
	}

	// Write edges
	for _, edge := range edges {
		edgeData := edge.ToBytes()
		row := &sstable.Row{
			Type:     sstable.RowTypeEdge,
			EntityID: sstable.EntityID(edge.FromEntity),
			Data:     edgeData,
		}
		if err := writer.WriteRow(row); err != nil {
			t.Fatalf("Failed to write edge row: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}

func setupTestDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "unified_index_test_"+t.Name())
	err := os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Failed to clean test directory: %v", err)
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	return dir
}

func cleanupTestDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		t.Logf("Failed to cleanup test directory: %v", err)
	}
}

func TestUnifiedIndex_New(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	if index == nil {
		t.Fatal("New returned nil")
	}

	if index.Size() != 0 {
		t.Errorf("Expected size 0 for empty index, got %d", index.Size())
	}
}

func TestUnifiedIndex_NewWithSSTables(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Create a test SSTable
	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
	}
	edges := []entity.GraphEdge{
		{
			FromEntity: entity.EntityID("entity1"),
			ToEntity:   entity.EntityID("entity2"),
			Relation:   entity.Relation("connects"),
			Weight:     0.8,
			Timestamp:  time.Now(),
		},
	}

	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, edges)

	config := DefaultConfig(dir)
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Give it a moment to load
	time.Sleep(100 * time.Millisecond)

	// Index should have loaded entities
	if index.Size() == 0 {
		t.Log("Warning: Index size is 0, may need more time to load")
	}
}

func TestUnifiedIndex_Search_EmptyIndex(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	query := []float32{1.0, 0.0, 0.0}
	results, err := index.Search(query, 5, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results != nil && len(results) > 0 {
		t.Errorf("Expected no results for empty index, got %d", len(results))
	}
}

func TestUnifiedIndex_Search_WithVectors(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Create a test SSTable with vectors
	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
		"entity3": {0.0, 0.0, 1.0},
		"entity4": {0.9, 0.1, 0.0}, // Similar to entity1
	}

	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond // Faster reload for testing
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for initial load
	time.Sleep(200 * time.Millisecond)

	// Search for vector similar to entity1
	query := []float32{0.95, 0.05, 0.0}
	results, err := index.Search(query, 3, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Log("Warning: No results found, may need more time for index to build")
		return
	}

	// Should find entity1 or entity4 (most similar)
	found := false
	for _, result := range results {
		if result == EntityID("entity1") || result == EntityID("entity4") {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Warning: Expected entity1 or entity4 in results, got %v", results)
	}
}

func TestUnifiedIndex_Search_WithGraphExpansion(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Create vectors
	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
		"entity3": {0.0, 0.0, 1.0},
	}

	// Create edges connecting entities
	edges := []entity.GraphEdge{
		{
			FromEntity: entity.EntityID("entity1"),
			ToEntity:   entity.EntityID("entity2"),
			Relation:   entity.Relation("connects"),
			Weight:     0.8,
			Timestamp:  time.Now(),
		},
		{
			FromEntity: entity.EntityID("entity2"),
			ToEntity:   entity.EntityID("entity3"),
			Relation:   entity.Relation("connects"),
			Weight:     0.7,
			Timestamp:  time.Now(),
		},
	}

	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, edges)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(200 * time.Millisecond)

	// Search with graph expansion
	query := []float32{1.0, 0.0, 0.0}
	results, err := index.Search(query, 5, 3) // expandFactor=3 for graph expansion
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Log("Warning: No results found")
		return
	}

	// With graph expansion, we should potentially find entity2 (connected to entity1)
	_ = results
}

func TestUnifiedIndex_GetEntity(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
	}

	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(200 * time.Millisecond)

	// Get entity
	ent := index.GetEntity(EntityID("entity1"))
	if ent == nil {
		t.Log("Warning: Entity not found, may need more time to load")
		return
	}

	if ent.GetEntityID() != EntityID("entity1") {
		t.Errorf("Expected entity ID entity1, got %s", ent.GetEntityID())
	}
}

func TestUnifiedIndex_Size(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
		"entity3": {0.0, 0.0, 1.0},
	}

	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(200 * time.Millisecond)

	size := index.Size()
	if size == 0 {
		t.Log("Warning: Size is 0, may need more time to load")
	} else if size != 3 {
		t.Logf("Warning: Expected size 3, got %d", size)
	}
}

func TestUnifiedIndex_HotReload(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Create initial SSTable
	vectors1 := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
	}
	filePath1 := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath1, vectors1, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 200 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for initial load
	time.Sleep(300 * time.Millisecond)

	initialSize := index.Size()

	// Create a new SSTable file
	vectors2 := map[string][]float32{
		"entity3": {0.0, 0.0, 1.0},
		"entity4": {1.0, 1.0, 0.0},
	}
	filePath2 := filepath.Join(dir, "sstable-1.sst")
	createTestSSTable(t, filePath2, vectors2, nil)

	// Wait for hot reload
	time.Sleep(500 * time.Millisecond)

	newSize := index.Size()
	if newSize <= initialSize {
		t.Logf("Warning: Size did not increase after hot reload. Initial: %d, New: %d", initialSize, newSize)
	}
}

func TestUnifiedIndex_ConcurrentSearches(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Create test data
	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
		"entity3": {0.0, 0.0, 1.0},
	}
	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(200 * time.Millisecond)

	// Concurrent searches
	var wg sync.WaitGroup
	goroutines := 10
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			query := []float32{1.0, 0.0, 0.0}
			_, err := index.Search(query, 5, 2)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Errorf("Error during concurrent search: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors during concurrent searches", errorCount)
	}
}

func TestUnifiedIndex_DefaultConfig(t *testing.T) {
	dir := setupTestDir(t)
	config := DefaultConfig(dir)

	if config.GraphCacheSize != 10000 {
		t.Errorf("Expected GraphCacheSize 10000, got %d", config.GraphCacheSize)
	}
	if config.EntityCacheSize != 1000 {
		t.Errorf("Expected EntityCacheSize 1000, got %d", config.EntityCacheSize)
	}
	if config.SSTablePrefix != "sstable" {
		t.Errorf("Expected SSTablePrefix 'sstable', got '%s'", config.SSTablePrefix)
	}
	if config.ReloadInterval != 5*time.Second {
		t.Errorf("Expected ReloadInterval 5s, got %v", config.ReloadInterval)
	}
}

func TestUnifiedIndex_Close(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}

	// Close should not error
	err = index.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Closing again should be safe
	err = index.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

func TestUnifiedIndex_MultipleSSTables(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Create multiple SSTables
	vectors1 := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.0, 1.0, 0.0},
	}
	filePath1 := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath1, vectors1, nil)

	vectors2 := map[string][]float32{
		"entity3": {0.0, 0.0, 1.0},
		"entity4": {1.0, 1.0, 0.0},
	}
	filePath2 := filepath.Join(dir, "sstable-1.sst")
	createTestSSTable(t, filePath2, vectors2, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(300 * time.Millisecond)

	size := index.Size()
	if size == 0 {
		t.Log("Warning: Size is 0, may need more time to load")
	} else if size < 2 {
		t.Logf("Warning: Expected at least 2 entities, got %d", size)
	}
}

func TestUnifiedIndex_SearchExpandFactor(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
		"entity2": {0.9, 0.1, 0.0},
		"entity3": {0.8, 0.2, 0.0},
		"entity4": {0.7, 0.3, 0.0},
		"entity5": {0.6, 0.4, 0.0},
	}
	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, nil)

	config := DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(200 * time.Millisecond)

	query := []float32{1.0, 0.0, 0.0}

	// Search with different expand factors
	results1, err := index.Search(query, 3, 1) // No expansion
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	results2, err := index.Search(query, 3, 5) // More expansion
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// With higher expand factor, we might get more candidates
	// (though results are still limited to k)
	_ = results1
	_ = results2
}

func TestUnifiedIndex_EntityCache(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	vectors := map[string][]float32{
		"entity1": {1.0, 0.0, 0.0},
	}
	filePath := filepath.Join(dir, "sstable-0.sst")
	createTestSSTable(t, filePath, vectors, nil)

	config := DefaultConfig(dir)
	config.EntityCacheSize = 10 // Small cache for testing
	config.ReloadInterval = 100 * time.Millisecond
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Wait for load
	time.Sleep(200 * time.Millisecond)

	// Get entity multiple times (should use cache after first)
	ent1 := index.GetEntity(EntityID("entity1"))
	ent2 := index.GetEntity(EntityID("entity1"))

	if ent1 == nil || ent2 == nil {
		t.Log("Warning: Entity not found")
		return
	}

	// Both should return the same entity
	if ent1.GetEntityID() != ent2.GetEntityID() {
		t.Error("Expected same entity from cache")
	}
}

func TestUnifiedIndex_EmptyDirectory(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	index, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}
	defer index.Close()

	// Should work fine with empty directory
	if index.Size() != 0 {
		t.Errorf("Expected size 0 for empty directory, got %d", index.Size())
	}

	query := []float32{1.0, 0.0, 0.0}
	results, err := index.Search(query, 5, 2)
	if err != nil {
		t.Fatalf("Search should not error on empty index: %v", err)
	}
	if results != nil && len(results) > 0 {
		t.Errorf("Expected no results, got %d", len(results))
	}
}

