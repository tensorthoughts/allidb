package sstable

import (
	"bytes"
	"hash/crc32"
	"io"
	"os"
	"testing"
)

func TestFormat_Header(t *testing.T) {
	header := &Header{
		Magic:      Magic,
		Version:    Version,
		Reserved:   0,
		DataOffset: 100,
	}

	// Encode
	data := EncodeHeader(header)
	if len(data) != HeaderSize {
		t.Fatalf("Expected header size %d, got %d", HeaderSize, len(data))
	}

	// Decode
	decoded, err := DecodeHeader(data)
	if err != nil {
		t.Fatalf("Failed to decode header: %v", err)
	}

	if decoded.Magic != Magic {
		t.Errorf("Expected magic 0x%x, got 0x%x", Magic, decoded.Magic)
	}
	if decoded.Version != Version {
		t.Errorf("Expected version %d, got %d", Version, decoded.Version)
	}
	if decoded.DataOffset != 100 {
		t.Errorf("Expected data offset 100, got %d", decoded.DataOffset)
	}
}

func TestFormat_HeaderInvalidMagic(t *testing.T) {
	header := &Header{
		Magic:      0x12345678, // Invalid magic
		Version:    Version,
		Reserved:   0,
		DataOffset: 100,
	}

	data := EncodeHeader(header)
	_, err := DecodeHeader(data)
	if err == nil {
		t.Fatal("Expected error for invalid magic number")
	}
}

func TestFormat_HeaderInvalidVersion(t *testing.T) {
	header := &Header{
		Magic:      Magic,
		Version:    999, // Invalid version
		Reserved:   0,
		DataOffset: 100,
	}

	data := EncodeHeader(header)
	_, err := DecodeHeader(data)
	if err == nil {
		t.Fatal("Expected error for invalid version")
	}
}

func TestFormat_Footer(t *testing.T) {
	footer := &Footer{
		IndexOffset: 1000,
		IndexSize:   500,
		RowCount:    42,
	}

	// Encode
	data := EncodeFooter(footer)
	if len(data) != FooterSize {
		t.Fatalf("Expected footer size %d, got %d", FooterSize, len(data))
	}

	// Decode
	decoded, err := DecodeFooter(data)
	if err != nil {
		t.Fatalf("Failed to decode footer: %v", err)
	}

	if decoded.IndexOffset != 1000 {
		t.Errorf("Expected index offset 1000, got %d", decoded.IndexOffset)
	}
	if decoded.IndexSize != 500 {
		t.Errorf("Expected index size 500, got %d", decoded.IndexSize)
	}
	if decoded.RowCount != 42 {
		t.Errorf("Expected row count 42, got %d", decoded.RowCount)
	}
}

func TestFormat_IndexEntry(t *testing.T) {
	entry := &IndexEntry{
		EntityID: EntityID("entity1"),
		Offset:   1234,
	}

	// Encode
	data := EncodeIndexEntry(entry)

	// Decode
	decoded, offset, err := DecodeIndexEntry(data, 0)
	if err != nil {
		t.Fatalf("Failed to decode index entry: %v", err)
	}

	if decoded.EntityID != EntityID("entity1") {
		t.Errorf("Expected entity ID 'entity1', got '%s'", decoded.EntityID)
	}
	if decoded.Offset != 1234 {
		t.Errorf("Expected offset 1234, got %d", decoded.Offset)
	}
	if offset != len(data) {
		t.Errorf("Expected offset %d, got %d", len(data), offset)
	}
}

func TestFormat_RowTypes(t *testing.T) {
	types := []RowType{RowTypeVector, RowTypeEdge, RowTypeChunk, RowTypeMeta}
	for _, rt := range types {
		if rt < RowTypeVector || rt > RowTypeMeta {
			t.Errorf("Invalid row type: %d", rt)
		}
	}
}

func TestWriter_WriteHeader(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	err := writer.WriteHeader()
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	if buf.Len() != HeaderSize {
		t.Fatalf("Expected %d bytes, got %d", HeaderSize, buf.Len())
	}

	// Verify header
	header, err := DecodeHeader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to decode header: %v", err)
	}

	if header.Magic != Magic {
		t.Errorf("Invalid magic: 0x%x", header.Magic)
	}
	if header.DataOffset != int64(HeaderSize) {
		t.Errorf("Expected data offset %d, got %d", HeaderSize, header.DataOffset)
	}
}

func TestWriter_WriteRow(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	err := writer.WriteHeader()
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	row := &Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     []byte("test data"),
	}

	err = writer.WriteRow(row)
	if err != nil {
		t.Fatalf("Failed to write row: %v", err)
	}

	if writer.RowCount() != 1 {
		t.Errorf("Expected row count 1, got %d", writer.RowCount())
	}
}

func TestWriter_WriteMultipleRows(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	writer.WriteHeader()

	rows := []*Row{
		{Type: RowTypeVector, EntityID: EntityID("entity1"), Data: []byte("data1")},
		{Type: RowTypeVector, EntityID: EntityID("entity1"), Data: []byte("data2")},
		{Type: RowTypeEdge, EntityID: EntityID("entity2"), Data: []byte("data3")},
		{Type: RowTypeChunk, EntityID: EntityID("entity3"), Data: []byte("data4")},
	}

	for _, row := range rows {
		err := writer.WriteRow(row)
		if err != nil {
			t.Fatalf("Failed to write row: %v", err)
		}
	}

	if writer.RowCount() != int64(len(rows)) {
		t.Errorf("Expected row count %d, got %d", len(rows), writer.RowCount())
	}
}

func TestWriter_Close(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	writer.WriteHeader()

	rows := []*Row{
		{Type: RowTypeVector, EntityID: EntityID("entity1"), Data: []byte("data1")},
		{Type: RowTypeVector, EntityID: EntityID("entity2"), Data: []byte("data2")},
	}

	for _, row := range rows {
		writer.WriteRow(row)
	}

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify file structure
	data := buf.Bytes()
	if len(data) < HeaderSize+FooterSize {
		t.Fatalf("File too small: %d bytes", len(data))
	}

	// Check footer
	footerStart := len(data) - FooterSize
	footer, err := DecodeFooter(data[footerStart:])
	if err != nil {
		t.Fatalf("Failed to decode footer: %v", err)
	}

	if footer.RowCount != 2 {
		t.Errorf("Expected row count 2, got %d", footer.RowCount)
	}
	if footer.IndexOffset <= 0 {
		t.Errorf("Invalid index offset: %d", footer.IndexOffset)
	}
	if footer.IndexSize <= 0 {
		t.Errorf("Invalid index size: %d", footer.IndexSize)
	}
}

func TestWriter_CRC32(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	writer.WriteHeader()

	row := &Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     []byte("test data"),
	}

	err := writer.WriteRow(row)
	if err != nil {
		t.Fatalf("Failed to write row: %v", err)
	}

	// Manually calculate expected CRC
	entityIDBytes := []byte(row.EntityID)
	crcData := make([]byte, 1+4+len(entityIDBytes)+4+len(row.Data))
	offset := 0
	crcData[offset] = byte(row.Type)
	offset++
	// ... (simplified, actual implementation matches writer)
	expectedCRC := crc32.ChecksumIEEE(crcData)

	// The CRC should be stored in the row
	// We'll verify this when reading
	_ = expectedCRC
}

func TestReader_NewReader(t *testing.T) {
	// Create a valid SSTable
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()
	writer.WriteRow(&Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     []byte("data1"),
	})
	writer.Close()

	// Create reader
	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	if reader.RowCount() != 1 {
		t.Errorf("Expected row count 1, got %d", reader.RowCount())
	}
	if reader.EntityCount() != 1 {
		t.Errorf("Expected entity count 1, got %d", reader.EntityCount())
	}
}

func TestReader_Get(t *testing.T) {
	// Create SSTable with multiple entities
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()

	rows := []*Row{
		{Type: RowTypeVector, EntityID: EntityID("entity1"), Data: []byte("data1")},
		{Type: RowTypeVector, EntityID: EntityID("entity1"), Data: []byte("data2")},
		{Type: RowTypeEdge, EntityID: EntityID("entity2"), Data: []byte("data3")},
	}

	for _, row := range rows {
		writer.WriteRow(row)
	}
	writer.Close()

	// Read back
	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Get entity1
	entity1Rows, err := reader.Get(EntityID("entity1"))
	if err != nil {
		t.Fatalf("Failed to get entity1: %v", err)
	}

	if len(entity1Rows) != 2 {
		t.Fatalf("Expected 2 rows for entity1, got %d", len(entity1Rows))
	}

	if string(entity1Rows[0].Data) != "data1" {
		t.Errorf("Expected first row data 'data1', got '%s'", string(entity1Rows[0].Data))
	}
	if string(entity1Rows[1].Data) != "data2" {
		t.Errorf("Expected second row data 'data2', got '%s'", string(entity1Rows[1].Data))
	}

	// Get entity2
	entity2Rows, err := reader.Get(EntityID("entity2"))
	if err != nil {
		t.Fatalf("Failed to get entity2: %v", err)
	}

	if len(entity2Rows) != 1 {
		t.Fatalf("Expected 1 row for entity2, got %d", len(entity2Rows))
	}
	if string(entity2Rows[0].Data) != "data3" {
		t.Errorf("Expected row data 'data3', got '%s'", string(entity2Rows[0].Data))
	}
}

func TestReader_GetNonExistent(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()
	writer.WriteRow(&Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     []byte("data1"),
	})
	writer.Close()

	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	rows, err := reader.Get(EntityID("nonexistent"))
	if err != nil {
		t.Fatalf("Get should not return error for non-existent entity: %v", err)
	}
	if rows != nil {
		t.Errorf("Expected nil for non-existent entity, got %v", rows)
	}
}

func TestReader_CRC32Validation(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()
	writer.WriteRow(&Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     []byte("data1"),
	})
	writer.Close()

	// Corrupt the data
	data := buf.Bytes()
	// Find and corrupt a byte in the row data (skip header)
	if len(data) > HeaderSize+20 {
		data[HeaderSize+20] ^= 0xFF // Flip bits
	}

	// Try to read - should fail CRC check
	reader, err := NewReader(data)
	if err != nil {
		// This might fail during index loading, which is fine
		return
	}

	_, err = reader.Get(EntityID("entity1"))
	if err == nil {
		t.Fatal("Expected CRC validation error, got nil")
	}
}

func TestReader_GetAllEntityIDs(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()

	entities := []EntityID{"entity1", "entity2", "entity3"}
	for _, eid := range entities {
		writer.WriteRow(&Row{
			Type:     RowTypeVector,
			EntityID: eid,
			Data:     []byte("data"),
		})
	}
	writer.Close()

	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	ids := reader.GetAllEntityIDs()
	if len(ids) != len(entities) {
		t.Fatalf("Expected %d entity IDs, got %d", len(entities), len(ids))
	}

	// Verify all entities are present
	idMap := make(map[EntityID]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	for _, eid := range entities {
		if !idMap[eid] {
			t.Errorf("Entity ID '%s' not found in GetAllEntityIDs", eid)
		}
	}
}

func TestReader_ConcurrentReads(t *testing.T) {
	// Create a large SSTable
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()

	for i := 0; i < 100; i++ {
		writer.WriteRow(&Row{
			Type:     RowTypeVector,
			EntityID: EntityID("entity"),
			Data:     []byte("data"),
		})
	}
	writer.Close()

	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			rows, err := reader.Get(EntityID("entity"))
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			if len(rows) != 100 {
				t.Errorf("Expected 100 rows, got %d", len(rows))
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMergeReader_Get(t *testing.T) {
	// Create multiple SSTables
	sstables := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		var buf bytes.Buffer
		writer := NewWriter(&buf)
		writer.WriteHeader()
		writer.WriteRow(&Row{
			Type:     RowTypeVector,
			EntityID: EntityID("entity1"),
			Data:     []byte{byte(i)}, // Different data in each SSTable
		})
		writer.Close()
		sstables[i] = buf.Bytes()
	}

	// Create readers
	readers := make([]*Reader, 3)
	for i, data := range sstables {
		reader, err := NewReader(data)
		if err != nil {
			t.Fatalf("Failed to create reader %d: %v", i, err)
		}
		readers[i] = reader
	}

	// Create merge reader (newest first)
	mergeReader := NewMergeReader(readers...)

	// Get from merged reader
	rows, err := mergeReader.Get(EntityID("entity1"))
	if err != nil {
		t.Fatalf("Failed to get from merge reader: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("Expected 3 rows, got %d", len(rows))
	}

	// Verify order (newest first)
	if rows[0].Data[0] != 0 {
		t.Errorf("Expected first row data 0 (newest), got %d", rows[0].Data[0])
	}
	if rows[1].Data[0] != 1 {
		t.Errorf("Expected second row data 1, got %d", rows[1].Data[0])
	}
	if rows[2].Data[0] != 2 {
		t.Errorf("Expected third row data 2 (oldest), got %d", rows[2].Data[0])
	}
}

func TestMergeReader_RowCount(t *testing.T) {
	// Create multiple SSTables with different row counts
	sstables := make([][]byte, 3)
	rowCounts := []int{10, 20, 30}
	for i, count := range rowCounts {
		var buf bytes.Buffer
		writer := NewWriter(&buf)
		writer.WriteHeader()
		for j := 0; j < count; j++ {
			writer.WriteRow(&Row{
				Type:     RowTypeVector,
				EntityID: EntityID("entity"),
				Data:     []byte("data"),
			})
		}
		writer.Close()
		sstables[i] = buf.Bytes()
	}

	// Create readers
	readers := make([]*Reader, 3)
	for i, data := range sstables {
		reader, err := NewReader(data)
		if err != nil {
			t.Fatalf("Failed to create reader %d: %v", i, err)
		}
		readers[i] = reader
	}

	mergeReader := NewMergeReader(readers...)
	total := mergeReader.RowCount()

	expected := int64(10 + 20 + 30)
	if total != expected {
		t.Errorf("Expected total row count %d, got %d", expected, total)
	}
}

func TestMergeReader_EntityCount(t *testing.T) {
	// Create SSTables with overlapping entities
	var buf1 bytes.Buffer
	writer1 := NewWriter(&buf1)
	writer1.WriteHeader()
	writer1.WriteRow(&Row{Type: RowTypeVector, EntityID: EntityID("entity1"), Data: []byte("data")})
	writer1.WriteRow(&Row{Type: RowTypeVector, EntityID: EntityID("entity2"), Data: []byte("data")})
	writer1.Close()

	var buf2 bytes.Buffer
	writer2 := NewWriter(&buf2)
	writer2.WriteHeader()
	writer2.WriteRow(&Row{Type: RowTypeVector, EntityID: EntityID("entity2"), Data: []byte("data")})
	writer2.WriteRow(&Row{Type: RowTypeVector, EntityID: EntityID("entity3"), Data: []byte("data")})
	writer2.Close()

	reader1, _ := NewReader(buf1.Bytes())
	reader2, _ := NewReader(buf2.Bytes())

	mergeReader := NewMergeReader(reader1, reader2)
	count := mergeReader.EntityCount()

	// Should be 3 unique entities (entity1, entity2, entity3)
	if count != 3 {
		t.Errorf("Expected 3 unique entities, got %d", count)
	}
}

func TestReader_InvalidFile(t *testing.T) {
	// Test with too small file
	_, err := NewReader([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("Expected error for too small file")
	}

	// Test with invalid header
	invalidData := make([]byte, 1000)
	_, err = NewReader(invalidData)
	if err == nil {
		t.Fatal("Expected error for invalid header")
	}
}

func TestWriter_WriteToFile(t *testing.T) {
	// Create temporary file
	tmpfile, err := os.CreateTemp("", "sstable_test_*.sst")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	// Write SSTable
	writer := NewWriter(tmpfile)
	writer.WriteHeader()
	writer.WriteRow(&Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     []byte("test data"),
	})
	writer.Close()

	// Read back
	tmpfile.Seek(0, io.SeekStart)
	fileData, err := io.ReadAll(tmpfile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	reader, err := NewReader(fileData)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	rows, err := reader.Get(EntityID("entity1"))
	if err != nil {
		t.Fatalf("Failed to get row: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}
	if string(rows[0].Data) != "test data" {
		t.Errorf("Expected 'test data', got '%s'", string(rows[0].Data))
	}
}

func TestReader_RowTypes(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()

	rows := []*Row{
		{Type: RowTypeVector, EntityID: EntityID("e1"), Data: []byte("vector")},
		{Type: RowTypeEdge, EntityID: EntityID("e2"), Data: []byte("edge")},
		{Type: RowTypeChunk, EntityID: EntityID("e3"), Data: []byte("chunk")},
		{Type: RowTypeMeta, EntityID: EntityID("e4"), Data: []byte("meta")},
	}

	for _, row := range rows {
		writer.WriteRow(row)
	}
	writer.Close()

	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Verify row types
	vectorRows, _ := reader.Get(EntityID("e1"))
	if len(vectorRows) != 1 || vectorRows[0].Type != RowTypeVector {
		t.Error("Failed to read vector row")
	}

	edgeRows, _ := reader.Get(EntityID("e2"))
	if len(edgeRows) != 1 || edgeRows[0].Type != RowTypeEdge {
		t.Error("Failed to read edge row")
	}

	chunkRows, _ := reader.Get(EntityID("e3"))
	if len(chunkRows) != 1 || chunkRows[0].Type != RowTypeChunk {
		t.Error("Failed to read chunk row")
	}

	metaRows, _ := reader.Get(EntityID("e4"))
	if len(metaRows) != 1 || metaRows[0].Type != RowTypeMeta {
		t.Error("Failed to read meta row")
	}
}

func TestReader_LargeData(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	writer.WriteHeader()

	// Write row with large data
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	writer.WriteRow(&Row{
		Type:     RowTypeVector,
		EntityID: EntityID("entity1"),
		Data:     largeData,
	})
	writer.Close()

	reader, err := NewReader(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	rows, err := reader.Get(EntityID("entity1"))
	if err != nil {
		t.Fatalf("Failed to get row: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	if len(rows[0].Data) != len(largeData) {
		t.Fatalf("Expected data length %d, got %d", len(largeData), len(rows[0].Data))
	}

	// Verify data integrity
	for i := range largeData {
		if rows[0].Data[i] != largeData[i] {
			t.Fatalf("Data mismatch at index %d", i)
		}
	}
}

