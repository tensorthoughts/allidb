package ring

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"sync"
)

// NodeID represents a physical node identifier.
type NodeID string

// VirtualNode represents a virtual node on the ring.
type VirtualNode struct {
	Hash   uint64 // Hash value on the ring
	NodeID NodeID // Physical node this virtual node belongs to
	Index  int    // Index of this virtual node (for ordering)
}

// Ring implements a consistent hashing ring with virtual nodes.
type Ring struct {
	mu sync.RWMutex

	// Virtual nodes sorted by hash
	virtualNodes []VirtualNode

	// Physical nodes and their virtual node counts
	nodes map[NodeID]int

	// Configuration
	virtualNodesPerNode int // Number of virtual nodes per physical node
	replicationFactor   int // Number of nodes to replicate to
}

// Config holds configuration for the ring.
type Config struct {
	VirtualNodesPerNode int // Number of virtual nodes per physical node (default: 150)
	ReplicationFactor   int // Replication factor (default: 3)
}

// DefaultConfig returns a default ring configuration.
func DefaultConfig() Config {
	return Config{
		VirtualNodesPerNode: 150,
		ReplicationFactor:   3,
	}
}

// New creates a new consistent hashing ring.
func New(config Config) *Ring {
	if config.VirtualNodesPerNode <= 0 {
		config.VirtualNodesPerNode = 150
	}
	if config.ReplicationFactor <= 0 {
		config.ReplicationFactor = 3
	}

	return &Ring{
		virtualNodes:        make([]VirtualNode, 0),
		nodes:               make(map[NodeID]int),
		virtualNodesPerNode: config.VirtualNodesPerNode,
		replicationFactor:   config.ReplicationFactor,
	}
}

// AddNode adds a physical node to the ring with virtual nodes.
func (r *Ring) AddNode(nodeID NodeID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nodes[nodeID]; exists {
		return fmt.Errorf("node %s already exists", nodeID)
	}

	// Create virtual nodes for this physical node
	for i := 0; i < r.virtualNodesPerNode; i++ {
		hash := r.hashVirtualNode(nodeID, i)
		vnode := VirtualNode{
			Hash:   hash,
			NodeID: nodeID,
			Index:  i,
		}
		r.virtualNodes = append(r.virtualNodes, vnode)
	}

	// Sort virtual nodes by hash
	sort.Slice(r.virtualNodes, func(i, j int) bool {
		return r.virtualNodes[i].Hash < r.virtualNodes[j].Hash
	})

	r.nodes[nodeID] = r.virtualNodesPerNode
	return nil
}

// RemoveNode removes a physical node and all its virtual nodes from the ring.
func (r *Ring) RemoveNode(nodeID NodeID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s does not exist", nodeID)
	}

	// Remove all virtual nodes for this physical node
	newVirtualNodes := make([]VirtualNode, 0, len(r.virtualNodes))
	for _, vnode := range r.virtualNodes {
		if vnode.NodeID != nodeID {
			newVirtualNodes = append(newVirtualNodes, vnode)
		}
	}
	r.virtualNodes = newVirtualNodes

	delete(r.nodes, nodeID)
	return nil
}

// GetOwners returns the owner nodes for a given key.
// Returns up to replicationFactor nodes.
func (r *Ring) GetOwners(key string) []NodeID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.virtualNodes) == 0 {
		return []NodeID{}
	}

	keyHash := r.hashKey(key)
	owners := make([]NodeID, 0, r.replicationFactor)
	seen := make(map[NodeID]bool)

	// Find the first virtual node with hash >= keyHash
	startIdx := r.findVirtualNode(keyHash)

	// Collect owners starting from the found index
	for i := 0; i < len(r.virtualNodes) && len(owners) < r.replicationFactor; i++ {
		idx := (startIdx + i) % len(r.virtualNodes)
		vnode := r.virtualNodes[idx]

		// Skip if we've already seen this physical node
		if !seen[vnode.NodeID] {
			owners = append(owners, vnode.NodeID)
			seen[vnode.NodeID] = true
		}
	}

	return owners
}

// GetOwner returns the primary owner node for a given key.
func (r *Ring) GetOwner(key string) NodeID {
	owners := r.GetOwners(key)
	if len(owners) == 0 {
		return ""
	}
	return owners[0]
}

// findVirtualNode finds the index of the first virtual node with hash >= keyHash.
// Uses binary search for O(log n) lookup.
func (r *Ring) findVirtualNode(keyHash uint64) int {
	// Binary search for the insertion point
	idx := sort.Search(len(r.virtualNodes), func(i int) bool {
		return r.virtualNodes[i].Hash >= keyHash
	})

	// If we're at the end, wrap around to the beginning
	if idx >= len(r.virtualNodes) {
		idx = 0
	}

	return idx
}

// hashKey hashes a key to a position on the ring.
func (r *Ring) hashKey(key string) uint64 {
	return r.hashString(key)
}

// hashVirtualNode hashes a virtual node identifier.
func (r *Ring) hashVirtualNode(nodeID NodeID, index int) uint64 {
	key := fmt.Sprintf("%s:%d", nodeID, index)
	return r.hashString(key)
}

// hashString hashes a string to a uint64 using SHA1.
func (r *Ring) hashString(s string) uint64 {
	h := sha1.New()
	h.Write([]byte(s))
	hash := h.Sum(nil)

	// Take first 8 bytes and convert to uint64
	var result uint64
	for i := 0; i < 8 && i < len(hash); i++ {
		result = result<<8 | uint64(hash[i])
	}
	return result
}

// GetNodes returns all physical nodes in the ring.
func (r *Ring) GetNodes() []NodeID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]NodeID, 0, len(r.nodes))
	for nodeID := range r.nodes {
		nodes = append(nodes, nodeID)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})

	return nodes
}

// GetNodeCount returns the number of physical nodes in the ring.
func (r *Ring) GetNodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nodes)
}

// GetVirtualNodeCount returns the total number of virtual nodes in the ring.
func (r *Ring) GetVirtualNodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.virtualNodes)
}

// HasNode checks if a node exists in the ring.
func (r *Ring) HasNode(nodeID NodeID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.nodes[nodeID]
	return exists
}

// GetReplicationFactor returns the replication factor.
func (r *Ring) GetReplicationFactor() int {
	return r.replicationFactor
}

// GetVirtualNodesPerNode returns the number of virtual nodes per physical node.
func (r *Ring) GetVirtualNodesPerNode() int {
	return r.virtualNodesPerNode
}

// GetVirtualNodesForNode returns all virtual nodes for a given physical node.
func (r *Ring) GetVirtualNodesForNode(nodeID NodeID) []VirtualNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	vnodes := make([]VirtualNode, 0)
	for _, vnode := range r.virtualNodes {
		if vnode.NodeID == nodeID {
			vnodes = append(vnodes, vnode)
		}
	}

	return vnodes
}

// Rebalance is a placeholder for future rebalancing logic.
// In a real implementation, this would redistribute keys when nodes are added/removed.
func (r *Ring) Rebalance() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Rebuild virtual nodes deterministically from current node set.
	r.virtualNodes = r.virtualNodes[:0]
	for nodeID := range r.nodes {
		for i := 0; i < r.virtualNodesPerNode; i++ {
			hash := r.hashVirtualNode(nodeID, i)
			r.virtualNodes = append(r.virtualNodes, VirtualNode{
				Hash:   hash,
				NodeID: nodeID,
				Index:  i,
			})
		}
	}

	// Sort virtual nodes by hash to maintain ring order.
	sort.Slice(r.virtualNodes, func(i, j int) bool {
		return r.virtualNodes[i].Hash < r.virtualNodes[j].Hash
	})
}

