package query

import (
	"math"
	"time"

	"github.com/tensorthoughts25/allidb/core/entity"
	"github.com/tensorthoughts25/allidb/core/index/hnsw"
)

// ScoringWeights holds configurable weights for different scoring factors.
type ScoringWeights struct {
	CosineSimilarity float64 // Weight for cosine similarity (default: 1.0)
	GraphWeight      float64 // Weight for graph edge weights (default: 0.3)
	Importance       float64 // Weight for entity importance (default: 0.2)
	Recency          float64 // Weight for recency decay (default: 0.1)
}

// DefaultScoringWeights returns default scoring weights.
func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		CosineSimilarity: 1.0,
		GraphWeight:      0.3,
		Importance:       0.2,
		Recency:          0.1,
	}
}

// Scorer computes unified scores for query results.
type Scorer struct {
	weights ScoringWeights
	now     time.Time // Current time for recency calculations
}

// NewScorer creates a new scorer with the given weights.
func NewScorer(weights ScoringWeights) *Scorer {
	return &Scorer{
		weights: weights,
		now:     time.Now(),
	}
}

// Candidate represents a candidate entity for scoring.
type Candidate struct {
	EntityID      entity.EntityID
	QueryVector   []float32
	Entity        *entity.UnifiedEntity
	GraphPath     []entity.EntityID // Path from query to this entity via graph
	GraphWeights  []float64          // Edge weights along the path
}

// Score computes the unified score for a candidate entity.
// The score combines:
//   - Cosine similarity between query and entity vector
//   - Graph edge weights along the path
//   - Entity importance score
//   - Recency decay based on entity update time
func (s *Scorer) Score(candidate *Candidate) float64 {
	if candidate.Entity == nil {
		return 0.0
	}

	// Get latest embedding
	embedding := candidate.Entity.GetLatestEmbedding()
	if embedding == nil {
		return 0.0
	}

	// 1. Cosine similarity score
	cosineScore := s.computeCosineScore(candidate.QueryVector, embedding.Vector)

	// 2. Graph weight score
	graphScore := s.computeGraphScore(candidate.GraphWeights)

	// 3. Importance score
	importanceScore := s.computeImportanceScore(candidate.Entity.GetImportance())

	// 4. Recency score
	recencyScore := s.computeRecencyScore(candidate.Entity.GetUpdatedAt())

	// Combine scores with weights
	totalScore := s.weights.CosineSimilarity*cosineScore +
		s.weights.GraphWeight*graphScore +
		s.weights.Importance*importanceScore +
		s.weights.Recency*recencyScore

	return totalScore
}

// computeCosineScore computes the cosine similarity score.
// Returns a value in [0, 1] where 1 means identical vectors.
func (s *Scorer) computeCosineScore(query, entity []float32) float64 {
	similarity := hnsw.CosineSimilarity(query, entity)
	// Normalize from [-1, 1] to [0, 1]
	return float64((similarity + 1.0) / 2.0)
}

// computeGraphScore computes the graph weight score.
// Uses the average edge weight along the path, or 1.0 if no path.
func (s *Scorer) computeGraphScore(weights []float64) float64 {
	if len(weights) == 0 {
		return 1.0 // No graph path, neutral score
	}

	// Average edge weight along the path
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	avgWeight := sum / float64(len(weights))

	// Normalize to [0, 1] (assuming weights are in [0, 1])
	if avgWeight > 1.0 {
		avgWeight = 1.0
	}
	if avgWeight < 0.0 {
		avgWeight = 0.0
	}

	return avgWeight
}

// computeImportanceScore computes the importance score.
// Normalizes importance to [0, 1] range.
func (s *Scorer) computeImportanceScore(importance float64) float64 {
	// Normalize importance using sigmoid-like function
	// This maps any importance value to [0, 1]
	return 1.0 / (1.0 + math.Exp(-importance))
}

// computeRecencyScore computes the recency decay score.
// More recent entities get higher scores.
func (s *Scorer) computeRecencyScore(updatedAt time.Time) float64 {
	age := s.now.Sub(updatedAt)

	// Exponential decay: score = e^(-age/halfLife)
	// Using 30 days as half-life
	halfLife := 30 * 24 * time.Hour
	decayFactor := math.Exp(-float64(age) / float64(halfLife))

	// Clamp to [0, 1]
	if decayFactor > 1.0 {
		decayFactor = 1.0
	}
	if decayFactor < 0.0 {
		decayFactor = 0.0
	}

	return decayFactor
}

// ScoreBatch computes scores for multiple candidates.
func (s *Scorer) ScoreBatch(candidates []*Candidate) []float64 {
	scores := make([]float64, len(candidates))
	for i, candidate := range candidates {
		scores[i] = s.Score(candidate)
	}
	return scores
}

// UpdateTime updates the current time used for recency calculations.
func (s *Scorer) UpdateTime(now time.Time) {
	s.now = now
}

