package repair

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// RepairTask represents a repair task to be executed.
type RepairTask struct {
	Key          string
	CorrectValue []byte
	TargetNodes  []interface{} // []cluster.NodeID, but using interface{} to avoid circular dependency
	Priority     Priority
	CreatedAt    int64 // Unix timestamp in nanoseconds
}

// Priority represents the priority of a repair task.
type Priority int

const (
	// PriorityLow represents low priority repairs.
	PriorityLow Priority = iota
	// PriorityNormal represents normal priority repairs.
	PriorityNormal
	// PriorityHigh represents high priority repairs.
	PriorityHigh
)

// Scheduler queues and rate-limits repair tasks.
type Scheduler struct {
	mu sync.RWMutex

	// Task queue (priority queue)
	taskQueue []*RepairTask

	// Rate limiting
	rateLimiter *RateLimiter

	// Executor for repair tasks
	executor *Executor

	// Worker management
	workers     int
	workerWg    sync.WaitGroup
	stopCh      chan struct{}
	running     bool
	runningMu   sync.RWMutex

	// Configuration
	maxConcurrent int
	checkInterval time.Duration
}

// SchedulerConfig holds configuration for the scheduler.
type SchedulerConfig struct {
	Executor      *Executor
	MaxConcurrent int           // Maximum concurrent repair tasks (default: 1)
	MaxRate       int           // Maximum repairs per second (default: 10)
	CheckInterval time.Duration // How often to check for new tasks (default: 100ms)
}

// DefaultSchedulerConfig returns a default scheduler configuration.
func DefaultSchedulerConfig(executor *Executor) SchedulerConfig {
	return SchedulerConfig{
		Executor:      executor,
		MaxConcurrent: 1,
		MaxRate:       10,
		CheckInterval: 100 * time.Millisecond,
	}
}

// RateLimiter limits the rate of repair operations.
type RateLimiter struct {
	maxRate     int
	opsCount    int
	windowStart time.Time
	mu          sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(maxRate int) *RateLimiter {
	return &RateLimiter{
		maxRate:     maxRate,
		windowStart: time.Now(),
	}
}

// Allow checks if an operation is allowed and records it.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowDuration := now.Sub(r.windowStart)

	// Reset counters every second
	if windowDuration >= time.Second {
		r.opsCount = 0
		r.windowStart = now
	}

	// Check rate limit
	if r.opsCount >= r.maxRate {
		return false
	}

	// Allow the operation
	r.opsCount++
	return true
}

// Wait waits until an operation is allowed.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if r.Allow() {
			return nil
		}

		// Wait a bit before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Continue loop
		}
	}
}

// NewScheduler creates a new repair scheduler.
func NewScheduler(config SchedulerConfig) *Scheduler {
	return &Scheduler{
		taskQueue:     make([]*RepairTask, 0),
		rateLimiter:   NewRateLimiter(config.MaxRate),
		executor:      config.Executor,
		maxConcurrent: config.MaxConcurrent,
		checkInterval: config.CheckInterval,
		stopCh:        make(chan struct{}),
		running:       false,
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start() error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler already running")
	}

	s.running = true
	s.stopCh = make(chan struct{})

	// Start worker goroutines
	s.workers = s.maxConcurrent
	for i := 0; i < s.workers; i++ {
		s.workerWg.Add(1)
		go s.worker(i)
	}

	return nil
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopCh)
	s.workerWg.Wait()

	return nil
}

// IsRunning returns whether the scheduler is currently running.
func (s *Scheduler) IsRunning() bool {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()
	return s.running
}

// ScheduleRepair schedules a repair task.
func (s *Scheduler) ScheduleRepair(ctx context.Context, task *RepairTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Add task to queue
	s.taskQueue = append(s.taskQueue, task)

	// Sort by priority (higher priority first)
	sort.Slice(s.taskQueue, func(i, j int) bool {
		return s.taskQueue[i].Priority > s.taskQueue[j].Priority
	})

	return nil
}

// dequeueTask removes and returns the highest priority task from the queue.
func (s *Scheduler) dequeueTask() *RepairTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.taskQueue) == 0 {
		return nil
	}

	task := s.taskQueue[0]
	s.taskQueue = s.taskQueue[1:]
	return task
}

// worker processes repair tasks.
func (s *Scheduler) worker(id int) {
	defer s.workerWg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		default:
			// Try to get a task
			task := s.dequeueTask()
			if task == nil {
				// No tasks available, wait a bit
				time.Sleep(s.checkInterval)
				continue
			}

			// Wait for rate limit
			ctx := context.Background()
			if err := s.rateLimiter.Wait(ctx); err != nil {
				// Context cancelled, re-enqueue task
				s.ScheduleRepair(ctx, task)
				return
			}

			// Execute repair
			if s.executor != nil {
				s.executor.ExecuteRepair(ctx, task)
			}
		}
	}
}

// GetQueueSize returns the current number of tasks in the queue.
func (s *Scheduler) GetQueueSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.taskQueue)
}

// GetStats returns scheduler statistics.
func (s *Scheduler) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Stats{
		QueueSize:    len(s.taskQueue),
		IsRunning:    s.IsRunning(),
		MaxConcurrent: s.maxConcurrent,
	}
}

// Stats holds scheduler statistics.
type Stats struct {
	QueueSize     int
	IsRunning     bool
	MaxConcurrent int
}

