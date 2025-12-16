package compaction

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

func setupTestPlanner(t *testing.T) (*Planner, string, func()) {
	dir := filepath.Join(os.TempDir(), "compaction_planner_test_"+t.Name())
	err := os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Failed to clean test directory: %v", err)
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	config := DefaultPlannerConfig(dir)
	planner := NewPlanner(config)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return planner, dir, cleanup
}

func createTestSSTable(t *testing.T, dir, filename string, entityCount int, sizeHint int64) string {
	path := filepath.Join(dir, filename)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test SSTable: %v", err)
	}
	defer file.Close()

	writer := sstable.NewWriter(file)
	if err := writer.WriteHeader(); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Write rows to approximate the desired size
	// Each row is roughly: 1 (type) + 4 (entityID len) + len(entityID) + 4 (data len) + data + 4 (CRC) = ~120+ bytes
	bytesWritten := int64(0)
	rowSize := int64(120) // Approximate row size
	rowsNeeded := (sizeHint / rowSize) + 1
	if rowsNeeded > int64(entityCount) {
		rowsNeeded = int64(entityCount)
	}

	for i := 0; i < int(rowsNeeded); i++ {
		entityID := sstable.EntityID(fmt.Sprintf("entity-%d", i))
		// Make data larger to reach size hint
		dataSize := 100
		if sizeHint > 0 && bytesWritten < sizeHint {
			remaining := sizeHint - bytesWritten
			if remaining > 100 {
				dataSize = int(remaining) - 50 // Leave room for row overhead
			}
		}
		row := &sstable.Row{
			Type:     sstable.RowTypeVector,
			EntityID: entityID,
			Data:     make([]byte, dataSize),
		}
		if err := writer.WriteRow(row); err != nil {
			t.Fatalf("Failed to write row: %v", err)
		}
		bytesWritten += int64(row.RowSize())
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Update file modification time to control age
	// Make file older by setting mod time in the past
	age := time.Hour * 2
	oldTime := time.Now().Add(-age)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	return path
}

func TestDefaultPlannerConfig(t *testing.T) {
	config := DefaultPlannerConfig("/test/dir")

	if config.SSTableDir != "/test/dir" {
		t.Errorf("Expected SSTableDir '/test/dir', got '%s'", config.SSTableDir)
	}
	if config.SSTablePrefix != "sstable" {
		t.Errorf("Expected SSTablePrefix 'sstable', got '%s'", config.SSTablePrefix)
	}
	if config.MinSize != 1*1024*1024 {
		t.Errorf("Expected MinSize 1MB, got %d", config.MinSize)
	}
	if config.MaxAge != 1*time.Hour {
		t.Errorf("Expected MaxAge 1 hour, got %v", config.MaxAge)
	}
}

func TestNewPlanner(t *testing.T) {
	planner, _, cleanup := setupTestPlanner(t)
	defer cleanup()

	if planner == nil {
		t.Fatal("NewPlanner returned nil")
	}
	if planner.sstableDir == "" {
		t.Error("SSTableDir not set")
	}
}

func TestPlanner_ListSSTables_Empty(t *testing.T) {
	planner, _, cleanup := setupTestPlanner(t)
	defer cleanup()

	infos, err := planner.ListSSTables()
	if err != nil {
		t.Fatalf("ListSSTables failed: %v", err)
	}

	if len(infos) != 0 {
		t.Errorf("Expected 0 SSTables, got %d", len(infos))
	}
}

func TestPlanner_ListSSTables_WithFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create test SSTables
	createTestSSTable(t, dir, "sstable-0.sst", 10, 1024)
	createTestSSTable(t, dir, "sstable-1.sst", 20, 2048)

	infos, err := planner.ListSSTables()
	if err != nil {
		t.Fatalf("ListSSTables failed: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("Expected 2 SSTables, got %d", len(infos))
	}

	// Verify they're sorted by size (smallest first)
	if infos[0].Size > infos[1].Size {
		t.Error("SSTables not sorted by size")
	}
}

func TestPlanner_ListSSTables_IgnoresNonSSTables(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create SSTable
	createTestSSTable(t, dir, "sstable-0.sst", 10, 1024)

	// Create non-SSTable file
	otherFile := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(otherFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create other file: %v", err)
	}

	infos, err := planner.ListSSTables()
	if err != nil {
		t.Fatalf("ListSSTables failed: %v", err)
	}

	if len(infos) != 1 {
		t.Errorf("Expected 1 SSTable, got %d", len(infos))
	}
}

func TestPlanner_ShouldCompact_NotEnoughFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Only one file
	createTestSSTable(t, dir, "sstable-0.sst", 10, 1024)

	should, err := planner.ShouldCompact()
	if err != nil {
		t.Fatalf("ShouldCompact failed: %v", err)
	}

	if should {
		t.Error("Should not compact with only 1 file")
	}
}

func TestPlanner_ShouldCompact_EnoughFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create 4 files in same tier (should trigger compaction)
	for i := 0; i < 4; i++ {
		createTestSSTable(t, dir, fmt.Sprintf("sstable-%d.sst", i), 10, 1024*1024) // 1MB each
	}

	should, err := planner.ShouldCompact()
	if err != nil {
		t.Fatalf("ShouldCompact failed: %v", err)
	}

	if !should {
		t.Error("Should compact with 4 files in same tier")
	}
}

func TestPlanner_ShouldCompact_OldFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create 2 old files (older than max age)
	// Need to ensure they're large enough to meet min size
	for i := 0; i < 2; i++ {
		path := createTestSSTable(t, dir, fmt.Sprintf("sstable-%d.sst", i), 100, 2*1024*1024) // 2MB each
		// Make them very old (already done in createTestSSTable, but ensure it's old enough)
		oldTime := time.Now().Add(-2 * time.Hour)
		os.Chtimes(path, oldTime, oldTime)
	}

	should, err := planner.ShouldCompact()
	if err != nil {
		t.Fatalf("ShouldCompact failed: %v", err)
	}

	if !should {
		t.Error("Should compact with 2 old files")
	}
}

func TestPlanner_PlanCompaction_NoFiles(t *testing.T) {
	planner, _, cleanup := setupTestPlanner(t)
	defer cleanup()

	compactions, err := planner.PlanCompaction()
	if err != nil {
		t.Fatalf("PlanCompaction failed: %v", err)
	}

	if compactions != nil && len(compactions) > 0 {
		t.Errorf("Expected no compactions, got %d", len(compactions))
	}
}

func TestPlanner_PlanCompaction_SizeTiered(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create 4 files of similar size (same tier)
	// Need enough entities to make files large enough
	for i := 0; i < 4; i++ {
		createTestSSTable(t, dir, fmt.Sprintf("sstable-%d.sst", i), 100, 2*1024*1024) // 2MB each
	}

	compactions, err := planner.PlanCompaction()
	if err != nil {
		t.Fatalf("PlanCompaction failed: %v", err)
	}

	if len(compactions) == 0 {
		t.Fatal("Expected at least one compaction plan")
	}

	// Should have one compaction with 4 files
	found := false
	for _, compaction := range compactions {
		if len(compaction) >= 4 {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected compaction plan with at least 4 files")
	}
}

func TestPlanner_PlanCompaction_MultipleTiers(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create files in different size tiers
	// Tier 1: 4 small files (2MB each)
	for i := 0; i < 4; i++ {
		createTestSSTable(t, dir, fmt.Sprintf("sstable-small-%d.sst", i), 100, 2*1024*1024)
	}

	// Tier 2: 2 medium files (5MB each)
	for i := 0; i < 2; i++ {
		createTestSSTable(t, dir, fmt.Sprintf("sstable-medium-%d.sst", i), 250, 5*1024*1024)
	}

	compactions, err := planner.PlanCompaction()
	if err != nil {
		t.Fatalf("PlanCompaction failed: %v", err)
	}

	// Should have at least one compaction (the 4 small files)
	if len(compactions) == 0 {
		t.Fatal("Expected at least one compaction plan")
	}
}

func TestPlanner_PlanCompaction_OldFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create 2 old files (large enough to meet min size)
	for i := 0; i < 2; i++ {
		path := createTestSSTable(t, dir, fmt.Sprintf("sstable-%d.sst", i), 100, 2*1024*1024)
		// Make them old
		oldTime := time.Now().Add(-2 * time.Hour)
		os.Chtimes(path, oldTime, oldTime)
	}

	compactions, err := planner.PlanCompaction()
	if err != nil {
		t.Fatalf("PlanCompaction failed: %v", err)
	}

	if len(compactions) == 0 {
		t.Fatal("Expected compaction plan for old files")
	}
}

func TestPlanner_GroupBySizeTier(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create files of different sizes
	infos := []SSTableInfo{
		{Path: filepath.Join(dir, "small1.sst"), Size: 1024 * 1024},      // 1MB
		{Path: filepath.Join(dir, "small2.sst"), Size: 1024 * 1024},      // 1MB
		{Path: filepath.Join(dir, "medium1.sst"), Size: 5 * 1024 * 1024}, // 5MB
		{Path: filepath.Join(dir, "medium2.sst"), Size: 6 * 1024 * 1024}, // 6MB
	}

	tiers := planner.groupBySizeTier(infos)

	// Should have at least 2 tiers (small and medium)
	if len(tiers) < 2 {
		t.Errorf("Expected at least 2 tiers, got %d", len(tiers))
	}
}

func TestPlanner_GroupBySizeTier_IgnoresSmallFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create files smaller than min size
	infos := []SSTableInfo{
		{Path: filepath.Join(dir, "tiny1.sst"), Size: 100}, // 100 bytes
		{Path: filepath.Join(dir, "tiny2.sst"), Size: 200}, // 200 bytes
	}

	tiers := planner.groupBySizeTier(infos)

	// Should ignore files smaller than min size
	if len(tiers) > 0 {
		t.Error("Should ignore files smaller than min size")
	}
}

func TestPlanner_ListSSTables_InvalidFiles(t *testing.T) {
	planner, dir, cleanup := setupTestPlanner(t)
	defer cleanup()

	// Create invalid SSTable file
	invalidFile := filepath.Join(dir, "sstable-invalid.sst")
	if err := os.WriteFile(invalidFile, []byte("invalid data"), 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	// Should skip invalid files and not error
	infos, err := planner.ListSSTables()
	if err != nil {
		t.Fatalf("ListSSTables should not fail on invalid files: %v", err)
	}

	// Should not include invalid file
	for _, info := range infos {
		if info.Path == invalidFile {
			t.Error("Should not include invalid SSTable file")
		}
	}
}

