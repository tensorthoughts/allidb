package query

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/index/unified"
)

// EntityID is an alias for entity.EntityID
type EntityID = entity.EntityID

// TraceEvent represents a tracing event in the query execution pipeline.
type TraceEvent struct {
	Stage      string
	Timestamp  time.Time
	Duration   time.Duration
	Details    map[string]interface{}
}

// QueryResult represents a single query result.
type QueryResult struct {
	EntityID entity.EntityID
	Score    float64
	Entity   *entity.UnifiedEntity
}

// Executor executes queries using the unified index.
type Executor struct {
	index  *unified.UnifiedIndex
	scorer *Scorer
	config Config

	// Tracing
	traceEnabled bool
	traceMu      sync.Mutex
	traces       []TraceEvent
}

// Config holds configuration for the query executor.
type Config struct {
	// Scoring weights
	ScoringWeights ScoringWeights

	// Query parameters
	DefaultK            int           // Default number of results (default: 10)
	DefaultExpandFactor int           // Default graph expansion factor (default: 2)
	MaxCandidates       int           // Maximum candidates to consider (default: 1000)
	DefaultConsistency  string        // Default consistency level (ONE, QUORUM, ALL)
	MaxGraphHops        int           // Maximum graph hops for expansion
	Timeout             time.Duration // Query timeout

	// Tracing
	EnableTracing bool // Enable query tracing (default: false)
}

// DefaultConfig returns a default executor configuration.
func DefaultConfig() Config {
	return Config{
		ScoringWeights:     DefaultScoringWeights(),
		DefaultK:           10,
		DefaultExpandFactor: 2,
		MaxCandidates:       1000,
		EnableTracing:       false,
	}
}

// NewExecutor creates a new query executor.
func NewExecutor(index *unified.UnifiedIndex, config Config) *Executor {
	scorer := NewScorer(config.ScoringWeights)
	return &Executor{
		index:        index,
		scorer:       scorer,
		config:       config,
		traceEnabled: config.EnableTracing,
		traces:       make([]TraceEvent, 0),
	}
}

// Query executes a query and returns the top-k results.
// The query vector should already be embedded.
func (e *Executor) Query(ctx context.Context, queryVector []float32, k int) ([]QueryResult, error) {
	return e.QueryWithOptions(ctx, queryVector, k, 2)
}

// QueryWithOptions executes a query with custom options.
func (e *Executor) QueryWithOptions(ctx context.Context, queryVector []float32, k int, expandFactor int) ([]QueryResult, error) {
	// Apply timeout if configured
	if e.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.Timeout)
		defer cancel()
	}

	// Use default expand factor if not provided
	if expandFactor <= 0 {
		expandFactor = e.config.DefaultExpandFactor
	}
	
	// Limit expand factor to max graph hops
	if expandFactor > e.config.MaxGraphHops {
		expandFactor = e.config.MaxGraphHops
	}

	startTime := time.Now()
	e.trace("query_start", startTime, 0, map[string]interface{}{
		"k":            k,
		"expandFactor": expandFactor,
		"vectorDim":    len(queryVector),
	})

	// Step 1: ANN vector search
	annStart := time.Now()
	annCandidates, err := e.annSearch(ctx, queryVector, k*expandFactor)
	if err != nil {
		return nil, fmt.Errorf("ANN search failed: %w", err)
	}
	e.trace("ann_search", annStart, time.Since(annStart), map[string]interface{}{
		"candidates": len(annCandidates),
	})

	// Step 2: Graph expansion
	graphStart := time.Now()
	expandedCandidates, graphPaths := e.graphExpansion(ctx, annCandidates, expandFactor)
	e.trace("graph_expansion", graphStart, time.Since(graphStart), map[string]interface{}{
		"expanded": len(expandedCandidates),
	})

	// Step 3: Unified scoring
	scoringStart := time.Now()
	scoredResults := e.unifiedScoring(ctx, queryVector, expandedCandidates, graphPaths)
	e.trace("unified_scoring", scoringStart, time.Since(scoringStart), map[string]interface{}{
		"scored": len(scoredResults),
	})

	// Step 4: Top-k selection
	selectionStart := time.Now()
	topK := e.topKSelection(scoredResults, k)
	e.trace("topk_selection", selectionStart, time.Since(selectionStart), map[string]interface{}{
		"results": len(topK),
	})

	totalDuration := time.Since(startTime)
	e.trace("query_complete", startTime, totalDuration, map[string]interface{}{
		"totalResults": len(topK),
	})

	return topK, nil
}

// annSearch performs approximate nearest neighbor search using configured caps.
func (e *Executor) annSearch(ctx context.Context, queryVector []float32, candidateCount int) ([]entity.EntityID, error) {
	// Respect MaxCandidates to avoid exploding the candidate set.
	searchK := candidateCount
	if searchK > e.config.MaxCandidates {
		searchK = e.config.MaxCandidates
	}
	if searchK < e.config.DefaultK {
		searchK = e.config.DefaultK
	}

	// Use ef at least as large as searchK for better recall.
	ef := searchK
	if ef < e.config.MaxCandidates {
		ef = e.config.MaxCandidates
	}

	results, err := e.index.Search(queryVector, searchK, ef)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// graphExpansion expands candidates using graph neighbors with bounded BFS.
func (e *Executor) graphExpansion(ctx context.Context, candidates []entity.EntityID, expandFactor int) ([]entity.EntityID, map[entity.EntityID][]entity.EntityID) {
	maxHops := e.config.MaxGraphHops
	if maxHops < 1 {
		maxHops = 1
	}

	expanded := make(map[entity.EntityID]bool)
	graphPaths := make(map[entity.EntityID][]entity.EntityID) // Path from seed to node

	// Seed queue with initial ANN candidates.
	type queueItem struct {
		id    entity.EntityID
		depth int
		path  []entity.EntityID
	}
	queue := make([]queueItem, 0, len(candidates))
	for _, cand := range candidates {
		expanded[cand] = true
		graphPaths[cand] = []EntityID{cand}
		queue = append(queue, queueItem{id: cand, depth: 0, path: []entity.EntityID{cand}})
	}

	totalLimit := e.config.MaxCandidates

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxHops {
			continue
		}

		neighbors := e.getNeighbors(item.id)
		count := 0
		for _, neighbor := range neighbors {
			if expanded[neighbor] {
				continue
			}
			if count >= expandFactor {
				break
			}
			if len(expanded) >= totalLimit {
				break
			}

			expanded[neighbor] = true
			pathCopy := append([]EntityID{}, item.path...)
			pathCopy = append(pathCopy, neighbor)
			graphPaths[neighbor] = pathCopy
			queue = append(queue, queueItem{id: neighbor, depth: item.depth + 1, path: pathCopy})
			count++
		}
	}

	// Convert to slice
	result := make([]entity.EntityID, 0, len(expanded))
	for entityID := range expanded {
		result = append(result, entityID)
	}

	return result, graphPaths
}

// getNeighbors gets neighbors for an entity.
func (e *Executor) getNeighbors(entityID entity.EntityID) []entity.EntityID {
	return e.index.GetNeighbors(entityID)
}

// unifiedScoring computes unified scores for all candidates.
func (e *Executor) unifiedScoring(ctx context.Context, queryVector []float32, candidates []entity.EntityID, graphPaths map[entity.EntityID][]entity.EntityID) []QueryResult {
	results := make([]QueryResult, 0, len(candidates))

	for _, entityID := range candidates {
		// Get entity
		entity := e.index.GetEntity(entityID)
		if entity == nil {
			continue
		}

		// Get graph path and weights
		path := graphPaths[entityID]
		weights := e.computePathWeights(entityID, path)

		// Create candidate
		candidate := &Candidate{
			EntityID:     entityID,
			QueryVector:  queryVector,
			Entity:       entity,
			GraphPath:    path,
			GraphWeights: weights,
		}

		// Compute score
		score := e.scorer.Score(candidate)

		results = append(results, QueryResult{
			EntityID: entityID,
			Score:    score,
			Entity:   entity,
		})
	}

	return results
}

// computePathWeights computes edge weights along a graph path.
func (e *Executor) computePathWeights(entityID entity.EntityID, path []entity.EntityID) []float64 {
	if len(path) < 2 {
		return []float64{}
	}

	weights := make([]float64, 0, len(path)-1)
	
	// For each edge in the path, get the weight
	for i := 0; i < len(path)-1; i++ {
		from := path[i]
		to := path[i+1]
		
		// Get edge weight from graph cache
		weight := e.getEdgeWeight(from, to)
		weights = append(weights, weight)
	}

	return weights
}

// getEdgeWeight gets the weight of an edge between two entities.
func (e *Executor) getEdgeWeight(from, to entity.EntityID) float64 {
	// Get outgoing edges from 'from' entity
	edges := e.index.GetOutgoingEdges(from)
	
	// Find edge to 'to' entity
	for _, edge := range edges {
		if edge.ToEntity == to {
			return edge.Weight
		}
	}
	
	// If not found, check incoming edges (bidirectional)
	incomingEdges := e.index.GetIncomingNeighbors(from)
	for _, neighbor := range incomingEdges {
		if neighbor == to {
			// Get edges from 'to' to 'from'
			reverseEdges := e.index.GetOutgoingEdges(to)
			for _, edge := range reverseEdges {
				if edge.ToEntity == from {
					return edge.Weight
				}
			}
		}
	}
	
	// Default weight if edge not found
	return 0.5
}

// topKSelection selects the top-k results by score.
func (e *Executor) topKSelection(results []QueryResult, k int) []QueryResult {
	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top k
	if len(results) > k {
		return results[:k]
	}
	return results
}

// trace records a tracing event.
func (e *Executor) trace(stage string, startTime time.Time, duration time.Duration, details map[string]interface{}) {
	if !e.traceEnabled {
		return
	}

	e.traceMu.Lock()
	defer e.traceMu.Unlock()

	event := TraceEvent{
		Stage:     stage,
		Timestamp: startTime,
		Duration:  duration,
		Details:   details,
	}

	e.traces = append(e.traces, event)
}

// GetTrace returns the trace events for the last query.
func (e *Executor) GetTrace() []TraceEvent {
	e.traceMu.Lock()
	defer e.traceMu.Unlock()

	// Return a copy
	result := make([]TraceEvent, len(e.traces))
	copy(result, e.traces)
	return result
}

// ClearTrace clears the trace events.
func (e *Executor) ClearTrace() {
	e.traceMu.Lock()
	defer e.traceMu.Unlock()
	e.traces = e.traces[:0]
}

