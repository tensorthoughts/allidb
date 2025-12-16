package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// WALEntry represents an entry that can be written to the WAL.
// Implementations must provide methods to serialize and deserialize the entry.
type WALEntry interface {
	// ToBytes serializes the entry to a byte slice.
	ToBytes() []byte
	// FromBytes deserializes a byte slice into the entry.
	// This is used during WAL replay.
	FromBytes(data []byte) error
}

// writeRequest represents a write request from a producer.
type writeRequest struct {
	entry WALEntry
	errCh chan error
}

// Config holds configuration for the WAL.
type Config struct {
	// Dir is the directory where WAL files will be stored.
	Dir string
	// MaxFileSize is the maximum size of a WAL file before rotation (in bytes).
	MaxFileSize int64
	// FilePrefix is the prefix for WAL file names (default: "wal").
	FilePrefix string
	// Fsync determines whether to fsync after each write (default: true).
	// If false, writes may be buffered by the OS for better performance.
	Fsync bool
	// EntryFactory creates new WALEntry instances for deserialization during replay.
	// If nil, Replay will return an error.
	EntryFactory func() WALEntry
}

// DefaultConfig returns a default WAL configuration.
func DefaultConfig(dir string) Config {
	return Config{
		Dir:         dir,
		MaxFileSize: 100 * 1024 * 1024, // 100MB default
		FilePrefix:  "wal",
		Fsync:       true, // Default to fsync for durability
	}
}

// WAL is a high-throughput Write Ahead Log implementation.
type WAL struct {
	config Config

	// Current file state
	currentFile   *os.File
	currentSize   int64
	fileSequence  uint64
	filePath      string

	// Channel for write requests
	writeCh chan writeRequest

	// Synchronization
	closeCh chan struct{}
	doneCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
	closed  bool
}

// New creates a new WAL instance.
func New(config Config) (*WAL, error) {
	// Ensure directory exists
	if err := os.MkdirAll(config.Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	w := &WAL{
		config:   config,
		writeCh:  make(chan writeRequest, 1000), // Buffered channel for throughput
		closeCh:  make(chan struct{}),
		doneCh:   make(chan struct{}),
		fileSequence: 0,
	}

	// Find the latest WAL file or create a new one
	if err := w.openOrCreateFile(); err != nil {
		return nil, fmt.Errorf("failed to open/create WAL file: %w", err)
	}

	// Start the writer goroutine
	w.wg.Add(1)
	go w.writer()

	return w, nil
}

// openOrCreateFile opens the latest WAL file or creates a new one.
func (w *WAL) openOrCreateFile() error {
	// Find the latest WAL file
	pattern := filepath.Join(w.config.Dir, w.config.FilePrefix+"-*.wal")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob WAL files: %w", err)
	}

	var latestSeq uint64 = 0
	var latestPath string

	for _, match := range matches {
		var seq uint64
		_, err := fmt.Sscanf(filepath.Base(match), w.config.FilePrefix+"-%d.wal", &seq)
		if err != nil {
			continue
		}
		if seq >= latestSeq {
			latestSeq = seq
			latestPath = match
		}
	}

	// If we found a file, check its size
	if latestPath != "" {
		info, err := os.Stat(latestPath)
		if err == nil && info.Size() < w.config.MaxFileSize {
			// Open existing file for appending
			file, err := os.OpenFile(latestPath, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("failed to open existing WAL file: %w", err)
			}
			w.currentFile = file
			w.currentSize = info.Size()
			w.fileSequence = latestSeq
			w.filePath = latestPath
			return nil
		}
		// File exists but is too large, increment sequence
		latestSeq++
	} else {
		// No existing file, start at 0
		latestSeq = 0
	}

	// Create new file
	return w.rotateFile(latestSeq)
}

// rotateFile creates a new WAL file with the given sequence number.
func (w *WAL) rotateFile(seq uint64) error {
	// Close current file if open
	if w.currentFile != nil {
		if err := w.currentFile.Close(); err != nil {
			return fmt.Errorf("failed to close current WAL file: %w", err)
		}
	}

	// Create new file
	fileName := fmt.Sprintf("%s-%d.wal", w.config.FilePrefix, seq)
	filePath := filepath.Join(w.config.Dir, fileName)
	
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new WAL file: %w", err)
	}

	w.currentFile = file
	w.currentSize = 0
	w.fileSequence = seq
	w.filePath = filePath

	return nil
}

// writer is the single writer goroutine that processes write requests.
func (w *WAL) writer() {
	defer w.wg.Done()
	defer close(w.doneCh)

	for {
		select {
		case <-w.closeCh:
			// Flush any remaining writes
			w.flushPending()
			// Close the current file
			if w.currentFile != nil {
				// Always sync on close to ensure data is persisted
				if w.config.Fsync {
					w.currentFile.Sync()
				}
				w.currentFile.Close()
			}
			return

		case req := <-w.writeCh:
			err := w.writeEntry(req.entry)
			req.errCh <- err
		}
	}
}

// flushPending flushes any pending writes in the channel.
func (w *WAL) flushPending() {
	for {
		select {
		case req := <-w.writeCh:
			err := w.writeEntry(req.entry)
			req.errCh <- err
		default:
			return
		}
	}
}

// writeEntry writes a single entry to the current WAL file.
func (w *WAL) writeEntry(entry WALEntry) error {
	data := entry.ToBytes()
	
	// Check if we need to rotate
	if w.currentSize+int64(8+4+len(data)) > w.config.MaxFileSize {
		if err := w.rotateFile(w.fileSequence + 1); err != nil {
			return fmt.Errorf("failed to rotate WAL file: %w", err)
		}
	}

	// Calculate CRC32 checksum
	checksum := crc32.ChecksumIEEE(data)

	// Write entry length (4 bytes)
	length := uint32(len(data))
	if err := binary.Write(w.currentFile, binary.LittleEndian, length); err != nil {
		return fmt.Errorf("failed to write entry length: %w", err)
	}

	// Write checksum (4 bytes)
	if err := binary.Write(w.currentFile, binary.LittleEndian, checksum); err != nil {
		return fmt.Errorf("failed to write checksum: %w", err)
	}

	// Write entry data
	n, err := w.currentFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write entry data: %w", err)
	}

	w.currentSize += int64(8 + n) // 4 bytes length + 4 bytes checksum + data

	// fsync before acknowledging if configured
	if w.config.Fsync {
		if err := w.currentFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL file: %w", err)
		}
	}

	return nil
}

// Append appends an entry to the WAL.
// This method is safe to call from multiple goroutines.
func (w *WAL) Append(entry WALEntry) error {
	w.mu.RLock()
	if w.closed {
		w.mu.RUnlock()
		return fmt.Errorf("WAL is closed")
	}
	w.mu.RUnlock()

	errCh := make(chan error, 1)
	req := writeRequest{
		entry: entry,
		errCh: errCh,
	}

	select {
	case w.writeCh <- req:
		return <-errCh
	case <-w.doneCh:
		return fmt.Errorf("WAL writer is closed")
	}
}

// Close closes the WAL and waits for all pending writes to complete.
func (w *WAL) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	close(w.closeCh)
	w.wg.Wait()

	return nil
}

// ReadEntry reads a single entry from a WAL file at the current position.
// This is a utility function for reading/replaying WAL files.
func ReadEntry(reader io.Reader) ([]byte, error) {
	// Read entry length
	var length uint32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read entry length: %w", err)
	}

	// Read checksum
	var checksum uint32
	if err := binary.Read(reader, binary.LittleEndian, &checksum); err != nil {
		return nil, fmt.Errorf("failed to read checksum: %w", err)
	}

	// Read entry data
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read entry data: %w", err)
	}

	// Verify checksum
	calculatedChecksum := crc32.ChecksumIEEE(data)
	if calculatedChecksum != checksum {
		return nil, fmt.Errorf("checksum mismatch: expected %d, got %d", checksum, calculatedChecksum)
	}

	return data, nil
}

