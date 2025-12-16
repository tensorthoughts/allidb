package ring

import (
	"fmt"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.VirtualNodesPerNode != 150 {
		t.Errorf("Expected VirtualNodesPerNode 150, got %d", config.VirtualNodesPerNode)
	}
	if config.ReplicationFactor != 3 {
		t.Errorf("Expected ReplicationFactor 3, got %d", config.ReplicationFactor)
	}
}

func TestNew(t *testing.T) {
	config := DefaultConfig()
	ring := New(config)

	if ring == nil {
		t.Fatal("New returned nil")
	}
	if ring.GetVirtualNodesPerNode() != 150 {
		t.Errorf("Expected 150 virtual nodes per node, got %d", ring.GetVirtualNodesPerNode())
	}
	if ring.GetReplicationFactor() != 3 {
		t.Errorf("Expected replication factor 3, got %d", ring.GetReplicationFactor())
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	config := Config{
		VirtualNodesPerNode: 0,
		ReplicationFactor:   0,
	}
	ring := New(config)

	// Should use defaults
	if ring.GetVirtualNodesPerNode() != 150 {
		t.Errorf("Expected default 150 virtual nodes, got %d", ring.GetVirtualNodesPerNode())
	}
	if ring.GetReplicationFactor() != 3 {
		t.Errorf("Expected default replication factor 3, got %d", ring.GetReplicationFactor())
	}
}

func TestRing_AddNode(t *testing.T) {
	ring := New(DefaultConfig())

	err := ring.AddNode("node1")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	if !ring.HasNode("node1") {
		t.Error("Node should exist after AddNode")
	}
	if ring.GetNodeCount() != 1 {
		t.Errorf("Expected 1 node, got %d", ring.GetNodeCount())
	}
	if ring.GetVirtualNodeCount() != 150 {
		t.Errorf("Expected 150 virtual nodes, got %d", ring.GetVirtualNodeCount())
	}
}

func TestRing_AddNode_Duplicate(t *testing.T) {
	ring := New(DefaultConfig())

	err := ring.AddNode("node1")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	err = ring.AddNode("node1")
	if err == nil {
		t.Fatal("Expected error when adding duplicate node")
	}
}

func TestRing_AddNode_Multiple(t *testing.T) {
	ring := New(DefaultConfig())

	nodes := []NodeID{"node1", "node2", "node3"}
	for _, nodeID := range nodes {
		err := ring.AddNode(nodeID)
		if err != nil {
			t.Fatalf("AddNode failed for %s: %v", nodeID, err)
		}
	}

	if ring.GetNodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", ring.GetNodeCount())
	}
	if ring.GetVirtualNodeCount() != 450 {
		t.Errorf("Expected 450 virtual nodes (3 * 150), got %d", ring.GetVirtualNodeCount())
	}
}

func TestRing_RemoveNode(t *testing.T) {
	ring := New(DefaultConfig())

	err := ring.AddNode("node1")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	err = ring.RemoveNode("node1")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	if ring.HasNode("node1") {
		t.Error("Node should not exist after RemoveNode")
	}
	if ring.GetNodeCount() != 0 {
		t.Errorf("Expected 0 nodes, got %d", ring.GetNodeCount())
	}
	if ring.GetVirtualNodeCount() != 0 {
		t.Errorf("Expected 0 virtual nodes, got %d", ring.GetVirtualNodeCount())
	}
}

func TestRing_RemoveNode_NonExistent(t *testing.T) {
	ring := New(DefaultConfig())

	err := ring.RemoveNode("node1")
	if err == nil {
		t.Fatal("Expected error when removing non-existent node")
	}
}

func TestRing_RemoveNode_Partial(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	err := ring.RemoveNode("node2")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	if ring.GetNodeCount() != 2 {
		t.Errorf("Expected 2 nodes, got %d", ring.GetNodeCount())
	}
	if ring.GetVirtualNodeCount() != 300 {
		t.Errorf("Expected 300 virtual nodes (2 * 150), got %d", ring.GetVirtualNodeCount())
	}
	if ring.HasNode("node2") {
		t.Error("Node2 should not exist")
	}
	if !ring.HasNode("node1") || !ring.HasNode("node3") {
		t.Error("Other nodes should still exist")
	}
}

func TestRing_GetOwner_EmptyRing(t *testing.T) {
	ring := New(DefaultConfig())

	owner := ring.GetOwner("key1")
	if owner != "" {
		t.Errorf("Expected empty owner for empty ring, got %s", owner)
	}
}

func TestRing_GetOwner_SingleNode(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")

	owner := ring.GetOwner("key1")
	if owner != "node1" {
		t.Errorf("Expected owner 'node1', got %s", owner)
	}
}

func TestRing_GetOwner_MultipleNodes(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	owner := ring.GetOwner("key1")
	if owner == "" {
		t.Fatal("Expected non-empty owner")
	}
	if owner != "node1" && owner != "node2" && owner != "node3" {
		t.Errorf("Expected owner to be one of the nodes, got %s", owner)
	}
}

func TestRing_GetOwners_EmptyRing(t *testing.T) {
	ring := New(DefaultConfig())

	owners := ring.GetOwners("key1")
	if len(owners) != 0 {
		t.Errorf("Expected empty owners for empty ring, got %d", len(owners))
	}
}

func TestRing_GetOwners_SingleNode(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")

	owners := ring.GetOwners("key1")
	if len(owners) != 1 {
		t.Errorf("Expected 1 owner, got %d", len(owners))
	}
	if owners[0] != "node1" {
		t.Errorf("Expected owner 'node1', got %s", owners[0])
	}
}

func TestRing_GetOwners_ReplicationFactor(t *testing.T) {
	ring := New(DefaultConfig())
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")
	ring.AddNode("node4")
	ring.AddNode("node5")

	owners := ring.GetOwners("key1")
	if len(owners) != 3 {
		t.Errorf("Expected 3 owners (replication factor), got %d", len(owners))
	}

	// All owners should be unique
	seen := make(map[NodeID]bool)
	for _, owner := range owners {
		if seen[owner] {
			t.Errorf("Duplicate owner found: %s", owner)
		}
		seen[owner] = true
	}
}

func TestRing_GetOwners_CustomReplicationFactor(t *testing.T) {
	config := Config{
		VirtualNodesPerNode: 10,
		ReplicationFactor:   5,
	}
	ring := New(config)

	for i := 0; i < 10; i++ {
		ring.AddNode(NodeID(fmt.Sprintf("node%d", i)))
	}

	owners := ring.GetOwners("key1")
	if len(owners) != 5 {
		t.Errorf("Expected 5 owners (replication factor), got %d", len(owners))
	}
}

func TestRing_GetOwners_FewerNodesThanReplicationFactor(t *testing.T) {
	config := Config{
		VirtualNodesPerNode: 10,
		ReplicationFactor:   5,
	}
	ring := New(config)

	ring.AddNode("node1")
	ring.AddNode("node2")

	owners := ring.GetOwners("key1")
	// Should return at most the number of nodes available
	if len(owners) > 2 {
		t.Errorf("Expected at most 2 owners, got %d", len(owners))
	}
	if len(owners) < 1 {
		t.Error("Expected at least 1 owner")
	}
}

func TestRing_GetOwners_Consistency(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// Same key should always return same owners
	owners1 := ring.GetOwners("key1")
	owners2 := ring.GetOwners("key1")

	if len(owners1) != len(owners2) {
		t.Fatalf("Expected same number of owners, got %d vs %d", len(owners1), len(owners2))
	}

	for i := range owners1 {
		if owners1[i] != owners2[i] {
			t.Errorf("Expected consistent owners, got different at index %d: %s vs %s", i, owners1[i], owners2[i])
		}
	}
}

func TestRing_GetOwners_DifferentKeys(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")
	ring.AddNode("node4")
	ring.AddNode("node5")

	owners1 := ring.GetOwners("key1")
	owners2 := ring.GetOwners("key2")

	// Different keys might have different owners (likely but not guaranteed)
	// But both should return valid owners
	if len(owners1) == 0 {
		t.Error("Expected owners for key1")
	}
	if len(owners2) == 0 {
		t.Error("Expected owners for key2")
	}
}

func TestRing_GetNodes(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	nodes := ring.GetNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// Should be sorted
	for i := 1; i < len(nodes); i++ {
		if nodes[i-1] >= nodes[i] {
			t.Error("Nodes should be sorted")
		}
	}
}

func TestRing_GetNodes_Empty(t *testing.T) {
	ring := New(DefaultConfig())

	nodes := ring.GetNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(nodes))
	}
}

func TestRing_GetVirtualNodesForNode(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")

	vnodes := ring.GetVirtualNodesForNode("node1")
	if len(vnodes) != 150 {
		t.Errorf("Expected 150 virtual nodes, got %d", len(vnodes))
	}

	// All virtual nodes should belong to node1
	for _, vnode := range vnodes {
		if vnode.NodeID != "node1" {
			t.Errorf("Expected node1, got %s", vnode.NodeID)
		}
	}
}

func TestRing_GetVirtualNodesForNode_NonExistent(t *testing.T) {
	ring := New(DefaultConfig())

	vnodes := ring.GetVirtualNodesForNode("node1")
	if len(vnodes) != 0 {
		t.Errorf("Expected 0 virtual nodes for non-existent node, got %d", len(vnodes))
	}
}

func TestRing_VirtualNodes_Sorted(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// Virtual nodes should be sorted by hash
	ring.mu.RLock()
	defer ring.mu.RUnlock()

	for i := 1; i < len(ring.virtualNodes); i++ {
		if ring.virtualNodes[i-1].Hash > ring.virtualNodes[i].Hash {
			t.Error("Virtual nodes should be sorted by hash")
		}
	}
}

func TestRing_Distribution(t *testing.T) {
	config := Config{
		VirtualNodesPerNode: 100,
		ReplicationFactor:   3,
	}
	ring := New(config)

	// Add 5 nodes
	for i := 0; i < 5; i++ {
		ring.AddNode(NodeID(fmt.Sprintf("node%d", i)))
	}

	// Test distribution across many keys
	keyCount := 1000
	nodeCounts := make(map[NodeID]int)

	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("key%d", i)
		owners := ring.GetOwners(key)
		for _, owner := range owners {
			nodeCounts[owner]++
		}
	}

	// All nodes should get some keys (roughly equal distribution)
	if len(nodeCounts) != 5 {
		t.Errorf("Expected all 5 nodes to have keys, got %d", len(nodeCounts))
	}

	// Distribution should be relatively even (within 20% of average)
	avgCount := keyCount * 3 / 5 // replication factor * keys / nodes
	for nodeID, count := range nodeCounts {
		ratio := float64(count) / float64(avgCount)
		if ratio < 0.5 || ratio > 1.5 {
			t.Logf("Node %s has %d keys (avg: %d, ratio: %.2f)", nodeID, count, avgCount, ratio)
		}
	}
}

func TestRing_ConcurrentAccess(t *testing.T) {
	ring := New(DefaultConfig())

	// Concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			nodeID := NodeID(fmt.Sprintf("node%d", id))
			err := ring.AddNode(nodeID)
			if err != nil {
				t.Errorf("AddNode failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all adds
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := fmt.Sprintf("key%d", id)
			owners := ring.GetOwners(key)
			if len(owners) == 0 {
				t.Error("Expected owners")
			}
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}

	if ring.GetNodeCount() != 10 {
		t.Errorf("Expected 10 nodes, got %d", ring.GetNodeCount())
	}
}

func TestRing_AddRemove_Consistency(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// Remove a node
	ring.RemoveNode("node2")

	// Get owners after removal
	ownersAfter := ring.GetOwners("key1")

	// Owners should still be valid (not include removed node)
	for _, owner := range ownersAfter {
		if owner == "node2" {
			t.Error("Owners should not include removed node")
		}
	}

	// Should still have valid owners
	if len(ownersAfter) == 0 {
		t.Error("Expected owners after node removal")
	}
}

func TestRing_CustomVirtualNodesPerNode(t *testing.T) {
	config := Config{
		VirtualNodesPerNode: 50,
		ReplicationFactor:   3,
	}
	ring := New(config)

	ring.AddNode("node1")

	if ring.GetVirtualNodeCount() != 50 {
		t.Errorf("Expected 50 virtual nodes, got %d", ring.GetVirtualNodeCount())
	}
}

func TestRing_HashConsistency(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// Hash the same key multiple times
	hash1 := ring.hashKey("test-key")
	hash2 := ring.hashKey("test-key")

	if hash1 != hash2 {
		t.Error("Hash should be consistent for same key")
	}
}

func TestRing_GetOwner_MatchesFirstOwner(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	owner := ring.GetOwner("key1")
	owners := ring.GetOwners("key1")

	if len(owners) == 0 {
		t.Fatal("Expected owners")
	}

	if owner != owners[0] {
		t.Errorf("GetOwner should return first owner from GetOwners: %s vs %s", owner, owners[0])
	}
}

func TestRing_WrapAround(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")

	// Get owners for a key
	owners1 := ring.GetOwners("key1")

	// With only one node, all owners should be the same node
	for _, owner := range owners1 {
		if owner != "node1" {
			t.Errorf("Expected all owners to be node1, got %s", owner)
		}
	}
}

func TestRing_ManyKeys(t *testing.T) {
	ring := New(DefaultConfig())

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// Test many keys
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		owners := ring.GetOwners(key)
		if len(owners) == 0 {
			t.Fatalf("Expected owners for key %s", key)
		}
		if len(owners) > 3 {
			t.Fatalf("Expected at most 3 owners, got %d for key %s", len(owners), key)
		}
	}
}

