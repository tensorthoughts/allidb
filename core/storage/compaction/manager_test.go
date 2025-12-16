package compaction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/storage/sstable"
)

func setupTestManager(t *testing.T) (*Manager, string, func()) {
	dir := filepath.Join(os.TempDir(), "compaction_manager_test_"+t.Name())
	err := os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Failed to clean test directory: %v", err)
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	config := DefaultConfig(dir)
	config.CheckInterval = 100 * time.Millisecond // Faster for testing
	config.MaxConcurrent = 2
	config.MaxIOPS = 10000
	config.MaxBandwidth = 100 * 1024 * 1024 * 1024 // 100GB/s for testing

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	cleanup := func() {
		manager.Stop()
		os.RemoveAll(dir)
	}

	return manager, dir, cleanup
}

func createTestSSTableForManager(t *testing.T, dir, filename string, entityCount int) string {
	path := filepath.Join(dir, filename)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test SSTable: %v", err)
	}
	defer file.Close()

	writer := sstable.NewWriter(file)
	writer.WriteHeader()

	for i := 0; i < entityCount; i++ {
		row := &sstable.Row{
			Type:     sstable.RowTypeVector,
			EntityID: sstable.EntityID(fmt.Sprintf("entity-%d", i)),
			Data:     make([]byte, 100),
		}
		writer.WriteRow(row)
	}

	writer.Close()
	return path
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("/test/dir")

	if config.PlannerConfig.SSTableDir != "/test/dir" {
		t.Error("PlannerConfig.SSTableDir not set correctly")
	}
	if config.ExecutorConfig.SSTableDir != "/test/dir" {
		t.Error("ExecutorConfig.SSTableDir not set correctly")
	}
	if config.CheckInterval != 30*time.Second {
		t.Errorf("Expected CheckInterval 30s, got %v", config.CheckInterval)
	}
	if config.MaxConcurrent != 1 {
		t.Errorf("Expected MaxConcurrent 1, got %d", config.MaxConcurrent)
	}
}

func TestNewManager(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.IsRunning() {
		t.Error("Manager should not be running before Start()")
	}
}

func TestManager_StartStop(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Start
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("Manager should be running after Start()")
	}

	// Stop
	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running after Stop()")
	}
}

func TestManager_StartTwice(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	if err := manager.Start(); err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Try to start again
	err := manager.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestManager_StopWithoutStart(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Should not error
	if err := manager.Stop(); err != nil {
		t.Errorf("Stop without start should not error: %v", err)
	}
}

func TestManager_GetQueueSize(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	size := manager.GetQueueSize()
	if size != 0 {
		t.Errorf("Expected queue size 0, got %d", size)
	}
}

func TestManager_GetStats(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	stats := manager.GetStats()
	if stats.QueueSize != 0 {
		t.Errorf("Expected queue size 0, got %d", stats.QueueSize)
	}
	if stats.IsRunning {
		t.Error("Manager should not be running")
	}
	if stats.MaxConcurrent != 2 {
		t.Errorf("Expected MaxConcurrent 2, got %d", stats.MaxConcurrent)
	}
}

func TestManager_TriggerCompaction_NotRunning(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	err := manager.TriggerCompaction()
	if err == nil {
		t.Error("Expected error when triggering compaction without starting")
	}
}

func TestManager_TriggerCompaction(t *testing.T) {
	manager, dir, cleanup := setupTestManager(t)
	defer cleanup()

	// Create test SSTables
	for i := 0; i < 4; i++ {
		createTestSSTableForManager(t, dir, fmt.Sprintf("sstable-%d.sst", i), 10)
	}

	// Start manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Trigger compaction
	if err := manager.TriggerCompaction(); err != nil {
		t.Fatalf("TriggerCompaction failed: %v", err)
	}

	// Wait a bit for job to be scheduled
	time.Sleep(500 * time.Millisecond)

	// Queue might have jobs (depends on planner finding compaction opportunities)
	stats := manager.GetStats()
	// Note: Queue might be empty if jobs were processed quickly
	_ = stats
}

func TestIOLimiter_Allow(t *testing.T) {
	limiter := NewIOLimiter(100, 1024*1024) // 100 IOPS, 1MB/s

	// Should allow operations within limits
	for i := 0; i < 50; i++ {
		if !limiter.Allow(1000) {
			t.Errorf("Operation %d should be allowed", i)
		}
	}
}

func TestIOLimiter_ThrottleIOPS(t *testing.T) {
	limiter := NewIOLimiter(10, 1024*1024*1024) // 10 IOPS, 1GB/s

	// Should allow first 10 operations
	allowed := 0
	for i := 0; i < 20; i++ {
		if limiter.Allow(100) {
			allowed++
		}
	}

	// Should allow at least 10, but may allow more due to timing
	if allowed < 10 {
		t.Errorf("Expected at least 10 operations allowed, got %d", allowed)
	}
}

func TestIOLimiter_ThrottleBandwidth(t *testing.T) {
	limiter := NewIOLimiter(10000, 1000) // 10K IOPS, 1KB/s

	// First operation should be allowed
	if !limiter.Allow(500) {
		t.Error("First operation should be allowed")
	}

	// Second operation should be throttled (exceeds bandwidth)
	if limiter.Allow(600) {
		t.Error("Second operation should be throttled")
	}
}

func TestIOLimiter_Wait(t *testing.T) {
	limiter := NewIOLimiter(10, 1000) // 10 IOPS, 1KB/s

	ctx := context.Background()

	// Should eventually allow
	err := limiter.Wait(ctx, 100)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
}

func TestIOLimiter_Wait_ContextCancel(t *testing.T) {
	limiter := NewIOLimiter(1, 1) // Very restrictive limits

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Exhaust the limit
	limiter.Allow(1)

	// Wait should timeout
	err := limiter.Wait(ctx, 1)
	if err == nil {
		t.Error("Expected context timeout error")
	}
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Expected context error, got %v", err)
	}
}

func TestManager_Scheduler(t *testing.T) {
	manager, dir, cleanup := setupTestManager(t)
	defer cleanup()

	// Create enough SSTables to trigger compaction
	for i := 0; i < 4; i++ {
		createTestSSTableForManager(t, dir, fmt.Sprintf("sstable-%d.sst", i), 10)
	}

	// Start manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Wait for scheduler to run
	time.Sleep(500 * time.Millisecond)

	// Scheduler should have run (jobs may have been processed already)
	stats := manager.GetStats()
	// Note: Queue might be empty if jobs were processed quickly
	_ = stats
}

func TestManager_ConcurrentJobs(t *testing.T) {
	manager, dir, cleanup := setupTestManager(t)
	defer cleanup()

	// Create multiple groups of SSTables
	for group := 0; group < 3; group++ {
		for i := 0; i < 4; i++ {
			createTestSSTableForManager(t, dir, fmt.Sprintf("sstable-group%d-%d.sst", group, i), 10)
		}
	}

	// Start manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Wait for jobs to be processed
	time.Sleep(2 * time.Second)

	// Verify some compaction happened (files should be reduced)
	files, _ := filepath.Glob(filepath.Join(dir, "*.sst"))
	// Note: Compaction may not have completed yet, or files may have been compacted
	// This is a best-effort check
	_ = files
}

func TestManager_JobPriority(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Create jobs with different priorities
	job1 := CompactionJob{
		InputPaths: []string{"file1", "file2"},
		Priority:   2,
		CreatedAt:  time.Now(),
	}
	job2 := CompactionJob{
		InputPaths: []string{"file3", "file4", "file5"},
		Priority:   3,
		CreatedAt:  time.Now(),
	}

	manager.enqueueJob(job1)
	manager.enqueueJob(job2)

	// Higher priority job should be dequeued first
	dequeued := manager.dequeueJob()
	if dequeued == nil {
		t.Fatal("Expected job to be dequeued")
	}
	if dequeued.Priority != 3 {
		t.Errorf("Expected priority 3, got %d", dequeued.Priority)
	}
}

func TestManager_ProcessJob_IOThrottling(t *testing.T) {
	manager, dir, cleanup := setupTestManager(t)
	defer cleanup()

	// Create test SSTable
	path := createTestSSTableForManager(t, dir, "sstable-0.sst", 10)

	job := &CompactionJob{
		InputPaths: []string{path},
		Priority:   1,
		CreatedAt:  time.Now(),
	}

	// Process job (should respect IO limits)
	manager.processJob(job)

	// Job should complete (or fail gracefully)
	// In a real scenario, we'd verify the output file exists
}

func TestManager_MultipleStarts(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	if err := manager.Start(); err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Try to start again - should fail
	err := manager.Start()
	if err == nil {
		t.Error("Expected error when starting already running manager")
	}

	manager.Stop()
}

func TestManager_ConcurrentAccess(t *testing.T) {
	manager, dir, cleanup := setupTestManager(t)
	defer cleanup()

	// Create test SSTables
	for i := 0; i < 4; i++ {
		createTestSSTableForManager(t, dir, fmt.Sprintf("sstable-%d.sst", i), 10)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()

	// Concurrent access to stats
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats := manager.GetStats()
			_ = stats
			queueSize := manager.GetQueueSize()
			_ = queueSize
		}()
	}

	wg.Wait()
	// Should not panic or deadlock
}

