package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster/gossip"
	"github.com/tensorthoughts25/allidb/core/cluster/ring"
)

// NodeID is an alias for ring.NodeID
type NodeID = ring.NodeID

// Request represents a request to be coordinated.
type Request struct {
	Key     string
	Value   []byte
	Op      Operation
	Context context.Context
}

// Operation represents the type of operation.
type Operation int

const (
	// OperationRead represents a read operation.
	OperationRead Operation = iota
	// OperationWrite represents a write operation.
	OperationWrite
)

// Response represents a response from a replica.
type Response struct {
	NodeID  NodeID
	Value   []byte
	Error   error
	Success bool
}

// GossipInterface is the interface for node liveness information.
type GossipInterface interface {
	GetAliveNodes() []gossip.NodeID
}

// Coordinator coordinates requests across replicas.
type Coordinator struct {
	mu sync.RWMutex

	ring   *ring.Ring
	gossip GossipInterface

	// Configuration
	config Config

	// Replica client (for sending requests to replicas)
	client ReplicaClient
}

// Config holds configuration for the coordinator.
type Config struct {
	ReadQuorum    int           // Number of replicas needed for read quorum (default: 2)
	WriteQuorum   int           // Number of replicas needed for write quorum (default: 2)
	ReadTimeout   time.Duration // Timeout for read requests (default: 5s)
	WriteTimeout  time.Duration // Timeout for write requests (default: 5s)
	ReplicaCount  int           // Number of replicas to use (default: 3)
}

// DefaultConfig returns a default coordinator configuration.
func DefaultConfig() Config {
	return Config{
		ReadQuorum:   2,
		WriteQuorum:  2,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		ReplicaCount: 3,
	}
}

// ReplicaClient is the interface for communicating with replicas.
type ReplicaClient interface {
	// Read reads a value from a replica.
	Read(ctx context.Context, nodeID NodeID, key string) ([]byte, error)
	// Write writes a value to a replica.
	Write(ctx context.Context, nodeID NodeID, key string, value []byte) error
}

// NewCoordinator creates a new coordinator.
func NewCoordinator(ring *ring.Ring, gossip GossipInterface, client ReplicaClient, config Config) *Coordinator {
	return &Coordinator{
		ring:   ring,
		gossip: gossip,
		config: config,
		client: client,
	}
}

// Read performs a quorum read.
func (c *Coordinator) Read(ctx context.Context, key string) ([]byte, error) {
	responses, err := c.ReadWithResponses(ctx, key)
	if err != nil {
		return nil, err
	}
	return c.mergeReadResponses(responses), nil
}

// ReadWithResponses performs a quorum read and returns all responses.
// This method exposes responses for repair detection and other use cases.
func (c *Coordinator) ReadWithResponses(ctx context.Context, key string) ([]Response, error) {
	// Select replicas
	replicas := c.selectReplicas(key)
	if len(replicas) == 0 {
		return nil, fmt.Errorf("no replicas available for key %s", key)
	}

	// Filter to only alive nodes
	aliveReplicas := c.filterAliveNodes(replicas)
	if len(aliveReplicas) < c.config.ReadQuorum {
		return nil, fmt.Errorf("insufficient alive replicas: need %d, have %d", c.config.ReadQuorum, len(aliveReplicas))
	}

	// Create context with timeout
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()

	// Send read requests to all alive replicas
	responses := c.readFromReplicas(readCtx, key, aliveReplicas)

	// Wait for quorum
	successfulResponses := c.waitForQuorum(responses, c.config.ReadQuorum, readCtx)

	if len(successfulResponses) < c.config.ReadQuorum {
		return nil, fmt.Errorf("quorum not reached: need %d, got %d", c.config.ReadQuorum, len(successfulResponses))
	}

	return responses, nil
}

// Write performs a quorum write.
func (c *Coordinator) Write(ctx context.Context, key string, value []byte) error {
	// Select replicas
	replicas := c.selectReplicas(key)
	if len(replicas) == 0 {
		return fmt.Errorf("no replicas available for key %s", key)
	}

	// Filter to only alive nodes
	aliveReplicas := c.filterAliveNodes(replicas)
	if len(aliveReplicas) < c.config.WriteQuorum {
		return fmt.Errorf("insufficient alive replicas: need %d, have %d", c.config.WriteQuorum, len(aliveReplicas))
	}

	// Create context with timeout
	writeCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
	defer cancel()

	// Send write requests to all alive replicas
	responses := c.writeToReplicas(writeCtx, key, value, aliveReplicas)

	// Wait for quorum
	successfulResponses := c.waitForQuorum(responses, c.config.WriteQuorum, writeCtx)

	if len(successfulResponses) < c.config.WriteQuorum {
		return fmt.Errorf("quorum not reached: need %d, got %d", c.config.WriteQuorum, len(successfulResponses))
	}

	return nil
}

// selectReplicas selects replicas for a key using consistent hashing.
func (c *Coordinator) selectReplicas(key string) []NodeID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Use ring to get owners (replicas)
	// GetOwners returns up to replication factor, we'll take what we need
	owners := c.ring.GetOwners(key)
	
	// Limit to ReplicaCount if needed
	if len(owners) > c.config.ReplicaCount {
		owners = owners[:c.config.ReplicaCount]
	}
	
	return owners
}

// filterAliveNodes filters out dead nodes.
func (c *Coordinator) filterAliveNodes(nodes []NodeID) []NodeID {
	if c.gossip == nil {
		// If no gossip, assume all nodes are alive
		return nodes
	}

	aliveNodes := c.gossip.GetAliveNodes()
	aliveSet := make(map[NodeID]bool)
	for _, nodeID := range aliveNodes {
		aliveSet[NodeID(nodeID)] = true
	}

	filtered := make([]NodeID, 0, len(nodes))
	for _, nodeID := range nodes {
		if aliveSet[nodeID] {
			filtered = append(filtered, nodeID)
		}
	}

	return filtered
}

// readFromReplicas sends read requests to replicas in parallel.
func (c *Coordinator) readFromReplicas(ctx context.Context, key string, replicas []NodeID) []Response {
	responses := make([]Response, len(replicas))
	var wg sync.WaitGroup

	for i, replica := range replicas {
		wg.Add(1)
		go func(idx int, nodeID NodeID) {
			defer wg.Done()

			value, err := c.client.Read(ctx, nodeID, key)
			responses[idx] = Response{
				NodeID:  nodeID,
				Value:   value,
				Error:   err,
				Success: err == nil,
			}
		}(i, replica)
	}

	wg.Wait()
	return responses
}

// writeToReplicas sends write requests to replicas in parallel.
func (c *Coordinator) writeToReplicas(ctx context.Context, key string, value []byte, replicas []NodeID) []Response {
	responses := make([]Response, len(replicas))
	var wg sync.WaitGroup

	for i, replica := range replicas {
		wg.Add(1)
		go func(idx int, nodeID NodeID) {
			defer wg.Done()

			err := c.client.Write(ctx, nodeID, key, value)
			responses[idx] = Response{
				NodeID:  nodeID,
				Error:   err,
				Success: err == nil,
			}
		}(i, replica)
	}

	wg.Wait()
	return responses
}

// waitForQuorum waits for quorum of successful responses.
func (c *Coordinator) waitForQuorum(responses []Response, quorum int, ctx context.Context) []Response {
	successful := make([]Response, 0, quorum)

	for _, resp := range responses {
		if resp.Success {
			successful = append(successful, resp)
			if len(successful) >= quorum {
				return successful
			}
		}
	}

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		return successful
	default:
		return successful
	}
}

// mergeReadResponses merges read responses.
// Returns the value that appears in the majority of responses (last-write-wins).
// In a real implementation, this would use vector clocks or timestamps to determine the latest value.
func (c *Coordinator) mergeReadResponses(responses []Response) []byte {
	if len(responses) == 0 {
		return nil
	}

	// Count occurrences of each value
	valueCounts := make(map[string]int)
	valueMap := make(map[string][]byte)

	for _, resp := range responses {
		if resp.Success && resp.Value != nil {
			key := string(resp.Value)
			valueCounts[key]++
			valueMap[key] = resp.Value
		}
	}

	// Find the value with the highest count
	maxCount := 0
	var result []byte
	for key, count := range valueCounts {
		if count > maxCount {
			maxCount = count
			result = valueMap[key]
		}
	}

	// If no majority, return the first successful response
	if result == nil && len(responses) > 0 {
		for _, resp := range responses {
			if resp.Success && resp.Value != nil {
				return resp.Value
			}
		}
	}

	return result
}

// GetReplicas returns the replicas for a key.
func (c *Coordinator) GetReplicas(key string) []NodeID {
	return c.selectReplicas(key)
}

// GetAliveReplicas returns the alive replicas for a key.
func (c *Coordinator) GetAliveReplicas(key string) []NodeID {
	replicas := c.selectReplicas(key)
	return c.filterAliveNodes(replicas)
}

// UpdateConfig updates the coordinator configuration.
func (c *Coordinator) UpdateConfig(config Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GetConfig returns the current configuration.
func (c *Coordinator) GetConfig() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

