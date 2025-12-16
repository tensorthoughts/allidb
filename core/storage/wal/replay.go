package wal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Replay replays all entries from WAL files in sequence.
// It scans all WAL files in order, validates CRC checksums, and calls the callback
// for each valid entry. Stops immediately on corruption or if the callback returns an error.
//
// The entry factory must be set in the WAL Config (Config.EntryFactory) before calling Replay.
func (w *WAL) Replay(callback func(WALEntry) error) error {
	if w.config.EntryFactory == nil {
		return fmt.Errorf("entry factory must be set in WAL config")
	}
	if callback == nil {
		return fmt.Errorf("callback function is required")
	}

	// Find all WAL files
	pattern := filepath.Join(w.config.Dir, w.config.FilePrefix+"-*.wal")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob WAL files: %w", err)
	}

	if len(matches) == 0 {
		// No WAL files to replay, nothing to do
		return nil
	}

	// Sort files by sequence number to ensure sequential replay
	type fileInfo struct {
		path string
		seq  uint64
	}

	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		var seq uint64
		_, err := fmt.Sscanf(filepath.Base(match), w.config.FilePrefix+"-%d.wal", &seq)
		if err != nil {
			// Skip files that don't match the pattern
			continue
		}
		files = append(files, fileInfo{path: match, seq: seq})
	}

	// Sort by sequence number
	sort.Slice(files, func(i, j int) bool {
		return files[i].seq < files[j].seq
	})

	// Replay each file in sequence
	for _, file := range files {
		if err := w.replayFile(file.path, callback); err != nil {
			return fmt.Errorf("failed to replay file %s: %w", file.path, err)
		}
	}

	return nil
}

// replayFile replays all entries from a single WAL file.
func (w *WAL) replayFile(filePath string, callback func(WALEntry) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer file.Close()

	// Sequential scan of the file
	for {
		// Read entry with CRC validation
		data, err := ReadEntry(file)
		if err != nil {
			if err == io.EOF {
				// End of file, normal termination
				return nil
			}
			// Any other error (including checksum mismatch) is corruption
			return fmt.Errorf("corruption detected: %w", err)
		}

		// Create a new entry instance and deserialize
		entry := w.config.EntryFactory()
		if entry == nil {
			return fmt.Errorf("entry factory returned nil")
		}
		
		if err := entry.FromBytes(data); err != nil {
			return fmt.Errorf("failed to deserialize entry: %w", err)
		}

		// Call the callback
		if err := callback(entry); err != nil {
			return fmt.Errorf("callback returned error: %w", err)
		}
	}
}

