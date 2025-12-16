package compaction

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// CompactionJob represents a compaction job.
type CompactionJob struct {
	InputPaths []string
	OutputPath string
	Priority   int // Higher priority jobs run first
	CreatedAt  time.Time
}

// Manager manages SSTable compaction using size-tiered strategy.
type Manager struct {
	planner  *Planner
	executor *Executor

	// Job queue
	jobQueue   []CompactionJob
	jobQueueMu sync.Mutex

	// IO throttling
	ioLimiter   *IOLimiter
	throttleMu  sync.Mutex
	bytesRead   int64
	bytesWritten int64
	lastReset   time.Time

	// Worker management
	workers     int
	workerWg    sync.WaitGroup
	stopCh      chan struct{}
	running     bool
	runningMu   sync.RWMutex

	// Configuration
	checkInterval time.Duration
	maxConcurrent int
}

// Config holds configuration for the compaction manager.
type Config struct {
	PlannerConfig  PlannerConfig
	ExecutorConfig ExecutorConfig
	CheckInterval  time.Duration // How often to check for compaction (default: 30s)
	MaxConcurrent  int           // Maximum concurrent compaction jobs (default: 1)
	MaxIOPS        int64         // Maximum IO operations per second (default: 1000)
	MaxBandwidth   int64         // Maximum bandwidth in bytes per second (default: 100MB/s)
}

// DefaultConfig returns a default manager configuration.
func DefaultConfig(sstableDir string) Config {
	return Config{
		PlannerConfig:  DefaultPlannerConfig(sstableDir),
		ExecutorConfig: DefaultExecutorConfig(sstableDir),
		CheckInterval:  30 * time.Second,
		MaxConcurrent:  1,
		MaxIOPS:        1000,
		MaxBandwidth:   100 * 1024 * 1024, // 100MB/s
	}
}

// IOLimiter limits IO operations to prevent overwhelming the system.
type IOLimiter struct {
	maxIOPS      int64
	maxBandwidth int64
	opsCount     int64
	bytesCount   int64
	windowStart  time.Time
	mu           sync.Mutex
}

// NewIOLimiter creates a new IO limiter.
func NewIOLimiter(maxIOPS, maxBandwidth int64) *IOLimiter {
	return &IOLimiter{
		maxIOPS:      maxIOPS,
		maxBandwidth: maxBandwidth,
		windowStart:  time.Now(),
	}
}

// Allow checks if an IO operation is allowed and records it.
func (l *IOLimiter) Allow(bytes int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowDuration := now.Sub(l.windowStart)

	// Reset counters every second
	if windowDuration >= time.Second {
		l.opsCount = 0
		l.bytesCount = 0
		l.windowStart = now
	}

	// Check IOPS limit
	if l.opsCount >= l.maxIOPS {
		return false
	}

	// Check bandwidth limit
	if l.bytesCount+bytes > l.maxBandwidth {
		return false
	}

	// Allow the operation
	l.opsCount++
	l.bytesCount += bytes
	return true
}

// Wait waits until an IO operation is allowed.
func (l *IOLimiter) Wait(ctx context.Context, bytes int64) error {
	for {
		if l.Allow(bytes) {
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

// NewManager creates a new compaction manager.
func NewManager(config Config) (*Manager, error) {
	planner := NewPlanner(config.PlannerConfig)
	executor, err := NewExecutor(config.ExecutorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return &Manager{
		planner:       planner,
		executor:      executor,
		jobQueue:      make([]CompactionJob, 0),
		ioLimiter:     NewIOLimiter(config.MaxIOPS, config.MaxBandwidth),
		checkInterval: config.CheckInterval,
		maxConcurrent: config.MaxConcurrent,
		stopCh:        make(chan struct{}),
		running:       false,
	}, nil
}

// Start starts the compaction manager.
// It begins checking for compaction opportunities and processing jobs.
func (m *Manager) Start() error {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if m.running {
		return fmt.Errorf("compaction manager already running")
	}

	m.running = true
	m.stopCh = make(chan struct{})

	// Start worker goroutines
	m.workers = m.maxConcurrent
	for i := 0; i < m.workers; i++ {
		m.workerWg.Add(1)
		go m.worker(i)
	}

	// Start scheduler goroutine
	m.workerWg.Add(1)
	go m.scheduler()

	return nil
}

// Stop stops the compaction manager.
func (m *Manager) Stop() error {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopCh)
	m.workerWg.Wait()

	return nil
}

// IsRunning returns whether the manager is currently running.
func (m *Manager) IsRunning() bool {
	m.runningMu.RLock()
	defer m.runningMu.RUnlock()
	return m.running
}

// scheduler periodically checks for compaction opportunities and schedules jobs.
func (m *Manager) scheduler() {
	defer m.workerWg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAndSchedule()
		}
	}
}

// checkAndSchedule checks if compaction is needed and schedules jobs.
func (m *Manager) checkAndSchedule() {
	shouldCompact, err := m.planner.ShouldCompact()
	if err != nil {
		// Log error but continue
		return
	}

	if !shouldCompact {
		return
	}

	// Plan compaction
	compactions, err := m.planner.PlanCompaction()
	if err != nil {
		// Log error but continue
		return
	}

	// Schedule jobs
	for _, inputPaths := range compactions {
		if len(inputPaths) == 0 {
			continue
		}

		job := CompactionJob{
			InputPaths: inputPaths,
			Priority:   len(inputPaths), // More files = higher priority
			CreatedAt:  time.Now(),
		}

		m.enqueueJob(job)
	}
}

// enqueueJob adds a job to the queue.
func (m *Manager) enqueueJob(job CompactionJob) {
	m.jobQueueMu.Lock()
	defer m.jobQueueMu.Unlock()

	m.jobQueue = append(m.jobQueue, job)

	// Sort by priority (higher first)
	sort.Slice(m.jobQueue, func(i, j int) bool {
		return m.jobQueue[i].Priority > m.jobQueue[j].Priority
	})
}

// dequeueJob removes and returns the highest priority job from the queue.
func (m *Manager) dequeueJob() *CompactionJob {
	m.jobQueueMu.Lock()
	defer m.jobQueueMu.Unlock()

	if len(m.jobQueue) == 0 {
		return nil
	}

	job := m.jobQueue[0]
	m.jobQueue = m.jobQueue[1:]
	return &job
}

// worker processes compaction jobs.
func (m *Manager) worker(id int) {
	defer m.workerWg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		default:
			// Try to get a job
			job := m.dequeueJob()
			if job == nil {
				// No jobs available, wait a bit
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process the job
			m.processJob(job)
		}
	}
}

// processJob processes a single compaction job with IO throttling.
func (m *Manager) processJob(job *CompactionJob) {
	ctx := context.Background()

	// Estimate IO for throttling (rough estimate: 2x input size for read+write)
	totalSize := int64(0)
	for _, path := range job.InputPaths {
		info, err := os.Stat(path)
		if err == nil {
			totalSize += info.Size()
		}
	}

	// Wait for IO allowance
	if err := m.ioLimiter.Wait(ctx, totalSize*2); err != nil {
		// Context cancelled or error, re-enqueue job
		m.enqueueJob(*job)
		return
	}

	// Execute compaction
	outputPath, err := m.executor.Compact(job.InputPaths)
	if err != nil {
		// Log error - job failed
		// In production, you might want to retry or handle errors differently
		return
	}

	job.OutputPath = outputPath
}

// TriggerCompaction manually triggers a compaction check and schedules jobs.
func (m *Manager) TriggerCompaction() error {
	if !m.IsRunning() {
		return fmt.Errorf("compaction manager is not running")
	}

	m.checkAndSchedule()
	return nil
}

// GetQueueSize returns the current number of jobs in the queue.
func (m *Manager) GetQueueSize() int {
	m.jobQueueMu.Lock()
	defer m.jobQueueMu.Unlock()
	return len(m.jobQueue)
}

// GetStats returns compaction statistics.
func (m *Manager) GetStats() Stats {
	m.jobQueueMu.Lock()
	defer m.jobQueueMu.Unlock()

	return Stats{
		QueueSize:    len(m.jobQueue),
		IsRunning:    m.IsRunning(),
		MaxConcurrent: m.maxConcurrent,
	}
}

// Stats holds compaction statistics.
type Stats struct {
	QueueSize     int
	IsRunning     bool
	MaxConcurrent int
}

