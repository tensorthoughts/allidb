package entropy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PartitionProvider provides partition information.
type PartitionProvider interface {
	// GetPartitions returns all partition IDs.
	GetPartitions(ctx context.Context) ([]PartitionID, error)
}

// Orchestrator schedules periodic and manual full repairs using Merkle tree comparison.
type Orchestrator struct {
	mu sync.RWMutex

	comparator       *Comparator
	executor         *RangeExecutor
	partitionProvider PartitionProvider

	// Scheduling
	periodicInterval time.Duration
	ticker           *time.Ticker
	stopCh           chan struct{}
	running          bool
	runningMu        sync.RWMutex

	// Manual repair requests
	manualRepairCh chan PartitionID

	// Statistics
	repairsCompleted int64
	repairsFailed    int64
	lastRepairTime   time.Time
}

// Config holds configuration for the orchestrator.
type OrchestratorConfig struct {
	Comparator        *Comparator
	Executor          *RangeExecutor
	PartitionProvider PartitionProvider
	PeriodicInterval  time.Duration // How often to run periodic repairs (default: 1 hour)
}

// DefaultOrchestratorConfig returns a default orchestrator configuration.
func DefaultOrchestratorConfig(comparator *Comparator, executor *RangeExecutor, partitionProvider PartitionProvider) OrchestratorConfig {
	return OrchestratorConfig{
		Comparator:        comparator,
		Executor:          executor,
		PartitionProvider: partitionProvider,
		PeriodicInterval:  1 * time.Hour,
	}
}

// NewOrchestrator creates a new repair orchestrator.
func NewOrchestrator(config OrchestratorConfig) *Orchestrator {
	return &Orchestrator{
		comparator:        config.Comparator,
		executor:          config.Executor,
		partitionProvider: config.PartitionProvider,
		periodicInterval:  config.PeriodicInterval,
		manualRepairCh:    make(chan PartitionID, 100),
		stopCh:            make(chan struct{}),
		running:           false,
	}
}

// Start starts the orchestrator for periodic repairs.
func (o *Orchestrator) Start() error {
	o.runningMu.Lock()
	defer o.runningMu.Unlock()

	if o.running {
		return fmt.Errorf("orchestrator already running")
	}

	o.running = true
	o.stopCh = make(chan struct{})
	o.ticker = time.NewTicker(o.periodicInterval)

	// Start periodic repair goroutine
	go o.periodicRepairLoop()

	// Start manual repair handler
	go o.manualRepairLoop()

	return nil
}

// Stop stops the orchestrator.
func (o *Orchestrator) Stop() error {
	o.runningMu.Lock()
	defer o.runningMu.Unlock()

	if !o.running {
		return nil
	}

	o.running = false
	if o.ticker != nil {
		o.ticker.Stop()
	}
	close(o.stopCh)
	close(o.manualRepairCh)

	return nil
}

// IsRunning returns whether the orchestrator is currently running.
func (o *Orchestrator) IsRunning() bool {
	o.runningMu.RLock()
	defer o.runningMu.RUnlock()
	return o.running
}

// TriggerRepair manually triggers a repair for a specific partition.
func (o *Orchestrator) TriggerRepair(partitionID PartitionID) error {
	if !o.IsRunning() {
		return fmt.Errorf("orchestrator is not running")
	}

	select {
	case o.manualRepairCh <- partitionID:
		return nil
	default:
		return fmt.Errorf("manual repair queue is full")
	}
}

// TriggerFullRepair triggers a full repair for all partitions.
func (o *Orchestrator) TriggerFullRepair(ctx context.Context) error {
	if o.partitionProvider == nil {
		return fmt.Errorf("partition provider not set")
	}

	partitions, err := o.partitionProvider.GetPartitions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get partitions: %w", err)
	}

	// Trigger repair for each partition
	for _, partitionID := range partitions {
		if err := o.TriggerRepair(partitionID); err != nil {
			// Log error but continue with other partitions
			continue
		}
	}

	return nil
}

// periodicRepairLoop runs periodic repairs.
func (o *Orchestrator) periodicRepairLoop() {
	for {
		select {
		case <-o.stopCh:
			return
		case <-o.ticker.C:
			o.runPeriodicRepair()
		}
	}
}

// manualRepairLoop handles manual repair requests.
func (o *Orchestrator) manualRepairLoop() {
	for {
		select {
		case <-o.stopCh:
			return
		case partitionID, ok := <-o.manualRepairCh:
			if !ok {
				return
			}
			o.repairPartition(context.Background(), partitionID)
		}
	}
}

// runPeriodicRepair runs a periodic repair for all partitions.
func (o *Orchestrator) runPeriodicRepair() {
	ctx := context.Background()

	if o.partitionProvider == nil {
		return
	}

	partitions, err := o.partitionProvider.GetPartitions(ctx)
	if err != nil {
		// Log error
		return
	}

	// Repair each partition
	for _, partitionID := range partitions {
		o.repairPartition(ctx, partitionID)
	}
}

// repairPartition repairs a single partition.
func (o *Orchestrator) repairPartition(ctx context.Context, partitionID PartitionID) {
	if o.comparator == nil || o.executor == nil {
		return
	}

	// Compare trees to find divergent ranges
	divergentRanges, err := o.comparator.ComparePartition(ctx, partitionID)
	if err != nil {
		o.mu.Lock()
		o.repairsFailed++
		o.mu.Unlock()
		return
	}

	if len(divergentRanges) == 0 {
		// No divergence, nothing to repair
		return
	}

	// Repair divergent ranges
	err = o.executor.RepairRanges(ctx, divergentRanges)
	if err != nil {
		o.mu.Lock()
		o.repairsFailed++
		o.mu.Unlock()
		return
	}

	// Update statistics
	o.mu.Lock()
	o.repairsCompleted++
	o.lastRepairTime = time.Now()
	o.mu.Unlock()
}

// GetRepairsCompleted returns the number of repairs completed.
func (o *Orchestrator) GetRepairsCompleted() int64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.repairsCompleted
}

// GetRepairsFailed returns the number of repairs that failed.
func (o *Orchestrator) GetRepairsFailed() int64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.repairsFailed
}

// GetLastRepairTime returns the time of the last repair.
func (o *Orchestrator) GetLastRepairTime() time.Time {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastRepairTime
}

// GetStats returns orchestrator statistics.
func (o *Orchestrator) GetStats() OrchestratorStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return OrchestratorStats{
		RepairsCompleted: o.repairsCompleted,
		RepairsFailed:    o.repairsFailed,
		LastRepairTime:   o.lastRepairTime,
		IsRunning:        o.IsRunning(),
	}
}

// OrchestratorStats holds orchestrator statistics.
type OrchestratorStats struct {
	RepairsCompleted int64
	RepairsFailed    int64
	LastRepairTime   time.Time
	IsRunning        bool
}

