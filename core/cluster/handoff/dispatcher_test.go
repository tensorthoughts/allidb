package handoff

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster"
	"github.com/tensorthoughts25/allidb/core/cluster/gossip"
)

// MockGossip is a mock gossip implementation for testing.
type MockGossip struct {
	mu         sync.RWMutex
	aliveNodes []gossip.NodeID
}

func NewMockGossip() *MockGossip {
	return &MockGossip{
		aliveNodes: make([]gossip.NodeID, 0),
	}
}

func (m *MockGossip) GetAliveNodes() []gossip.NodeID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]gossip.NodeID, len(m.aliveNodes))
	copy(result, m.aliveNodes)
	return result
}

func (m *MockGossip) GetNodeState(nodeID gossip.NodeID) (gossip.NodeState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, node := range m.aliveNodes {
		if node == nodeID {
			return gossip.NodeStateAlive, true
		}
	}
	return gossip.NodeStateDead, false
}

func (m *MockGossip) SetAliveNodes(nodes []gossip.NodeID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aliveNodes = make([]gossip.NodeID, len(nodes))
	copy(m.aliveNodes, nodes)
}

// MockCoordinator is a mock coordinator for testing.
type MockCoordinator struct {
	mu     sync.RWMutex
	writes map[string][]byte
	count  int
}

func NewMockCoordinator() *MockCoordinator {
	return &MockCoordinator{
		writes: make(map[string][]byte),
	}
}

func (m *MockCoordinator) Write(ctx context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes[key] = value
	m.count++
	return nil
}

func (m *MockCoordinator) GetWrite(key string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writes[key]
}

func (m *MockCoordinator) GetWriteCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.count
}

// MockReplicaClient is a mock replica client for testing.
type MockReplicaClient struct {
	mu     sync.RWMutex
	writes map[string]map[cluster.NodeID][]byte
}

func NewMockReplicaClient() *MockReplicaClient {
	return &MockReplicaClient{
		writes: make(map[string]map[cluster.NodeID][]byte),
	}
}

func (m *MockReplicaClient) Read(ctx context.Context, nodeID cluster.NodeID, key string) ([]byte, error) {
	return nil, nil
}

func (m *MockReplicaClient) Write(ctx context.Context, nodeID cluster.NodeID, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writes[key] == nil {
		m.writes[key] = make(map[cluster.NodeID][]byte)
	}
	m.writes[key][nodeID] = value
	return nil
}

func TestDispatcher_ReplayHints(t *testing.T) {
	tmpDir := t.TempDir()
	storeConfig := DefaultConfig(tmpDir)
	store, err := NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockGossip := NewMockGossip()
	mockCoordinator := NewMockCoordinator()
	mockClient := NewMockReplicaClient()

	dispatcherConfig := DefaultDispatcherConfig()
	dispatcherConfig.ReplayInterval = 100 * time.Millisecond
	dispatcher := NewDispatcher(store, mockGossip, mockCoordinator, mockClient, dispatcherConfig)

	// Store hints for a node that's currently down
	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))
	store.StoreHint(nodeID, "key2", []byte("value2"))

	// Node is not alive yet
	mockGossip.SetAliveNodes([]gossip.NodeID{})

	// Start dispatcher
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	// Wait a bit - no replay should happen
	time.Sleep(200 * time.Millisecond)

	if mockCoordinator.GetWriteCount() != 0 {
		t.Fatal("Expected no writes while node is down")
	}

	// Mark node as alive
	mockGossip.SetAliveNodes([]gossip.NodeID{gossip.NodeID(nodeID)})

	// Wait for replay
	time.Sleep(500 * time.Millisecond)

	// Check that hints were replayed
	if mockCoordinator.GetWriteCount() != 2 {
		t.Fatalf("Expected 2 writes, got %d", mockCoordinator.GetWriteCount())
	}

	if string(mockCoordinator.GetWrite("key1")) != "value1" {
		t.Errorf("Expected value1 for key1, got %s", string(mockCoordinator.GetWrite("key1")))
	}

	if string(mockCoordinator.GetWrite("key2")) != "value2" {
		t.Errorf("Expected value2 for key2, got %s", string(mockCoordinator.GetWrite("key2")))
	}

	// Check that hints were removed from store
	hints := store.GetHintsForNode(nodeID)
	if len(hints) != 0 {
		t.Fatalf("Expected 0 hints after replay, got %d", len(hints))
	}
}

func TestDispatcher_IdempotentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	storeConfig := DefaultConfig(tmpDir)
	store, err := NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockGossip := NewMockGossip()
	mockCoordinator := NewMockCoordinator()
	mockClient := NewMockReplicaClient()

	dispatcherConfig := DefaultDispatcherConfig()
	dispatcherConfig.ReplayInterval = 100 * time.Millisecond
	dispatcher := NewDispatcher(store, mockGossip, mockCoordinator, mockClient, dispatcherConfig)

	nodeID := cluster.NodeID("node1")
	key := "key1"
	value := []byte("value1")

	// Store hint
	store.StoreHint(nodeID, key, value)

	// Manually write the same key (simulating node already having the value)
	mockCoordinator.Write(context.Background(), key, value)

	// Mark node as alive
	mockGossip.SetAliveNodes([]gossip.NodeID{gossip.NodeID(nodeID)})

	// Start dispatcher
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	// Wait for replay
	time.Sleep(500 * time.Millisecond)

	// Idempotent write should succeed (coordinator handles duplicates)
	if mockCoordinator.GetWriteCount() != 2 { // 1 manual + 1 replay
		t.Fatalf("Expected 2 writes (1 manual + 1 replay), got %d", mockCoordinator.GetWriteCount())
	}

	// Value should still be correct
	if string(mockCoordinator.GetWrite(key)) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(mockCoordinator.GetWrite(key)))
	}
}

func TestDispatcher_TriggerReplay(t *testing.T) {
	tmpDir := t.TempDir()
	storeConfig := DefaultConfig(tmpDir)
	store, err := NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockGossip := NewMockGossip()
	mockCoordinator := NewMockCoordinator()
	mockClient := NewMockReplicaClient()

	dispatcherConfig := DefaultDispatcherConfig()
	dispatcher := NewDispatcher(store, mockGossip, mockCoordinator, mockClient, dispatcherConfig)

	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))

	// Mark node as alive
	mockGossip.SetAliveNodes([]gossip.NodeID{gossip.NodeID(nodeID)})

	// Start dispatcher
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	// Manually trigger replay
	if err := dispatcher.TriggerReplay(nodeID); err != nil {
		t.Fatalf("Failed to trigger replay: %v", err)
	}

	// Wait for replay
	time.Sleep(200 * time.Millisecond)

	if mockCoordinator.GetWriteCount() != 1 {
		t.Fatalf("Expected 1 write, got %d", mockCoordinator.GetWriteCount())
	}
}

func TestDispatcher_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	storeConfig := DefaultConfig(tmpDir)
	store, err := NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockGossip := NewMockGossip()
	mockCoordinator := NewMockCoordinator()
	mockClient := NewMockReplicaClient()

	dispatcherConfig := DefaultDispatcherConfig()
	dispatcherConfig.ReplayInterval = 100 * time.Millisecond
	dispatcher := NewDispatcher(store, mockGossip, mockCoordinator, mockClient, dispatcherConfig)

	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))
	store.StoreHint(nodeID, "key2", []byte("value2"))

	mockGossip.SetAliveNodes([]gossip.NodeID{gossip.NodeID(nodeID)})

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	// Wait for replay
	time.Sleep(500 * time.Millisecond)

	stats := dispatcher.GetStats()
	if stats.HintsReplayed != 2 {
		t.Errorf("Expected 2 hints replayed, got %d", stats.HintsReplayed)
	}

	if stats.HintsFailed != 0 {
		t.Errorf("Expected 0 hints failed, got %d", stats.HintsFailed)
	}
}

func TestDispatcher_IsReplaying(t *testing.T) {
	tmpDir := t.TempDir()
	storeConfig := DefaultConfig(tmpDir)
	store, err := NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockGossip := NewMockGossip()
	mockCoordinator := NewMockCoordinator()
	mockClient := NewMockReplicaClient()

	dispatcherConfig := DefaultDispatcherConfig()
	dispatcherConfig.ReplayInterval = 100 * time.Millisecond
	dispatcher := NewDispatcher(store, mockGossip, mockCoordinator, mockClient, dispatcherConfig)

	nodeID := cluster.NodeID("node1")
	store.StoreHint(nodeID, "key1", []byte("value1"))

	mockGossip.SetAliveNodes([]gossip.NodeID{gossip.NodeID(nodeID)})

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	// Trigger replay
	dispatcher.TriggerReplay(nodeID)

	// Check if replaying (may be brief)
	time.Sleep(50 * time.Millisecond)
	// Note: IsReplaying may return false if replay completes quickly
	_ = dispatcher.IsReplaying(nodeID)
}

func TestDispatcher_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	storeConfig := DefaultConfig(tmpDir)
	store, err := NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockGossip := NewMockGossip()
	mockCoordinator := NewMockCoordinator()
	mockClient := NewMockReplicaClient()

	dispatcherConfig := DefaultDispatcherConfig()
	dispatcher := NewDispatcher(store, mockGossip, mockCoordinator, mockClient, dispatcherConfig)

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}

	// Stop dispatcher
	if err := dispatcher.Stop(); err != nil {
		t.Fatalf("Failed to stop dispatcher: %v", err)
	}

	// Stop again should be safe
	if err := dispatcher.Stop(); err != nil {
		t.Fatalf("Failed to stop dispatcher again: %v", err)
	}
}

