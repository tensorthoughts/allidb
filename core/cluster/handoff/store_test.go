package handoff

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster"
)

func TestStore_StoreHint(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")
	key := "test-key"
	value := []byte("test-value")

	if err := store.StoreHint(nodeID, key, value); err != nil {
		t.Fatalf("Failed to store hint: %v", err)
	}

	hints := store.GetHintsForNode(nodeID)
	if len(hints) != 1 {
		t.Fatalf("Expected 1 hint, got %d", len(hints))
	}

	if hints[0].Key != key {
		t.Errorf("Expected key %s, got %s", key, hints[0].Key)
	}

	if string(hints[0].Value) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(hints[0].Value))
	}

	if hints[0].TargetNodeID != nodeID {
		t.Errorf("Expected target node %s, got %s", nodeID, hints[0].TargetNodeID)
	}
}

func TestStore_StoreHint_MultipleNodes(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	node1 := cluster.NodeID("node1")
	node2 := cluster.NodeID("node2")

	store.StoreHint(node1, "key1", []byte("value1"))
	store.StoreHint(node2, "key2", []byte("value2"))
	store.StoreHint(node1, "key3", []byte("value3"))

	hints1 := store.GetHintsForNode(node1)
	if len(hints1) != 2 {
		t.Fatalf("Expected 2 hints for node1, got %d", len(hints1))
	}

	hints2 := store.GetHintsForNode(node2)
	if len(hints2) != 1 {
		t.Fatalf("Expected 1 hint for node2, got %d", len(hints2))
	}
}

func TestStore_RemoveHint(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))
	store.StoreHint(nodeID, "key2", []byte("value2"))

	if err := store.RemoveHint(nodeID, "key1"); err != nil {
		t.Fatalf("Failed to remove hint: %v", err)
	}

	hints := store.GetHintsForNode(nodeID)
	if len(hints) != 1 {
		t.Fatalf("Expected 1 hint, got %d", len(hints))
	}

	if hints[0].Key != "key2" {
		t.Errorf("Expected remaining hint to be key2, got %s", hints[0].Key)
	}
}

func TestStore_RemoveHintsForNode(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))
	store.StoreHint(nodeID, "key2", []byte("value2"))

	if err := store.RemoveHintsForNode(nodeID); err != nil {
		t.Fatalf("Failed to remove hints: %v", err)
	}

	hints := store.GetHintsForNode(nodeID)
	if len(hints) != 0 {
		t.Fatalf("Expected 0 hints, got %d", len(hints))
	}
}

func TestStore_MaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	config.MaxSize = 100 // Very small size limit
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")
	largeValue := make([]byte, 200) // Larger than max size

	err = store.StoreHint(nodeID, "key1", largeValue)
	if err == nil {
		t.Fatal("Expected error when storing hint larger than max size")
	}
}

func TestStore_TTL(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	config.MaxTTL = 100 * time.Millisecond
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Trigger cleanup
	store.cleanup()

	hints := store.GetHintsForNode(nodeID)
	if len(hints) != 0 {
		t.Fatalf("Expected 0 hints after TTL expiration, got %d", len(hints))
	}
}

func TestStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)

	// Create store and add hints
	store1, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	nodeID := cluster.NodeID("node1")
	store1.StoreHint(nodeID, "key1", []byte("value1"))
	store1.StoreHint(nodeID, "key2", []byte("value2"))
	store1.Close()

	// Reopen store
	store2, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	hints := store2.GetHintsForNode(nodeID)
	if len(hints) != 2 {
		t.Fatalf("Expected 2 hints after reload, got %d", len(hints))
	}
}

func TestStore_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	node1 := cluster.NodeID("node1")
	node2 := cluster.NodeID("node2")

	store.StoreHint(node1, "key1", []byte("value1"))
	store.StoreHint(node1, "key2", []byte("value2"))
	store.StoreHint(node2, "key3", []byte("value3"))

	stats := store.GetStats()
	if stats.TotalHints != 3 {
		t.Errorf("Expected 3 total hints, got %d", stats.TotalHints)
	}

	if stats.NodesWithHints != 2 {
		t.Errorf("Expected 2 nodes with hints, got %d", stats.NodesWithHints)
	}

	if stats.HintsStored != 3 {
		t.Errorf("Expected 3 hints stored, got %d", stats.HintsStored)
	}
}

func TestStore_CRC32(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")
	key := "test-key"
	value := []byte("test-value")

	store.StoreHint(nodeID, key, value)

	hints := store.GetHintsForNode(nodeID)
	if len(hints) == 0 {
		t.Fatal("Expected at least one hint")
	}

	hint := hints[0]
	expectedCRC := store.CalculateCRC(hint)

	if hint.CRC32 != expectedCRC {
		t.Errorf("CRC32 mismatch: expected %d, got %d", expectedCRC, hint.CRC32)
	}
}

func TestStore_GetNodesWithHints(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	node1 := cluster.NodeID("node1")
	node2 := cluster.NodeID("node2")
	node3 := cluster.NodeID("node3")

	store.StoreHint(node1, "key1", []byte("value1"))
	store.StoreHint(node2, "key2", []byte("value2"))
	store.StoreHint(node1, "key3", []byte("value3"))

	nodes := store.GetNodesWithHints()
	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	nodeSet := make(map[cluster.NodeID]bool)
	for _, node := range nodes {
		nodeSet[node] = true
	}

	if !nodeSet[node1] || !nodeSet[node2] {
		t.Error("Expected node1 and node2 to be in the set")
	}

	if nodeSet[node3] {
		t.Error("Did not expect node3 to be in the set")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	nodeID := cluster.NodeID("node1")

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := "key" + string(rune(idx))
			value := []byte("value" + string(rune(idx)))
			store.StoreHint(nodeID, key, value)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	hints := store.GetHintsForNode(nodeID)
	if len(hints) != 10 {
		t.Fatalf("Expected 10 hints, got %d", len(hints))
	}
}

func TestStore_LoadCorruptedHints(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig(tmpDir)

	// Create a corrupted hint file
	hintFile := filepath.Join(tmpDir, "hints.db")
	file, err := os.Create(hintFile)
	if err != nil {
		t.Fatalf("Failed to create hint file: %v", err)
	}
	file.WriteString("invalid json data\n")
	file.Close()

	// Should still load successfully (skips corrupted entries)
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store with corrupted file: %v", err)
	}
	defer store.Close()

	// Store should be empty
	stats := store.GetStats()
	if stats.TotalHints != 0 {
		t.Errorf("Expected 0 hints after loading corrupted file, got %d", stats.TotalHints)
	}
}

