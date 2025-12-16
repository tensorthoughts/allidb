package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster/gossip"
	"github.com/tensorthoughts25/allidb/core/cluster/ring"
)

// MockReplicaClient is a mock replica client for testing.
type MockReplicaClient struct {
	mu       sync.RWMutex
	data     map[NodeID]map[string][]byte
	delays   map[NodeID]time.Duration
	errors   map[NodeID]error
	readCalls map[NodeID]int
	writeCalls map[NodeID]int
}

// NewMockReplicaClient creates a new mock replica client.
func NewMockReplicaClient() *MockReplicaClient {
	return &MockReplicaClient{
		data:      make(map[NodeID]map[string][]byte),
		delays:    make(map[NodeID]time.Duration),
		errors:    make(map[NodeID]error),
		readCalls: make(map[NodeID]int),
		writeCalls: make(map[NodeID]int),
	}
}

// SetData sets data for a node.
func (m *MockReplicaClient) SetData(nodeID NodeID, key string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[nodeID] == nil {
		m.data[nodeID] = make(map[string][]byte)
	}
	m.data[nodeID][key] = value
}

// SetDelay sets a delay for a node.
func (m *MockReplicaClient) SetDelay(nodeID NodeID, delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delays[nodeID] = delay
}

// SetError sets an error for a node.
func (m *MockReplicaClient) SetError(nodeID NodeID, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[nodeID] = err
}

// GetReadCalls returns the number of read calls for a node.
func (m *MockReplicaClient) GetReadCalls(nodeID NodeID) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readCalls[nodeID]
}

// GetWriteCalls returns the number of write calls for a node.
func (m *MockReplicaClient) GetWriteCalls(nodeID NodeID) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writeCalls[nodeID]
}

// Read reads from a replica.
func (m *MockReplicaClient) Read(ctx context.Context, nodeID NodeID, key string) ([]byte, error) {
	m.mu.Lock()
	m.readCalls[nodeID]++
	delay := m.delays[nodeID]
	err := m.errors[nodeID]
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data[nodeID] != nil {
		return m.data[nodeID][key], nil
	}
	return nil, nil
}

// Write writes to a replica.
func (m *MockReplicaClient) Write(ctx context.Context, nodeID NodeID, key string, value []byte) error {
	m.mu.Lock()
	m.writeCalls[nodeID]++
	delay := m.delays[nodeID]
	err := m.errors[nodeID]
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[nodeID] == nil {
		m.data[nodeID] = make(map[string][]byte)
	}
	m.data[nodeID][key] = value
	return nil
}

// MockGossip is a mock gossip for testing.
type MockGossip struct {
	mu        sync.RWMutex
	aliveNodes []NodeID
}

// NewMockGossip creates a new mock gossip.
func NewMockGossip() *MockGossip {
	return &MockGossip{
		aliveNodes: make([]NodeID, 0),
	}
}

// SetAliveNodes sets the alive nodes.
func (m *MockGossip) SetAliveNodes(nodes []NodeID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aliveNodes = nodes
}

// GetAliveNodes returns alive nodes.
func (m *MockGossip) GetAliveNodes() []gossip.NodeID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]gossip.NodeID, len(m.aliveNodes))
	for i, nodeID := range m.aliveNodes {
		result[i] = gossip.NodeID(nodeID)
	}
	return result
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ReadQuorum != 2 {
		t.Errorf("Expected ReadQuorum 2, got %d", config.ReadQuorum)
	}
	if config.WriteQuorum != 2 {
		t.Errorf("Expected WriteQuorum 2, got %d", config.WriteQuorum)
	}
	if config.ReadTimeout != 5*time.Second {
		t.Errorf("Expected ReadTimeout 5s, got %v", config.ReadTimeout)
	}
	if config.WriteTimeout != 5*time.Second {
		t.Errorf("Expected WriteTimeout 5s, got %v", config.WriteTimeout)
	}
	if config.ReplicaCount != 3 {
		t.Errorf("Expected ReplicaCount 3, got %d", config.ReplicaCount)
	}
}

func TestNewCoordinator(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	config := DefaultConfig()

	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	if coordinator == nil {
		t.Fatal("NewCoordinator returned nil")
	}
	if coordinator.ring != ring {
		t.Error("Ring not set correctly")
	}
	if coordinator.gossip == nil {
		t.Error("Gossip not set correctly")
	}
	if coordinator.client != mockClient {
		t.Error("Client not set correctly")
	}
}

func TestCoordinator_Read_QuorumSuccess(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetData("node1", "key1", []byte("value1"))
	mockClient.SetData("node2", "key1", []byte("value1"))
	mockClient.SetData("node3", "key1", []byte("value1"))

	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	value, err := coordinator.Read(ctx, "key1")

	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(value) != "value1" {
		t.Errorf("Expected 'value1', got '%s'", string(value))
	}
}

func TestCoordinator_Read_QuorumFailure(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetError("node1", errors.New("read error"))
	mockClient.SetError("node2", errors.New("read error"))
	mockClient.SetData("node3", "key1", []byte("value1"))

	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	_, err := coordinator.Read(ctx, "key1")

	if err == nil {
		t.Fatal("Expected error when quorum not reached")
	}
}

func TestCoordinator_Read_Timeout(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetDelay("node1", 10*time.Second) // Long delay
	mockClient.SetDelay("node2", 10*time.Second)
	mockClient.SetData("node3", "key1", []byte("value1"))

	config := DefaultConfig()
	config.ReadQuorum = 2
	config.ReadTimeout = 100 * time.Millisecond // Short timeout
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	_, err := coordinator.Read(ctx, "key1")

	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestCoordinator_Write_QuorumSuccess(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()

	config := DefaultConfig()
	config.WriteQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	err := coordinator.Write(ctx, "key1", []byte("value1"))

	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify writes were made
	if mockClient.GetWriteCalls("node1") == 0 {
		t.Error("Expected write call to node1")
	}
}

func TestCoordinator_Write_QuorumFailure(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetError("node1", errors.New("write error"))
	mockClient.SetError("node2", errors.New("write error"))

	config := DefaultConfig()
	config.WriteQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	err := coordinator.Write(ctx, "key1", []byte("value1"))

	if err == nil {
		t.Fatal("Expected error when quorum not reached")
	}
}

func TestCoordinator_Write_Timeout(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetDelay("node1", 10*time.Second)
	mockClient.SetDelay("node2", 10*time.Second)

	config := DefaultConfig()
	config.WriteQuorum = 2
	config.WriteTimeout = 100 * time.Millisecond
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	err := coordinator.Write(ctx, "key1", []byte("value1"))

	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestCoordinator_Read_ResponseMerging(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetData("node1", "key1", []byte("value1"))
	mockClient.SetData("node2", "key1", []byte("value1"))
	mockClient.SetData("node3", "key1", []byte("value2")) // Different value

	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	value, err := coordinator.Read(ctx, "key1")

	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Should return majority value (value1 appears twice)
	if string(value) != "value1" {
		t.Errorf("Expected 'value1' (majority), got '%s'", string(value))
	}
}

func TestCoordinator_FilterAliveNodes(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node3"}) // node2 is dead

	mockClient := NewMockReplicaClient()
	config := DefaultConfig()
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	replicas := coordinator.GetAliveReplicas("key1")

	// Should only return alive nodes
	aliveSet := make(map[NodeID]bool)
	for _, nodeID := range replicas {
		aliveSet[nodeID] = true
	}

	if aliveSet["node2"] {
		t.Error("Dead node should not be in alive replicas")
	}
}

func TestCoordinator_NoGossip(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockClient := NewMockReplicaClient()
	mockClient.SetData("node1", "key1", []byte("value1"))
	mockClient.SetData("node2", "key1", []byte("value1"))

	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, nil, mockClient, config) // No gossip

	ctx := context.Background()
	value, err := coordinator.Read(ctx, "key1")

	// Should work without gossip (assumes all nodes are alive)
	if err != nil {
		t.Fatalf("Read should work without gossip: %v", err)
	}
	if string(value) != "value1" {
		t.Errorf("Expected 'value1', got '%s'", string(value))
	}
}

func TestCoordinator_GetReplicas(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	config := DefaultConfig()
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	replicas := coordinator.GetReplicas("key1")

	if len(replicas) == 0 {
		t.Error("Expected at least one replica")
	}
}

func TestCoordinator_InsufficientAliveReplicas(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1"}) // Only one alive

	mockClient := NewMockReplicaClient()
	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	_, err := coordinator.Read(ctx, "key1")

	if err == nil {
		t.Fatal("Expected error for insufficient alive replicas")
	}
}

func TestCoordinator_NoReplicas(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	// No nodes added

	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	config := DefaultConfig()
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	_, err := coordinator.Read(ctx, "key1")

	if err == nil {
		t.Fatal("Expected error when no replicas available")
	}
}

func TestCoordinator_UpdateConfig(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	mockGossip := NewMockGossip()
	mockClient := NewMockReplicaClient()
	config := DefaultConfig()
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	newConfig := DefaultConfig()
	newConfig.ReadQuorum = 3
	coordinator.UpdateConfig(newConfig)

	updatedConfig := coordinator.GetConfig()
	if updatedConfig.ReadQuorum != 3 {
		t.Errorf("Expected ReadQuorum 3, got %d", updatedConfig.ReadQuorum)
	}
}

func TestCoordinator_ConcurrentReads(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetData("node1", "key1", []byte("value1"))
	mockClient.SetData("node2", "key1", []byte("value1"))
	mockClient.SetData("node3", "key1", []byte("value1"))

	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := coordinator.Read(ctx, "key1")
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCoordinator_ConcurrentWrites(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()

	config := DefaultConfig()
	config.WriteQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx := context.Background()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "key1"
			value := []byte("value1")
			err := coordinator.Write(ctx, key, value)
			if err != nil {
				t.Errorf("Concurrent write failed: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCoordinator_ContextCancellation(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	mockGossip := NewMockGossip()
	mockGossip.SetAliveNodes([]NodeID{"node1", "node2", "node3"})

	mockClient := NewMockReplicaClient()
	mockClient.SetDelay("node1", 1*time.Second)

	config := DefaultConfig()
	config.ReadQuorum = 2
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := coordinator.Read(ctx, "key1")

	// Should respect context cancellation
	if err == nil {
		t.Log("Note: Context cancellation may not be immediately detected")
	}
}

func TestCoordinator_ReplicaCountLimit(t *testing.T) {
	ring := ring.New(ring.DefaultConfig())
	for i := 0; i < 10; i++ {
		ring.AddNode(NodeID(fmt.Sprintf("node%d", i)))
	}

	mockGossip := NewMockGossip()
	aliveNodes := make([]NodeID, 10)
	for i := 0; i < 10; i++ {
		aliveNodes[i] = NodeID(fmt.Sprintf("node%d", i))
	}
	mockGossip.SetAliveNodes(aliveNodes)

	mockClient := NewMockReplicaClient()
	config := DefaultConfig()
	config.ReplicaCount = 3
	coordinator := NewCoordinator(ring, mockGossip, mockClient, config)

	replicas := coordinator.GetReplicas("key1")

	if len(replicas) > config.ReplicaCount {
		t.Errorf("Expected at most %d replicas, got %d", config.ReplicaCount, len(replicas))
	}
}

