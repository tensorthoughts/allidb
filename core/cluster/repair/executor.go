package repair

import (
	"context"
	"fmt"
	"sync"

	"github.com/tensorthoughts25/allidb/core/cluster"
)

// Executor executes repair writes using the normal write path.
type Executor struct {
	mu sync.RWMutex

	// Coordinator for executing writes
	coordinator *cluster.Coordinator
	client      cluster.ReplicaClient

	// Statistics
	repairsExecuted int64
	repairsFailed   int64
}

// ExecutorConfig holds configuration for the executor.
type ExecutorConfig struct {
	Coordinator *cluster.Coordinator
	Client      cluster.ReplicaClient
}

// NewExecutor creates a new repair executor.
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		coordinator: config.Coordinator,
		client:      config.Client,
	}
}

// ExecuteRepair executes a repair task by writing the correct value to divergent nodes.
func (e *Executor) ExecuteRepair(ctx context.Context, task *RepairTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if e.coordinator == nil {
		return fmt.Errorf("coordinator not set")
	}

	// Convert target nodes to cluster.NodeID
	targetNodes := make([]cluster.NodeID, 0, len(task.TargetNodes))
	for _, node := range task.TargetNodes {
		if nodeID, ok := node.(cluster.NodeID); ok {
			targetNodes = append(targetNodes, nodeID)
		}
	}

	if len(targetNodes) == 0 {
		return fmt.Errorf("no valid target nodes")
	}

	// Prefer direct replica writes to targeted nodes
	successCount := 0
	for _, nodeID := range targetNodes {
		var err error
		if e.client != nil {
			err = e.client.Write(ctx, nodeID, task.Key, task.CorrectValue)
		} else {
			// Fallback to coordinator write (broadcast/quorum)
			err = e.coordinator.Write(ctx, task.Key, task.CorrectValue)
		}
		if err != nil {
			e.mu.Lock()
			e.repairsFailed++
			e.mu.Unlock()
			continue
		}
		successCount++
	}

	// Update statistics
	e.mu.Lock()
	e.repairsExecuted += int64(successCount)
	e.mu.Unlock()

	if successCount == 0 {
		return fmt.Errorf("all repair writes failed")
	}

	return nil
}

// ExecuteRepairToNode executes a repair write to a specific node.
// This is a more targeted repair that only writes to the specified node.
func (e *Executor) ExecuteRepairToNode(ctx context.Context, key string, value []byte, nodeID cluster.NodeID) error {
	if e.coordinator == nil {
		return fmt.Errorf("coordinator not set")
	}

	var err error
	if e.client != nil {
		err = e.client.Write(ctx, nodeID, key, value)
	} else {
		err = e.coordinator.Write(ctx, key, value)
	}
	if err != nil {
		e.mu.Lock()
		e.repairsFailed++
		e.mu.Unlock()
		return fmt.Errorf("failed to repair node %s: %w", nodeID, err)
	}

	e.mu.Lock()
	e.repairsExecuted++
	e.mu.Unlock()

	return nil
}

// GetRepairsExecuted returns the number of repairs executed.
func (e *Executor) GetRepairsExecuted() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.repairsExecuted
}

// GetRepairsFailed returns the number of repairs that failed.
func (e *Executor) GetRepairsFailed() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.repairsFailed
}

// GetStats returns executor statistics.
func (e *Executor) GetStats() ExecutorStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return ExecutorStats{
		RepairsExecuted: e.repairsExecuted,
		RepairsFailed:   e.repairsFailed,
	}
}

// ExecutorStats holds executor statistics.
type ExecutorStats struct {
	RepairsExecuted int64
	RepairsFailed   int64
}

