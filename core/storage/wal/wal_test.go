package wal

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// testEntry is a simple WALEntry implementation for testing.
type testEntry struct {
	ID    uint64
	Data  []byte
	Value string
}

func (e *testEntry) ToBytes() []byte {
	// Format: [8 bytes ID][4 bytes data len][data][4 bytes value len][value bytes]
	dataLen := uint32(len(e.Data))
	valueLen := uint32(len(e.Value))
	
	buf := make([]byte, 8+4+len(e.Data)+4+len(e.Value))
	offset := 0
	
	binary.LittleEndian.PutUint64(buf[offset:], e.ID)
	offset += 8
	
	binary.LittleEndian.PutUint32(buf[offset:], dataLen)
	offset += 4
	
	copy(buf[offset:], e.Data)
	offset += len(e.Data)
	
	binary.LittleEndian.PutUint32(buf[offset:], valueLen)
	offset += 4
	
	copy(buf[offset:], []byte(e.Value))
	
	return buf
}

func (e *testEntry) FromBytes(data []byte) error {
	if len(data) < 16 {
		return os.ErrInvalid
	}
	
	offset := 0
	e.ID = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	
	dataLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	
	if len(data) < offset+int(dataLen)+4 {
		return os.ErrInvalid
	}
	
	e.Data = make([]byte, dataLen)
	copy(e.Data, data[offset:offset+int(dataLen)])
	offset += int(dataLen)
	
	valueLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	
	if len(data) < offset+int(valueLen) {
		return os.ErrInvalid
	}
	
	e.Value = string(data[offset : offset+int(valueLen)])
	
	return nil
}

func setupTestDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "wal_test_"+t.Name())
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

func TestWAL_Append(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer w.Close()

	entry := &testEntry{
		ID:    1,
		Data:  []byte("test data"),
		Value: "test value",
	}

	err = w.Append(entry)
	if err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}
}

func TestWAL_AppendMultiple(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer w.Close()

	count := 100
	for i := 0; i < count; i++ {
		entry := &testEntry{
			ID:    uint64(i),
			Data:  []byte("data"),
			Value: "value",
		}
		err := w.Append(entry)
		if err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}
}

func TestWAL_ConcurrentAppend(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer w.Close()

	goroutines := 10
	entriesPerGoroutine := 50
	var wg sync.WaitGroup
	errors := make(chan error, goroutines*entriesPerGoroutine)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < entriesPerGoroutine; j++ {
				entry := &testEntry{
					ID:    uint64(id*entriesPerGoroutine + j),
					Data:  []byte("concurrent data"),
					Value: "concurrent value",
				}
				if err := w.Append(entry); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Fatalf("Error during concurrent append: %v", err)
	}
}

func TestWAL_Replay(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	config.EntryFactory = func() WALEntry {
		return &testEntry{}
	}

	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Append some entries
	expectedEntries := make([]*testEntry, 0, 50)
	for i := 0; i < 50; i++ {
		entry := &testEntry{
			ID:    uint64(i),
			Data:  []byte("replay data"),
			Value: "replay value",
		}
		expectedEntries = append(expectedEntries, entry)
		err := w.Append(entry)
		if err != nil {
			t.Fatalf("Failed to append entry: %v", err)
		}
	}

	w.Close()

	// Create a new WAL instance for replay
	w2, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL for replay: %v", err)
	}
	defer w2.Close()

	// Replay entries
	replayedEntries := make([]*testEntry, 0, 50)
	err = w2.Replay(func(entry WALEntry) error {
		te := entry.(*testEntry)
		replayedEntries = append(replayedEntries, te)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to replay: %v", err)
	}

	// Verify entries
	if len(replayedEntries) != len(expectedEntries) {
		t.Fatalf("Expected %d entries, got %d", len(expectedEntries), len(replayedEntries))
	}

	for i, expected := range expectedEntries {
		actual := replayedEntries[i]
		if actual.ID != expected.ID {
			t.Errorf("Entry %d: expected ID %d, got %d", i, expected.ID, actual.ID)
		}
		if string(actual.Data) != string(expected.Data) {
			t.Errorf("Entry %d: expected Data %s, got %s", i, string(expected.Data), string(actual.Data))
		}
		if actual.Value != expected.Value {
			t.Errorf("Entry %d: expected Value %s, got %s", i, expected.Value, actual.Value)
		}
	}
}

func TestWAL_ReplayEmpty(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	config.EntryFactory = func() WALEntry {
		return &testEntry{}
	}

	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer w.Close()

	// Replay with no entries
	err = w.Replay(func(entry WALEntry) error {
		t.Error("Callback should not be called for empty WAL")
		return nil
	})
	if err != nil {
		t.Fatalf("Replay should succeed with empty WAL: %v", err)
	}
}

func TestWAL_ReplayCallbackError(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	config.EntryFactory = func() WALEntry {
		return &testEntry{}
	}

	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Append some entries
	for i := 0; i < 10; i++ {
		entry := &testEntry{
			ID:    uint64(i),
			Data:  []byte("data"),
			Value: "value",
		}
		w.Append(entry)
	}

	w.Close()

	// Create new WAL for replay
	w2, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL for replay: %v", err)
	}
	defer w2.Close()

	// Replay should stop on callback error
	callCount := 0
	err = w2.Replay(func(entry WALEntry) error {
		callCount++
		if callCount == 5 {
			return os.ErrInvalid
		}
		return nil
	})

	if err == nil {
		t.Fatal("Expected error from callback, got nil")
	}
	if callCount != 5 {
		t.Errorf("Expected callback to be called 5 times, got %d", callCount)
	}
}

func TestWAL_FileRotation(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	// Use a small max file size to trigger rotation
	config := DefaultConfig(dir)
	config.MaxFileSize = 1024 // 1KB
	config.EntryFactory = func() WALEntry {
		return &testEntry{}
	}

	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Append entries until rotation occurs
	// Each entry is ~8+4+10+4+10 = 36 bytes + 8 bytes overhead = 44 bytes
	// So we need ~24 entries to fill 1KB
	for i := 0; i < 30; i++ {
		entry := &testEntry{
			ID:    uint64(i),
			Data:  make([]byte, 10),
			Value: "value",
		}
		err := w.Append(entry)
		if err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	w.Close()

	// Verify multiple files were created
	pattern := filepath.Join(dir, "wal-*.wal")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Failed to glob WAL files: %v", err)
	}

	if len(matches) < 2 {
		t.Errorf("Expected at least 2 WAL files due to rotation, got %d", len(matches))
	}

	// Verify replay works across multiple files
	w2, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL for replay: %v", err)
	}
	defer w2.Close()

	replayCount := 0
	err = w2.Replay(func(entry WALEntry) error {
		replayCount++
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to replay: %v", err)
	}

	if replayCount != 30 {
		t.Errorf("Expected 30 entries in replay, got %d", replayCount)
	}
}

func TestWAL_CRCValidation(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	entry := &testEntry{
		ID:    1,
		Data:  []byte("test"),
		Value: "value",
	}
	err = w.Append(entry)
	if err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}
	w.Close()

	// Corrupt the file by modifying a byte
	pattern := filepath.Join(dir, "wal-*.wal")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		t.Fatalf("Failed to find WAL file: %v", err)
	}

	file, err := os.OpenFile(matches[0], os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open WAL file: %v", err)
	}

	// Skip header (8 bytes: 4 length + 4 checksum) and corrupt first data byte
	file.Seek(8, 0)
	file.Write([]byte{0xFF})
	file.Close()

	// Try to replay - should fail due to CRC mismatch
	config.EntryFactory = func() WALEntry {
		return &testEntry{}
	}
	w2, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL for replay: %v", err)
	}
	defer w2.Close()

	err = w2.Replay(func(entry WALEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("Expected error due to corruption, got nil")
	}
}

func TestWAL_Close(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Append some entries
	for i := 0; i < 10; i++ {
		entry := &testEntry{
			ID:    uint64(i),
			Data:  []byte("data"),
			Value: "value",
		}
		w.Append(entry)
	}

	// Close should succeed
	err = w.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Appending after close should fail
	entry := &testEntry{
		ID:    100,
		Data:  []byte("data"),
		Value: "value",
	}
	err = w.Append(entry)
	if err == nil {
		t.Fatal("Expected error when appending to closed WAL")
	}
}

func TestWAL_ReplayWithoutFactory(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	entry := &testEntry{
		ID:    1,
		Data:  []byte("data"),
		Value: "value",
	}
	w.Append(entry)
	w.Close()

	// Try to replay without factory - should fail
	w2, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer w2.Close()

	err = w2.Replay(func(entry WALEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("Expected error when replaying without factory")
	}
}

func TestReadEntry(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupTestDir(t, dir)

	config := DefaultConfig(dir)
	w, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	entry := &testEntry{
		ID:    42,
		Data:  []byte("test data"),
		Value: "test value",
	}
	err = w.Append(entry)
	if err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}
	w.Close()

	// Read entry directly
	pattern := filepath.Join(dir, "wal-*.wal")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		t.Fatalf("Failed to find WAL file: %v", err)
	}

	file, err := os.Open(matches[0])
	if err != nil {
		t.Fatalf("Failed to open WAL file: %v", err)
	}
	defer file.Close()

	data, err := ReadEntry(file)
	if err != nil {
		t.Fatalf("Failed to read entry: %v", err)
	}

	// Verify we can deserialize
	readEntry := &testEntry{}
	err = readEntry.FromBytes(data)
	if err != nil {
		t.Fatalf("Failed to deserialize entry: %v", err)
	}

	if readEntry.ID != 42 {
		t.Errorf("Expected ID 42, got %d", readEntry.ID)
	}
	if string(readEntry.Data) != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", string(readEntry.Data))
	}
	if readEntry.Value != "test value" {
		t.Errorf("Expected value 'test value', got '%s'", readEntry.Value)
	}
}

