package entropy

import (
	"context"
	"fmt"
	"sync"

	"github.com/tensorthoughts25/allidb/core/cluster"
)

// DataProvider provides data for key ranges.
type DataProvider interface {
	// GetRangeData returns all key-value pairs in a key range from a specific node.
	GetRangeData(ctx context.Context, nodeID cluster.NodeID, partitionID PartitionID, keyRange KeyRange) (map[string][]byte, error)
}

// RangeExecutor executes repairs for divergent key ranges using the normal write path.
type RangeExecutor struct {
	mu sync.RWMutex

	coordinator  *cluster.Coordinator
	dataProvider DataProvider

	// Statistics
	rangesRepaired int64
	rangesFailed   int64
	keysRepaired   int64
}

// Config holds configuration for the range executor.
type RangeExecutorConfig struct {
	Coordinator  *cluster.Coordinator
	DataProvider DataProvider
}

// NewRangeExecutor creates a new range executor.
func NewRangeExecutor(config RangeExecutorConfig) *RangeExecutor {
	return &RangeExecutor{
		coordinator:  config.Coordinator,
		dataProvider: config.DataProvider,
	}
}

// RepairRange repairs a divergent key range by copying correct data to divergent nodes.
func (re *RangeExecutor) RepairRange(ctx context.Context, divergentRange DivergentRange) error {
	if re.coordinator == nil {
		return fmt.Errorf("coordinator not set")
	}

	if re.dataProvider == nil {
		return fmt.Errorf("data provider not set")
	}

	if len(divergentRange.DivergentNodes) == 0 {
		return fmt.Errorf("no divergent nodes to repair")
	}

	// Get correct data from the correct node
	correctData, err := re.dataProvider.GetRangeData(ctx, divergentRange.CorrectNode, divergentRange.PartitionID, divergentRange.KeyRange)
	if err != nil {
		re.mu.Lock()
		re.rangesFailed++
		re.mu.Unlock()
		return fmt.Errorf("failed to get correct data: %w", err)
	}

	if len(correctData) == 0 {
		// No data in this range, nothing to repair
		return nil
	}

	// Write correct data to all divergent nodes using coordinator's write path
	successCount := 0
	for key, value := range correctData {
		// Use coordinator's write to replicate to all nodes
		err := re.coordinator.Write(ctx, key, value)
		if err != nil {
			// Log error but continue with other keys
			continue
		}
		successCount++
	}

	// Update statistics
	re.mu.Lock()
	if successCount > 0 {
		re.rangesRepaired++
		re.keysRepaired += int64(successCount)
	} else {
		re.rangesFailed++
	}
	re.mu.Unlock()

	if successCount == 0 {
		return fmt.Errorf("failed to repair any keys in range")
	}

	return nil
}

// RepairRanges repairs multiple divergent ranges.
func (re *RangeExecutor) RepairRanges(ctx context.Context, divergentRanges []DivergentRange) error {
	var lastErr error
	successCount := 0

	for _, divergentRange := range divergentRanges {
		err := re.RepairRange(ctx, divergentRange)
		if err != nil {
			lastErr = err
			continue
		}
		successCount++
	}

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("all repairs failed: %w", lastErr)
	}

	return nil
}

// GetRangesRepaired returns the number of ranges repaired.
func (re *RangeExecutor) GetRangesRepaired() int64 {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.rangesRepaired
}

// GetRangesFailed returns the number of ranges that failed to repair.
func (re *RangeExecutor) GetRangesFailed() int64 {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.rangesFailed
}

// GetKeysRepaired returns the number of keys repaired.
func (re *RangeExecutor) GetKeysRepaired() int64 {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.keysRepaired
}

// GetStats returns executor statistics.
func (re *RangeExecutor) GetStats() RangeExecutorStats {
	re.mu.RLock()
	defer re.mu.RUnlock()

	return RangeExecutorStats{
		RangesRepaired: re.rangesRepaired,
		RangesFailed:   re.rangesFailed,
		KeysRepaired:   re.keysRepaired,
	}
}

// RangeExecutorStats holds executor statistics.
type RangeExecutorStats struct {
	RangesRepaired int64
	RangesFailed   int64
	KeysRepaired   int64
}

