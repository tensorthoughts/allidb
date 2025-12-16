package query

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/index/unified"
)

func setupTestIndex(t *testing.T) (*unified.UnifiedIndex, func()) {
	dir := filepath.Join(os.TempDir(), "executor_test_"+t.Name())
	err := os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Failed to clean test directory: %v", err)
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	config := unified.DefaultConfig(dir)
	config.ReloadInterval = 100 * time.Millisecond
	index, err := unified.New(config)
	if err != nil {
		t.Fatalf("Failed to create unified index: %v", err)
	}

	cleanup := func() {
		index.Close()
		os.RemoveAll(dir)
	}

	return index, cleanup
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DefaultK != 10 {
		t.Errorf("Expected DefaultK 10, got %d", config.DefaultK)
	}
	if config.DefaultExpandFactor != 2 {
		t.Errorf("Expected DefaultExpandFactor 2, got %d", config.DefaultExpandFactor)
	}
	if config.MaxCandidates != 1000 {
		t.Errorf("Expected MaxCandidates 1000, got %d", config.MaxCandidates)
	}
	if config.EnableTracing {
		t.Error("Expected EnableTracing false by default")
	}
}

func TestNewExecutor(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	if executor == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if executor.index != index {
		t.Error("Executor index not set correctly")
	}
	if executor.scorer == nil {
		t.Error("Executor scorer not initialized")
	}
}

func TestExecutor_Query_EmptyIndex(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	results, err := executor.Query(ctx, queryVector, 5)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Empty index should return empty results
	if results != nil && len(results) > 0 {
		t.Errorf("Expected empty results for empty index, got %d results", len(results))
	}
}

func TestExecutor_QueryWithOptions(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	results, err := executor.QueryWithOptions(ctx, queryVector, 5, 3)

	if err != nil {
		t.Fatalf("QueryWithOptions failed: %v", err)
	}

	// Should not error even with empty index
	_ = results
}

func TestExecutor_Query_WithTracing(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	config.EnableTracing = true
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	_, err := executor.Query(ctx, queryVector, 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Check trace events
	traces := executor.GetTrace()
	if len(traces) == 0 {
		t.Error("Expected trace events, got none")
	}

	// Check for expected stages
	stages := make(map[string]bool)
	for _, trace := range traces {
		stages[trace.Stage] = true
	}

	expectedStages := []string{"query_start", "ann_search", "graph_expansion", "unified_scoring", "topk_selection", "query_complete"}
	for _, stage := range expectedStages {
		if !stages[stage] {
			t.Errorf("Expected trace stage %s not found", stage)
		}
	}
}

func TestExecutor_Query_WithoutTracing(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	config.EnableTracing = false
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	_, err := executor.Query(ctx, queryVector, 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Without tracing, should have no trace events
	traces := executor.GetTrace()
	if len(traces) > 0 {
		t.Errorf("Expected no trace events when tracing disabled, got %d", len(traces))
	}
}

func TestExecutor_GetTrace(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	config.EnableTracing = true
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	_, err := executor.Query(ctx, queryVector, 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	traces1 := executor.GetTrace()
	if len(traces1) == 0 {
		t.Fatal("Expected trace events")
	}

	// Get trace again should return same events
	traces2 := executor.GetTrace()
	if len(traces1) != len(traces2) {
		t.Errorf("Expected same number of trace events, got %d vs %d", len(traces1), len(traces2))
	}

	// Verify trace event structure
	for _, trace := range traces1 {
		if trace.Stage == "" {
			t.Error("Trace event missing stage")
		}
		if trace.Timestamp.IsZero() {
			t.Error("Trace event missing timestamp")
		}
	}
}

func TestExecutor_ClearTrace(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	config.EnableTracing = true
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	_, err := executor.Query(ctx, queryVector, 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify traces exist
	traces := executor.GetTrace()
	if len(traces) == 0 {
		t.Fatal("Expected trace events before clear")
	}

	// Clear traces
	executor.ClearTrace()

	// Verify traces are cleared
	traces = executor.GetTrace()
	if len(traces) > 0 {
		t.Errorf("Expected no trace events after clear, got %d", len(traces))
	}
}

func TestExecutor_Query_ContextCancellation(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	queryVector := []float32{1.0, 0.0, 0.0}
	_, err := executor.Query(ctx, queryVector, 5)

	// Should handle cancellation gracefully
	// (Implementation may or may not check context, but shouldn't panic)
	_ = err
}

func TestExecutor_Query_TopKSelection(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}

	// Request k=3
	results, err := executor.Query(ctx, queryVector, 3)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should return at most 3 results
	if results != nil && len(results) > 3 {
		t.Errorf("Expected at most 3 results, got %d", len(results))
	}
}

func TestExecutor_Query_ResultsSorted(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	results, err := executor.Query(ctx, queryVector, 10)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Results should be sorted by score (descending)
	if len(results) > 1 {
		for i := 1; i < len(results); i++ {
			if results[i-1].Score < results[i].Score {
				t.Errorf("Results not sorted: result %d has score %f > result %d score %f",
					i-1, results[i-1].Score, i, results[i].Score)
			}
		}
	}
}

func TestExecutor_Query_ExpandFactor(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}

	// Test with different expand factors
	results1, err := executor.QueryWithOptions(ctx, queryVector, 5, 1)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	results2, err := executor.QueryWithOptions(ctx, queryVector, 5, 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// With higher expand factor, we might get more candidates
	// (though final results are still limited to k)
	_ = results1
	_ = results2
}

func TestExecutor_Query_CustomScoringWeights(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	weights := ScoringWeights{
		CosineSimilarity: 2.0,
		GraphWeight:      0.5,
		Importance:       0.3,
		Recency:          0.2,
	}

	config := DefaultConfig()
	config.ScoringWeights = weights
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	results, err := executor.Query(ctx, queryVector, 5)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should work with custom weights
	_ = results
}

func TestExecutor_Query_MultipleQueries(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}

	// Execute multiple queries
	for i := 0; i < 5; i++ {
		results, err := executor.Query(ctx, queryVector, 5)
		if err != nil {
			t.Fatalf("Query %d failed: %v", i, err)
		}
		_ = results
	}
}

func TestExecutor_Query_Concurrent(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}

	// Execute concurrent queries
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := executor.Query(ctx, queryVector, 5)
			if err != nil {
				t.Errorf("Concurrent query failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all queries
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestExecutor_Query_ZeroK(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	results, err := executor.Query(ctx, queryVector, 0)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should return empty results for k=0
	if results != nil && len(results) > 0 {
		t.Errorf("Expected empty results for k=0, got %d", len(results))
	}
}

func TestExecutor_Query_EmptyVector(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{} // Empty vector
	results, err := executor.Query(ctx, queryVector, 5)

	// Should handle empty vector gracefully
	_ = results
	_ = err
}

func TestExecutor_QueryResult_Structure(t *testing.T) {
	index, cleanup := setupTestIndex(t)
	defer cleanup()

	config := DefaultConfig()
	executor := NewExecutor(index, config)

	ctx := context.Background()
	queryVector := []float32{1.0, 0.0, 0.0}
	results, err := executor.Query(ctx, queryVector, 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify result structure
	for _, result := range results {
		if result.EntityID == "" {
			t.Error("Result missing EntityID")
		}
		if result.Score < 0.0 {
			t.Errorf("Result has negative score: %f", result.Score)
		}
		// Entity may be nil for empty index
	}
}

