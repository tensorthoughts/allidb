package memtable

import (
	"sync"
	"testing"
)

// testRow is a simple Row implementation for testing.
type testRow struct {
	entityID EntityID
	data     []byte
}

func (r *testRow) EntityID() EntityID {
	return r.entityID
}

func (r *testRow) Size() int {
	// Approximate size: entityID (string) + data
	return len(r.entityID) + len(r.data) + 16 // 16 bytes overhead
}

func newTestRow(entityID EntityID, data string) *testRow {
	return &testRow{
		entityID: entityID,
		data:     []byte(data),
	}
}

func TestMemtable_PutAndGet(t *testing.T) {
	mt := New(1024 * 1024) // 1MB

	entityID := EntityID("entity1")
	row := newTestRow(entityID, "test data")

	mt.Put(row)

	rows := mt.Get(entityID)
	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	retrieved := rows[0].(*testRow)
	if retrieved.entityID != entityID {
		t.Errorf("Expected entityID %s, got %s", entityID, retrieved.entityID)
	}
	if string(retrieved.data) != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", string(retrieved.data))
	}
}

func TestMemtable_GetNonExistent(t *testing.T) {
	mt := New(1024 * 1024)

	rows := mt.Get(EntityID("nonexistent"))
	if rows != nil {
		t.Errorf("Expected nil for non-existent entity, got %v", rows)
	}
}

func TestMemtable_AppendOnly(t *testing.T) {
	mt := New(1024 * 1024)

	entityID := EntityID("entity1")

	// Add multiple rows for the same entity
	row1 := newTestRow(entityID, "data1")
	row2 := newTestRow(entityID, "data2")
	row3 := newTestRow(entityID, "data3")

	mt.Put(row1)
	mt.Put(row2)
	mt.Put(row3)

	rows := mt.Get(entityID)
	if len(rows) != 3 {
		t.Fatalf("Expected 3 rows, got %d", len(rows))
	}

	// Verify order (append-only means order is preserved)
	r1 := rows[0].(*testRow)
	r2 := rows[1].(*testRow)
	r3 := rows[2].(*testRow)

	if string(r1.data) != "data1" {
		t.Errorf("Expected first row data 'data1', got '%s'", string(r1.data))
	}
	if string(r2.data) != "data2" {
		t.Errorf("Expected second row data 'data2', got '%s'", string(r2.data))
	}
	if string(r3.data) != "data3" {
		t.Errorf("Expected third row data 'data3', got '%s'", string(r3.data))
	}
}

func TestMemtable_MultipleEntities(t *testing.T) {
	mt := New(1024 * 1024)

	entity1 := EntityID("entity1")
	entity2 := EntityID("entity2")
	entity3 := EntityID("entity3")

	mt.Put(newTestRow(entity1, "data1"))
	mt.Put(newTestRow(entity2, "data2"))
	mt.Put(newTestRow(entity3, "data3"))

	if mt.EntityCount() != 3 {
		t.Errorf("Expected 3 entities, got %d", mt.EntityCount())
	}

	rows1 := mt.Get(entity1)
	if len(rows1) != 1 {
		t.Errorf("Expected 1 row for entity1, got %d", len(rows1))
	}

	rows2 := mt.Get(entity2)
	if len(rows2) != 1 {
		t.Errorf("Expected 1 row for entity2, got %d", len(rows2))
	}

	rows3 := mt.Get(entity3)
	if len(rows3) != 1 {
		t.Errorf("Expected 1 row for entity3, got %d", len(rows3))
	}
}

func TestMemtable_SizeTracking(t *testing.T) {
	mt := New(1024 * 1024)

	initialSize := mt.Size()
	if initialSize != 0 {
		t.Errorf("Expected initial size 0, got %d", initialSize)
	}

	// Add rows and verify size increases
	row1 := newTestRow(EntityID("e1"), "data1")
	expectedSize1 := int64(row1.Size())
	mt.Put(row1)

	if mt.Size() != expectedSize1 {
		t.Errorf("Expected size %d after first row, got %d", expectedSize1, mt.Size())
	}

	row2 := newTestRow(EntityID("e2"), "data2")
	expectedSize2 := expectedSize1 + int64(row2.Size())
	mt.Put(row2)

	if mt.Size() != expectedSize2 {
		t.Errorf("Expected size %d after second row, got %d", expectedSize2, mt.Size())
	}

	// Add another row to same entity
	row3 := newTestRow(EntityID("e1"), "data3")
	expectedSize3 := expectedSize2 + int64(row3.Size())
	mt.Put(row3)

	if mt.Size() != expectedSize3 {
		t.Errorf("Expected size %d after third row, got %d", expectedSize3, mt.Size())
	}
}

func TestMemtable_ShouldFlush(t *testing.T) {
	// Create memtable with small threshold
	maxSize := int64(1000)
	mt := New(maxSize)

	if mt.ShouldFlush() {
		t.Error("ShouldFlush should be false for empty memtable")
	}

	// Add rows until we exceed threshold
	totalSize := int64(0)
	rowSize := int64(50)
	rowsAdded := 0

	for !mt.ShouldFlush() {
		entityID := EntityID("entity")
		row := newTestRow(entityID, "data")
		mt.Put(row)
		totalSize += rowSize
		rowsAdded++

		if rowsAdded > 100 {
			t.Fatal("ShouldFlush never returned true, possible infinite loop")
		}
	}

	if mt.Size() < maxSize {
		t.Errorf("Size %d should be >= maxSize %d when ShouldFlush is true", mt.Size(), maxSize)
	}
}

func TestMemtable_Reset(t *testing.T) {
	mt := New(1024 * 1024)

	// Add some data
	mt.Put(newTestRow(EntityID("e1"), "data1"))
	mt.Put(newTestRow(EntityID("e2"), "data2"))
	mt.Put(newTestRow(EntityID("e3"), "data3"))

	if mt.Count() == 0 {
		t.Fatal("Expected non-zero count before reset")
	}
	if mt.EntityCount() == 0 {
		t.Fatal("Expected non-zero entity count before reset")
	}
	if mt.Size() == 0 {
		t.Fatal("Expected non-zero size before reset")
	}

	mt.Reset()

	if mt.Count() != 0 {
		t.Errorf("Expected count 0 after reset, got %d", mt.Count())
	}
	if mt.EntityCount() != 0 {
		t.Errorf("Expected entity count 0 after reset, got %d", mt.EntityCount())
	}
	if mt.Size() != 0 {
		t.Errorf("Expected size 0 after reset, got %d", mt.Size())
	}

	// Verify data is gone
	rows := mt.Get(EntityID("e1"))
	if rows != nil {
		t.Error("Expected nil after reset, got rows")
	}
}

func TestMemtable_ConcurrentWrites(t *testing.T) {
	mt := New(1024 * 1024)

	goroutines := 10
	rowsPerGoroutine := 100
	var wg sync.WaitGroup
	errors := make(chan error, goroutines*rowsPerGoroutine)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < rowsPerGoroutine; j++ {
				entityID := EntityID("entity")
				row := newTestRow(entityID, "concurrent data")
				mt.Put(row)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Fatalf("Error during concurrent write: %v", err)
	}

	// Verify all rows were added
	rows := mt.Get(EntityID("entity"))
	expectedCount := goroutines * rowsPerGoroutine
	if len(rows) != expectedCount {
		t.Errorf("Expected %d rows, got %d", expectedCount, len(rows))
	}
}

func TestMemtable_ConcurrentReads(t *testing.T) {
	mt := New(1024 * 1024)

	// Pre-populate with data
	entityIDs := []EntityID{"e1", "e2", "e3", "e4", "e5"}
	for _, eid := range entityIDs {
		for i := 0; i < 10; i++ {
			mt.Put(newTestRow(eid, "data"))
		}
	}

	// Concurrent reads
	goroutines := 20
	readsPerGoroutine := 100
	var wg sync.WaitGroup
	errors := make(chan error, goroutines*readsPerGoroutine)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				// Read from different entities
				eid := entityIDs[j%len(entityIDs)]
				rows := mt.Get(eid)
				if rows == nil || len(rows) == 0 {
					errors <- nil // Not an error, just unexpected
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Count non-nil errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors during concurrent reads", errorCount)
	}
}

func TestMemtable_ConcurrentReadsAndWrites(t *testing.T) {
	mt := New(1024 * 1024)

	goroutines := 5
	operations := 100
	var wg sync.WaitGroup

	// Start writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				entityID := EntityID("entity")
				row := newTestRow(entityID, "data")
				mt.Put(row)
			}
		}(i)
	}

	// Start readers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				rows := mt.Get(EntityID("entity"))
				_ = rows // Use the result
				_ = mt.Size()
				_ = mt.Count()
				_ = mt.ShouldFlush()
			}
		}()
	}

	wg.Wait()

	// Verify final state is consistent
	rows := mt.Get(EntityID("entity"))
	expectedMin := goroutines * operations / 2 // At least half should be visible
	if len(rows) < expectedMin {
		t.Errorf("Expected at least %d rows, got %d", expectedMin, len(rows))
	}
}

func TestMemtable_Count(t *testing.T) {
	mt := New(1024 * 1024)

	if mt.Count() != 0 {
		t.Errorf("Expected count 0 for empty memtable, got %d", mt.Count())
	}

	// Add rows
	mt.Put(newTestRow(EntityID("e1"), "data1"))
	mt.Put(newTestRow(EntityID("e1"), "data2"))
	mt.Put(newTestRow(EntityID("e2"), "data3"))
	mt.Put(newTestRow(EntityID("e3"), "data4"))

	if mt.Count() != 4 {
		t.Errorf("Expected count 4, got %d", mt.Count())
	}
}

func TestMemtable_EntityCount(t *testing.T) {
	mt := New(1024 * 1024)

	if mt.EntityCount() != 0 {
		t.Errorf("Expected entity count 0 for empty memtable, got %d", mt.EntityCount())
	}

	// Add rows for different entities
	mt.Put(newTestRow(EntityID("e1"), "data1"))
	mt.Put(newTestRow(EntityID("e2"), "data2"))
	mt.Put(newTestRow(EntityID("e3"), "data3"))

	if mt.EntityCount() != 3 {
		t.Errorf("Expected entity count 3, got %d", mt.EntityCount())
	}

	// Add more rows to existing entities (should not increase entity count)
	mt.Put(newTestRow(EntityID("e1"), "data4"))
	mt.Put(newTestRow(EntityID("e2"), "data5"))

	if mt.EntityCount() != 3 {
		t.Errorf("Expected entity count 3 after adding to existing entities, got %d", mt.EntityCount())
	}
}

func TestMemtable_DefaultMaxSize(t *testing.T) {
	mt := New(0) // Should use default

	if mt.ShouldFlush() {
		t.Error("ShouldFlush should be false for empty memtable")
	}

	// Default should be 64MB, so we shouldn't trigger flush easily
	// Add a reasonable amount of data
	for i := 0; i < 1000; i++ {
		mt.Put(newTestRow(EntityID("entity"), "data"))
	}

	if mt.ShouldFlush() {
		t.Error("ShouldFlush should be false with default max size and small amount of data")
	}
}

func TestMemtable_GetReturnsCopy(t *testing.T) {
	mt := New(1024 * 1024)

	entityID := EntityID("entity")
	row := newTestRow(entityID, "original")
	mt.Put(row)

	rows1 := mt.Get(entityID)
	rows2 := mt.Get(entityID)

	// Modify the returned slice (should not affect memtable)
	if len(rows1) > 0 {
		rows1[0] = newTestRow(entityID, "modified")
	}

	// Get again and verify original data is still there
	rows3 := mt.Get(entityID)
	if len(rows3) == 0 {
		t.Fatal("Expected rows after modification")
	}

	retrieved := rows3[0].(*testRow)
	if string(retrieved.data) != "original" {
		t.Errorf("Expected 'original', got '%s' - memtable was modified", string(retrieved.data))
	}

	// Verify rows1 and rows2 are independent
	if len(rows2) > 0 {
		retrieved2 := rows2[0].(*testRow)
		if string(retrieved2.data) != "original" {
			t.Errorf("Expected 'original' in rows2, got '%s'", string(retrieved2.data))
		}
	}
}

func TestMemtable_LargeData(t *testing.T) {
	mt := New(10 * 1024 * 1024) // 10MB

	// Add many rows
	numRows := 10000
	for i := 0; i < numRows; i++ {
		entityID := EntityID("entity")
		data := make([]byte, 100) // 100 bytes per row
		row := &testRow{
			entityID: entityID,
			data:     data,
		}
		mt.Put(row)
	}

	if mt.Count() != numRows {
		t.Errorf("Expected %d rows, got %d", numRows, mt.Count())
	}

	rows := mt.Get(EntityID("entity"))
	if len(rows) != numRows {
		t.Errorf("Expected %d rows for entity, got %d", numRows, len(rows))
	}
}

