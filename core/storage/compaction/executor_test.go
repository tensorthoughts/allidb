package compaction

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

func setupTestExecutor(t *testing.T) (*Executor, string, func()) {
	dir := filepath.Join(os.TempDir(), "compaction_executor_test_"+t.Name())
	err := os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Failed to clean test directory: %v", err)
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	config := DefaultExecutorConfig(dir)
	executor, err := NewExecutor(config)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return executor, dir, cleanup
}

func createTestSSTableWithRows(t *testing.T, dir, filename string, rows []*sstable.Row) string {
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

	for _, row := range rows {
		if err := writer.WriteRow(row); err != nil {
			t.Fatalf("Failed to write row: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	return path
}

func createRowWithTimestamp(t *testing.T, entityID sstable.EntityID, timestamp int64) *sstable.Row {
	// Create row data with timestamp at the beginning
	data := make([]byte, 8+10) // 8 bytes for timestamp + some data
	binary.LittleEndian.PutUint64(data[0:8], uint64(timestamp))
	copy(data[8:], []byte("testdata"))

	return &sstable.Row{
		Type:     sstable.RowTypeVector,
		EntityID: entityID,
		Data:     data,
	}
}

func TestDefaultExecutorConfig(t *testing.T) {
	config := DefaultExecutorConfig("/test/dir")

	if config.SSTableDir != "/test/dir" {
		t.Errorf("Expected SSTableDir '/test/dir', got '%s'", config.SSTableDir)
	}
	if config.SSTablePrefix != "sstable" {
		t.Errorf("Expected SSTablePrefix 'sstable', got '%s'", config.SSTablePrefix)
	}
}

func TestNewExecutor(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()

	if executor == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if executor.sstableDir == "" {
		t.Error("SSTableDir not set")
	}
}

func TestExecutor_Compact_SingleSSTable(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Create a single SSTable
	rows := []*sstable.Row{
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity1"), Data: []byte("data1")},
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity2"), Data: []byte("data2")},
	}
	inputPath := createTestSSTableWithRows(t, dir, "sstable-0.sst", rows)

	// Compact (should still work with single file)
	outputPath, err := executor.Compact([]string{inputPath})
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Verify output file exists
	if outputPath == "" {
		t.Fatal("Output path is empty")
	}
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Output file does not exist: %s", outputPath)
	}

	// Verify input file is deleted
	if _, err := os.Stat(inputPath); !os.IsNotExist(err) {
		t.Error("Input file should be deleted after compaction")
	}

	// Verify output can be read
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	reader, err := sstable.NewReader(data)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	if reader.RowCount() != 2 {
		t.Errorf("Expected 2 rows, got %d", reader.RowCount())
	}
}

func TestExecutor_Compact_MultipleSSTables(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Create two SSTables with overlapping entities
	rows1 := []*sstable.Row{
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity1"), Data: []byte("data1-old")},
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity2"), Data: []byte("data2")},
	}
	rows2 := []*sstable.Row{
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity1"), Data: []byte("data1-new")},
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity3"), Data: []byte("data3")},
	}

	inputPath1 := createTestSSTableWithRows(t, dir, "sstable-0.sst", rows1)
	inputPath2 := createTestSSTableWithRows(t, dir, "sstable-1.sst", rows2)

	// Compact
	outputPath, err := executor.Compact([]string{inputPath1, inputPath2})
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if outputPath == "" {
		t.Fatal("Output path is empty")
	}

	// Verify output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file %s: %v", outputPath, err)
	}

	reader, err := sstable.NewReader(data)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Should have 3 unique entities
	if reader.EntityCount() != 3 {
		t.Errorf("Expected 3 entities, got %d", reader.EntityCount())
	}

	// Verify input files are deleted
	if _, err := os.Stat(inputPath1); !os.IsNotExist(err) {
		t.Error("Input file 1 should be deleted")
	}
	if _, err := os.Stat(inputPath2); !os.IsNotExist(err) {
		t.Error("Input file 2 should be deleted")
	}
}

func TestExecutor_Compact_Deduplication(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Create rows with timestamps
	now := time.Now().UnixNano()
	oldTime := now - int64(time.Hour)
	newTime := now

	rows1 := []*sstable.Row{
		createRowWithTimestamp(t, sstable.EntityID("entity1"), oldTime),
	}
	rows2 := []*sstable.Row{
		createRowWithTimestamp(t, sstable.EntityID("entity1"), newTime),
	}

	inputPath1 := createTestSSTableWithRows(t, dir, "sstable-0.sst", rows1)
	inputPath2 := createTestSSTableWithRows(t, dir, "sstable-1.sst", rows2)

	// Compact
	outputPath, err := executor.Compact([]string{inputPath1, inputPath2})
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if outputPath == "" {
		t.Fatal("Output path is empty")
	}

	// Verify output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file %s: %v", outputPath, err)
	}

	reader, err := sstable.NewReader(data)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Should have only 1 entity (deduplicated)
	rows, err := reader.Get(sstable.EntityID("entity1"))
	if err != nil {
		t.Fatalf("Failed to get entity1: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("Expected 1 row after deduplication, got %d", len(rows))
	}

	// Verify it's the newer row
	timestamp := int64(binary.LittleEndian.Uint64(rows[0].Data[0:8]))
	if timestamp != newTime {
		t.Errorf("Expected newer timestamp %d, got %d", newTime, timestamp)
	}
}

func TestExecutor_Compact_EmptyInput(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()

	_, err := executor.Compact([]string{})
	if err == nil {
		t.Fatal("Expected error for empty input")
	}
}

func TestExecutor_Compact_InvalidSSTable(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Create invalid file
	invalidPath := filepath.Join(dir, "invalid.sst")
	if err := os.WriteFile(invalidPath, []byte("invalid data"), 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	_, err := executor.Compact([]string{invalidPath})
	if err == nil {
		t.Fatal("Expected error for invalid SSTable")
	}
}

func TestExecutor_Compact_AtomicSwap(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Create SSTable
	rows := []*sstable.Row{
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity1"), Data: []byte("data1")},
	}
	inputPath := createTestSSTableWithRows(t, dir, "sstable-0.sst", rows)

	// Compact
	outputPath, err := executor.Compact([]string{inputPath})
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Verify temp file doesn't exist
	tempPath := outputPath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after compaction")
	}

	// Verify output file exists and is valid
	if outputPath == "" {
		t.Fatal("Output path is empty")
	}
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Output file should exist: %s", outputPath)
	}

	// Verify input file is deleted
	if _, err := os.Stat(inputPath); !os.IsNotExist(err) {
		t.Errorf("Input file should be deleted: %s", inputPath)
	}

	// Verify it's a valid SSTable
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	_, err = sstable.NewReader(data)
	if err != nil {
		t.Fatalf("Output is not a valid SSTable: %v", err)
	}
}

func TestExecutor_SelectBestRow(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()
	defer cleanup()

	now := time.Now().UnixNano()
	oldTime := now - int64(time.Hour)
	newTime := now

	rows := []*sstable.Row{
		createRowWithTimestamp(t, sstable.EntityID("entity1"), oldTime),
		createRowWithTimestamp(t, sstable.EntityID("entity1"), newTime),
		createRowWithTimestamp(t, sstable.EntityID("entity1"), oldTime),
	}

	best := executor.selectBestRow(rows)
	if best == nil {
		t.Fatal("selectBestRow returned nil")
	}

	// Verify it selected the row with newest timestamp
	timestamp := int64(binary.LittleEndian.Uint64(best.Data[0:8]))
	if timestamp != newTime {
		t.Errorf("Expected newest timestamp %d, got %d", newTime, timestamp)
	}
}

func TestExecutor_SelectBestRow_SingleRow(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()

	row := createRowWithTimestamp(t, sstable.EntityID("entity1"), time.Now().UnixNano())
	best := executor.selectBestRow([]*sstable.Row{row})

	if best != row {
		t.Error("selectBestRow should return the single row")
	}
}

func TestExecutor_SelectBestRow_Empty(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()

	best := executor.selectBestRow([]*sstable.Row{})
	if best != nil {
		t.Error("selectBestRow should return nil for empty slice")
	}
}

func TestExecutor_ExtractTimestamp(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()

	now := time.Now().UnixNano()
	row := createRowWithTimestamp(t, sstable.EntityID("entity1"), now)

	timestamp := executor.extractTimestamp(row)
	if timestamp != now {
		t.Errorf("Expected timestamp %d, got %d", now, timestamp)
	}
}

func TestExecutor_ExtractTimestamp_Invalid(t *testing.T) {
	executor, _, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Row with too little data
	row := &sstable.Row{
		Type:     sstable.RowTypeVector,
		EntityID: sstable.EntityID("entity1"),
		Data:     []byte("short"),
	}

	timestamp := executor.extractTimestamp(row)
	if timestamp != 0 {
		t.Errorf("Expected 0 for invalid timestamp, got %d", timestamp)
	}
}

func TestExecutor_GetOutputPath(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	path1 := executor.GetOutputPath()
	path2 := executor.GetOutputPath()

	if path1 == path2 {
		t.Error("GetOutputPath should return different paths")
	}

	// Verify paths are in correct directory
	expectedDir := dir
	if filepath.Dir(path1) != expectedDir {
		t.Errorf("Expected path in %s, got %s", expectedDir, filepath.Dir(path1))
	}
}

func TestExecutor_StreamMergeAndDeduplicate(t *testing.T) {
	executor, dir, cleanup := setupTestExecutor(t)
	defer cleanup()

	// Create two SSTables
	rows1 := []*sstable.Row{
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity1"), Data: []byte("data1")},
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity2"), Data: []byte("data2")},
	}
	rows2 := []*sstable.Row{
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity1"), Data: []byte("data1-new")},
		{Type: sstable.RowTypeVector, EntityID: sstable.EntityID("entity3"), Data: []byte("data3")},
	}

	path1 := createTestSSTableWithRows(t, dir, "sstable-0.sst", rows1)
	path2 := createTestSSTableWithRows(t, dir, "sstable-1.sst", rows2)

	// Load readers
	data1, _ := os.ReadFile(path1)
	data2, _ := os.ReadFile(path2)
	reader1, _ := sstable.NewReader(data1)
	reader2, _ := sstable.NewReader(data2)

	// Create output file
	outputPath := filepath.Join(dir, "output.sst")
	outputFile, _ := os.Create(outputPath)
	defer outputFile.Close()

	writer := sstable.NewWriter(outputFile)
	writer.WriteHeader()

	// Stream merge
	err := executor.streamMergeAndDeduplicate([]*sstable.Reader{reader1, reader2}, writer)
	if err != nil {
		t.Fatalf("streamMergeAndDeduplicate failed: %v", err)
	}

	writer.Close()
	outputFile.Close()

	// Verify output
	outputData, _ := os.ReadFile(outputPath)
	outputReader, _ := sstable.NewReader(outputData)

	// Should have 3 unique entities
	if outputReader.EntityCount() != 3 {
		t.Errorf("Expected 3 entities, got %d", outputReader.EntityCount())
	}
}

