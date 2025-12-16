package handoff

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster"
)

// Hint represents a write that needs to be replayed to a node.
type Hint struct {
	TargetNodeID cluster.NodeID
	Key          string
	Value        []byte
	Timestamp    time.Time
	CRC32        uint32
}

// Store is a persistent store for hinted handoff writes.
// Hints are stored when target nodes are unavailable and replayed when they come back online.
type Store struct {
	mu sync.RWMutex

	// Configuration
	config Config

	// In-memory index: nodeID -> []hint
	hints map[cluster.NodeID][]*Hint

	// Total size of all hints in bytes
	totalSize int64

	// File handle for persistence
	file *os.File
	filePath string

	// Background goroutine management
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Statistics
	hintsStored   int64
	hintsReplayed int64
	hintsExpired  int64
	hintsDropped  int64
}

// Config holds configuration for the hint store.
type Config struct {
	DataDir      string        // Directory for hint files
	MaxSize      int64         // Maximum total size of hints in bytes (default: 1GB)
	MaxTTL       time.Duration // Maximum time to keep hints (default: 24 hours)
	FlushInterval time.Duration // How often to flush to disk (default: 1 second)
}

// DefaultConfig returns a default configuration.
func DefaultConfig(dataDir string) Config {
	return Config{
		DataDir:       dataDir,
		MaxSize:       1 * 1024 * 1024 * 1024, // 1GB
		MaxTTL:        24 * time.Hour,
		FlushInterval: 1 * time.Second,
	}
}

// NewStore creates a new hinted handoff store.
func NewStore(config Config) (*Store, error) {
	if config.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	store := &Store{
		config:   config,
		hints:    make(map[cluster.NodeID][]*Hint),
		filePath: filepath.Join(config.DataDir, "hints.db"),
		stopCh:   make(chan struct{}),
	}

	// Load existing hints from disk
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load hints: %w", err)
	}

	// Start background cleanup goroutine
	store.wg.Add(1)
	go store.cleanupLoop()

	return store, nil
}

// StoreHint stores a hint for a target node.
// Returns an error if the store is full or the hint is too old.
func (s *Store) StoreHint(targetNodeID cluster.NodeID, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we have space
	hintSize := s.estimateHintSize(key, value)
	if s.totalSize+int64(hintSize) > s.config.MaxSize {
		s.hintsDropped++
		return fmt.Errorf("hint store is full (size: %d, max: %d)", s.totalSize, s.config.MaxSize)
	}

	// Create hint
	hint := &Hint{
		TargetNodeID: targetNodeID,
		Key:          key,
		Value:        make([]byte, len(value)),
		Timestamp:    time.Now(),
	}
	copy(hint.Value, value)
	hint.CRC32 = s.calculateCRC(hint)

	// Add to in-memory index
	s.hints[targetNodeID] = append(s.hints[targetNodeID], hint)
	s.totalSize += int64(hintSize)
	s.hintsStored++

	// Persist to disk (async flush)
	go s.flush()

	return nil
}

// GetHintsForNode returns all hints for a specific node.
func (s *Store) GetHintsForNode(nodeID cluster.NodeID) []*Hint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hints := s.hints[nodeID]
	if len(hints) == 0 {
		return nil
	}

	// Return a copy
	result := make([]*Hint, len(hints))
	for i, hint := range hints {
		result[i] = &Hint{
			TargetNodeID: hint.TargetNodeID,
			Key:          hint.Key,
			Value:        make([]byte, len(hint.Value)),
			Timestamp:    hint.Timestamp,
			CRC32:        hint.CRC32,
		}
		copy(result[i].Value, hint.Value)
	}
	return result
}

// RemoveHint removes a specific hint from the store.
func (s *Store) RemoveHint(nodeID cluster.NodeID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hints := s.hints[nodeID]
	if len(hints) == 0 {
		return nil
	}

	// Find and remove the hint
	newHints := make([]*Hint, 0, len(hints))
	for _, hint := range hints {
		if hint.Key != key {
			newHints = append(newHints, hint)
		} else {
			// Remove this hint
			hintSize := s.estimateHintSize(hint.Key, hint.Value)
			s.totalSize -= int64(hintSize)
		}
	}

	if len(newHints) == 0 {
		delete(s.hints, nodeID)
	} else {
		s.hints[nodeID] = newHints
	}

	// Persist to disk
	go s.flush()

	return nil
}

// RemoveHintsForNode removes all hints for a specific node.
func (s *Store) RemoveHintsForNode(nodeID cluster.NodeID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hints := s.hints[nodeID]
	if len(hints) == 0 {
		return nil
	}

	// Calculate total size to remove
	for _, hint := range hints {
		hintSize := s.estimateHintSize(hint.Key, hint.Value)
		s.totalSize -= int64(hintSize)
	}

	delete(s.hints, nodeID)

	// Persist to disk
	go s.flush()

	return nil
}

// GetStats returns store statistics.
func (s *Store) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalHints := 0
	for _, hints := range s.hints {
		totalHints += len(hints)
	}

	return Stats{
		TotalHints:    int64(totalHints),
		TotalSize:     s.totalSize,
		HintsStored:   s.hintsStored,
		HintsReplayed: s.hintsReplayed,
		HintsExpired:  s.hintsExpired,
		HintsDropped:  s.hintsDropped,
		NodesWithHints: int64(len(s.hints)),
	}
}

// Stats holds store statistics.
type Stats struct {
	TotalHints     int64
	TotalSize      int64
	HintsStored    int64
	HintsReplayed  int64
	HintsExpired   int64
	HintsDropped   int64
	NodesWithHints int64
}

// Close closes the store and flushes all data to disk.
func (s *Store) Close() error {
	// Stop cleanup goroutine
	close(s.stopCh)
	s.wg.Wait()

	if err := s.flush(); err != nil {
		return fmt.Errorf("failed to flush on close: %w", err)
	}

	return nil
}

// cleanupLoop periodically removes expired hints.
func (s *Store) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes expired hints.
func (s *Store) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for nodeID, hints := range s.hints {
		newHints := make([]*Hint, 0, len(hints))
		for _, hint := range hints {
			age := now.Sub(hint.Timestamp)
			if age > s.config.MaxTTL {
				// Expired - remove it
				hintSize := s.estimateHintSize(hint.Key, hint.Value)
				s.totalSize -= int64(hintSize)
				expiredCount++
				s.hintsExpired++
			} else {
				newHints = append(newHints, hint)
			}
		}

		if len(newHints) == 0 {
			delete(s.hints, nodeID)
		} else {
			s.hints[nodeID] = newHints
		}
	}

	if expiredCount > 0 {
		// Persist to disk
		go s.flush()
	}
}

// estimateHintSize estimates the size of a hint in bytes.
func (s *Store) estimateHintSize(key string, value []byte) int {
	// Rough estimate: nodeID + key + value + timestamp + CRC32
	return len(string(cluster.NodeID(""))) + len(key) + len(value) + 8 + 4 + 16 // overhead
}

// calculateCRC calculates the CRC32 checksum for a hint.
func (s *Store) calculateCRC(hint *Hint) uint32 {
	data := make([]byte, 0, len(hint.Key)+len(hint.Value)+8)
	data = append(data, []byte(hint.Key)...)
	data = append(data, hint.Value...)
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(hint.Timestamp.UnixNano()))
	data = append(data, timestampBytes...)
	return crc32.ChecksumIEEE(data)
}

// load loads hints from disk.
func (s *Store) load() error {
	file, err := os.OpenFile(s.filePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open hint file: %w", err)
	}
	defer file.Close()

	// Read file line by line (JSON per line). Decoder on a bad token can stick;
	// using Scanner avoids infinite loops on corrupted lines.
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var hint Hint
		if err := json.Unmarshal(line, &hint); err != nil {
			// Skip malformed entry
			continue
		}

		// Validate CRC
		expectedCRC := s.calculateCRC(&hint)
		if hint.CRC32 != expectedCRC {
			// Skip corrupted hints
			continue
		}

		// Check if expired
		age := time.Since(hint.Timestamp)
		if age > s.config.MaxTTL {
			s.hintsExpired++
			continue
		}

		// Add to in-memory index
		s.hints[hint.TargetNodeID] = append(s.hints[hint.TargetNodeID], &hint)
		s.totalSize += int64(s.estimateHintSize(hint.Key, hint.Value))
	}

	// Ignore scanner error except EOF (already handled)
	return nil
}

// flush flushes hints to disk.
func (s *Store) flush() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Open file for writing (truncate)
	file, err := os.OpenFile(s.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open hint file for writing: %w", err)
	}
	defer file.Close()

	// Write all hints as JSON
	encoder := json.NewEncoder(file)
	for _, hints := range s.hints {
		for _, hint := range hints {
			if err := encoder.Encode(hint); err != nil {
				return fmt.Errorf("failed to encode hint: %w", err)
			}
		}
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync hint file: %w", err)
	}

	return nil
}

// MarkHintReplayed marks a hint as replayed (for statistics).
func (s *Store) MarkHintReplayed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hintsReplayed++
}

// GetNodesWithHints returns all node IDs that have hints stored.
func (s *Store) GetNodesWithHints() []cluster.NodeID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]cluster.NodeID, 0, len(s.hints))
	for nodeID := range s.hints {
		nodes = append(nodes, nodeID)
	}
	return nodes
}

// CalculateCRC calculates the CRC32 checksum for a hint.
// This is exported for use by the dispatcher.
func (s *Store) CalculateCRC(hint *Hint) uint32 {
	return s.calculateCRC(hint)
}

