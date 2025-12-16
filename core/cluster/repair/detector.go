package repair

import (
	"context"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/cluster"
)

// Divergence represents detected divergence between replicas.
type Divergence struct {
	Key           string
	CorrectValue  []byte
	DivergentNodes []cluster.NodeID
	DetectedAt    int64 // Unix timestamp in nanoseconds
}

// Detector detects replica divergence during reads and schedules repairs.
type Detector struct {
	mu sync.RWMutex

	// Scheduler for repair tasks
	scheduler *Scheduler

	// Callback for when divergence is detected (optional)
	onDivergence func(*Divergence)

	// Configuration
	enabled bool

	// Statistics
	divergencesDetected int64
}

// DetectorConfig holds configuration for the detector.
type DetectorConfig struct {
	Scheduler    *Scheduler
	OnDivergence func(*Divergence) // Optional callback
	Enabled      bool              // Whether read repair is enabled (default: true)
}

// NewDetector creates a new divergence detector.
func NewDetector(config DetectorConfig) *Detector {
	enabled := config.Enabled
	if !config.Enabled && config.Scheduler != nil {
		// Default to enabled if scheduler is provided
		enabled = true
	}
	return &Detector{
		scheduler:   config.Scheduler,
		onDivergence: config.OnDivergence,
		enabled:     enabled,
	}
}

// CheckReadResponses checks read responses for divergence and schedules repairs if needed.
// This should be called after a quorum read to detect any inconsistencies.
func (d *Detector) CheckReadResponses(ctx context.Context, key string, responses []cluster.Response) error {
	// Skip if read repair is disabled
	if !d.enabled {
		return nil
	}

	if len(responses) < 2 {
		// Need at least 2 responses to detect divergence
		return nil
	}

	// Group responses by value
	valueGroups := make(map[string][]cluster.Response)
	for _, resp := range responses {
		if !resp.Success || resp.Value == nil {
			continue
		}
		valueKey := string(resp.Value)
		valueGroups[valueKey] = append(valueGroups[valueKey], resp)
	}

	// If all responses have the same value, no divergence
	if len(valueGroups) <= 1 {
		return nil
	}

	// Find the majority value (correct value)
	correctValue := d.findMajorityValue(valueGroups, len(responses))
	if correctValue == nil {
		// No clear majority, can't determine correct value
		// In a real system, you might use vector clocks or timestamps
		return nil
	}

	// Find nodes with divergent values
	divergentNodes := make([]cluster.NodeID, 0)
	for valueKey, group := range valueGroups {
		if valueKey != string(correctValue) {
			for _, resp := range group {
				divergentNodes = append(divergentNodes, resp.NodeID)
			}
		}
	}

	if len(divergentNodes) == 0 {
		return nil
	}

	// Create divergence record
	divergence := &Divergence{
		Key:            key,
		CorrectValue:   correctValue,
		DivergentNodes: divergentNodes,
		DetectedAt:     time.Now().UnixNano(),
	}

	// Update statistics
	d.mu.Lock()
	d.divergencesDetected++
	d.mu.Unlock()

	// Call callback if set
	if d.onDivergence != nil {
		d.onDivergence(divergence)
	}

	// Schedule repair asynchronously
	if d.scheduler != nil {
		// Convert NodeID slice to interface{} slice
		targetNodes := make([]interface{}, len(divergentNodes))
		for i, nodeID := range divergentNodes {
			targetNodes[i] = nodeID
		}

		repairTask := &RepairTask{
			Key:           key,
			CorrectValue:  correctValue,
			TargetNodes:   targetNodes,
			Priority:      PriorityNormal,
			CreatedAt:     time.Now().UnixNano(),
		}
		return d.scheduler.ScheduleRepair(ctx, repairTask)
	}

	return nil
}

// findMajorityValue finds the value that appears in the majority of responses.
func (d *Detector) findMajorityValue(valueGroups map[string][]cluster.Response, totalResponses int) []byte {
	maxCount := 0
	var majorityValue []byte

	for _, group := range valueGroups {
		count := len(group)
		if count > maxCount {
			maxCount = count
			// Get the actual value from the first response in the group
			if len(group) > 0 {
				majorityValue = group[0].Value
			}
		}
	}

	// Check if we have a majority (more than half)
	if maxCount > totalResponses/2 {
		return majorityValue
	}

	return nil
}

// GetDivergencesDetected returns the number of divergences detected.
func (d *Detector) GetDivergencesDetected() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.divergencesDetected
}


