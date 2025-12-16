package compaction

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

// Executor executes SSTable compaction by merging multiple SSTables.
type Executor struct {
	sstableDir    string
	sstablePrefix string
	nextSeq       uint64
	seqMu         sync.Mutex
}

// ExecutorConfig holds configuration for the compaction executor.
type ExecutorConfig struct {
	SSTableDir    string // Directory containing SSTable files
	SSTablePrefix string // Prefix for SSTable files (default: "sstable")
}

// DefaultExecutorConfig returns a default executor configuration.
func DefaultExecutorConfig(sstableDir string) ExecutorConfig {
	return ExecutorConfig{
		SSTableDir:    sstableDir,
		SSTablePrefix: "sstable",
	}
}

// NewExecutor creates a new compaction executor.
func NewExecutor(config ExecutorConfig) (*Executor, error) {
	// Find the highest sequence number
	nextSeq, err := findNextSequence(config.SSTableDir, config.SSTablePrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to find next sequence: %w", err)
	}

	return &Executor{
		sstableDir:    config.SSTableDir,
		sstablePrefix: config.SSTablePrefix,
		nextSeq:       nextSeq,
	}, nil
}

// findNextSequence finds the next available sequence number for SSTable files.
func findNextSequence(dir, prefix string) (uint64, error) {
	pattern := filepath.Join(dir, prefix+"-*.sst")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, err
	}

	maxSeq := uint64(0)
	for _, match := range matches {
		var seq uint64
		_, err := fmt.Sscanf(filepath.Base(match), prefix+"-%d.sst", &seq)
		if err != nil {
			continue
		}
		if seq >= maxSeq {
			maxSeq = seq + 1
		}
	}

	return maxSeq, nil
}

// Compact merges multiple SSTables into a single new SSTable.
// The merge process:
// 1. Streams rows from all input SSTables
// 2. Deduplicates by entity ID and timestamp (keeps newest)
// 3. Writes merged rows to a new SSTable
// 4. Atomically swaps the old SSTables with the new one
func (e *Executor) Compact(inputPaths []string) (string, error) {
	if len(inputPaths) == 0 {
		return "", fmt.Errorf("no input SSTables provided")
	}

	// Find the highest sequence number from existing files and inputs to avoid collisions.
	inputSet := make(map[string]bool)
	e.seqMu.Lock()
	maxSeq := e.nextSeq
	for _, path := range inputPaths {
		inputSet[path] = true
		var seq uint64
		if _, err := fmt.Sscanf(filepath.Base(path), e.sstablePrefix+"-%d.sst", &seq); err == nil && seq >= maxSeq {
			maxSeq = seq + 1
		}
	}
	pattern := filepath.Join(e.sstableDir, e.sstablePrefix+"-*.sst")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for _, match := range matches {
			// Skip input files
			if inputSet[match] {
				continue
			}
			var seq uint64
			_, err := fmt.Sscanf(filepath.Base(match), e.sstablePrefix+"-%d.sst", &seq)
			if err == nil && seq >= maxSeq {
				maxSeq = seq + 1
			}
		}
	}
	// Use maxSeq for output, and update nextSeq if needed
	if maxSeq >= e.nextSeq {
		e.nextSeq = maxSeq + 1
	}
	outputPath := filepath.Join(e.sstableDir, fmt.Sprintf("%s-%d.sst", e.sstablePrefix, maxSeq))
	tempPath := outputPath + ".tmp"
	e.seqMu.Unlock()

	// Load all input readers
	readers := make([]*sstable.Reader, 0, len(inputPaths))
	for _, path := range inputPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read SSTable %s: %w", path, err)
		}

		reader, err := sstable.NewReader(data)
		if err != nil {
			return "", fmt.Errorf("failed to create reader for %s: %w", path, err)
		}

		readers = append(readers, reader)
	}

	// Create output file with temp name
	outputFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	var finalErr error
	defer func() {
		outputFile.Close()
		// Clean up temp file if compaction fails
		if finalErr != nil {
			os.Remove(tempPath)
		}
	}()

	// Create writer
	writer := sstable.NewWriter(outputFile)

	// Write header
	if err := writer.WriteHeader(); err != nil {
		return "", fmt.Errorf("failed to write header: %w", err)
	}

	// Stream merge and deduplicate rows
	if err := e.streamMergeAndDeduplicate(readers, writer); err != nil {
		finalErr = fmt.Errorf("failed to merge rows: %w", err)
		return "", finalErr
	}

	// Write index and footer
	if err := writer.Close(); err != nil {
		finalErr = fmt.Errorf("failed to close writer: %w", err)
		return "", finalErr
	}

	// Close file before renaming
	if err := outputFile.Close(); err != nil {
		finalErr = fmt.Errorf("failed to close output file: %w", err)
		return "", finalErr
	}

	// Sync directory to ensure file is written
	dir, _ := filepath.Split(tempPath)
	dirFile, err := os.Open(dir)
	if err == nil {
		dirFile.Sync()
		dirFile.Close()
	}

	// Atomically swap: rename temp to final, then delete old files
	// This ensures the new file is visible before old files are removed
	if err := os.Rename(tempPath, outputPath); err != nil {
		return "", fmt.Errorf("failed to rename temp to final: %w", err)
	}

	// Delete old files after successful rename
	for _, path := range inputPaths {
		if err := os.Remove(path); err != nil {
			// Log error but continue - we can clean up later
			_ = err
		}
	}

	return outputPath, nil
}

// streamMergeAndDeduplicate performs a stream merge of rows from multiple readers.
// It processes rows in sorted order (by entity ID) and deduplicates on the fly.
// This is more memory-efficient than loading all rows into memory.
func (e *Executor) streamMergeAndDeduplicate(readers []*sstable.Reader, writer *sstable.Writer) error {
	// Collect all entity IDs from all readers
	allEntityIDs := make(map[sstable.EntityID]bool)
	for _, reader := range readers {
		entityIDs := reader.GetAllEntityIDs()
		for _, id := range entityIDs {
			allEntityIDs[id] = true
		}
	}

	// Convert to sorted slice
	sortedEntityIDs := make([]sstable.EntityID, 0, len(allEntityIDs))
	for id := range allEntityIDs {
		sortedEntityIDs = append(sortedEntityIDs, id)
	}
	sort.Slice(sortedEntityIDs, func(i, j int) bool {
		return sortedEntityIDs[i] < sortedEntityIDs[j]
	})

	// For each entity, get rows from all readers and deduplicate
	for _, entityID := range sortedEntityIDs {
		// Collect all rows for this entity from all readers
		allRows := make([]*sstable.Row, 0)

		// Read from all readers (newer readers first for LSM-tree semantics)
		for i := len(readers) - 1; i >= 0; i-- {
			rows, err := readers[i].Get(entityID)
			if err != nil {
				return fmt.Errorf("failed to get rows for entity %s: %w", entityID, err)
			}
			allRows = append(allRows, rows...)
		}

		if len(allRows) == 0 {
			continue
		}

		// Deduplicate: keep only the newest row
		bestRow := e.selectBestRow(allRows)

		// Write the best row
		if err := writer.WriteRow(bestRow); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// selectBestRow selects the best row from a list of rows for the same entity.
// Prefers rows with newer timestamps. If timestamps are not available or equal,
// prefers rows from newer SSTables (later in the readers list).
func (e *Executor) selectBestRow(rows []*sstable.Row) *sstable.Row {
	if len(rows) == 0 {
		return nil
	}
	if len(rows) == 1 {
		return rows[0]
	}

	// Try to extract timestamps from row data
	// The timestamp is typically stored in the row data
	// For now, we'll use a simple heuristic: prefer rows with more data (likely newer)
	// In a real implementation, you'd parse the row data to extract actual timestamps

	bestRow := rows[0]
	bestTimestamp := e.extractTimestamp(bestRow)

	for i := 1; i < len(rows); i++ {
		timestamp := e.extractTimestamp(rows[i])
		if timestamp > bestTimestamp {
			bestRow = rows[i]
			bestTimestamp = timestamp
		} else if timestamp == bestTimestamp {
			// If timestamps are equal, prefer row with more data (newer version)
			if len(rows[i].Data) > len(bestRow.Data) {
				bestRow = rows[i]
			}
		}
	}

	return bestRow
}

// extractTimestamp extracts a timestamp from row data.
// This is a simplified version - in practice, you'd parse the actual row format.
// For now, we'll look for a timestamp in the first 8 bytes of data if available.
func (e *Executor) extractTimestamp(row *sstable.Row) int64 {
	if len(row.Data) < 8 {
		return 0
	}

	// Try to read a timestamp from the beginning of the data
	// This assumes the row data format includes a timestamp
	// Adjust based on your actual row data format
	timestamp := int64(binary.LittleEndian.Uint64(row.Data[0:8]))

	// Validate timestamp (reasonable range: 2000-2100)
	minTimestamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	if timestamp >= minTimestamp && timestamp <= maxTimestamp {
		return timestamp
	}

	return 0
}

// GetOutputPath returns the next output path for a compacted SSTable.
func (e *Executor) GetOutputPath() string {
	e.seqMu.Lock()
	defer e.seqMu.Unlock()

	path := filepath.Join(e.sstableDir, fmt.Sprintf("%s-%d.sst", e.sstablePrefix, e.nextSeq))
	e.nextSeq++
	return path
}

