package query

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/tensorthoughts25/allidb/core/entity"
)

func TestDefaultScoringWeights(t *testing.T) {
	weights := DefaultScoringWeights()

	if weights.CosineSimilarity != 1.0 {
		t.Errorf("Expected CosineSimilarity 1.0, got %f", weights.CosineSimilarity)
	}
	if weights.GraphWeight != 0.3 {
		t.Errorf("Expected GraphWeight 0.3, got %f", weights.GraphWeight)
	}
	if weights.Importance != 0.2 {
		t.Errorf("Expected Importance 0.2, got %f", weights.Importance)
	}
	if weights.Recency != 0.1 {
		t.Errorf("Expected Recency 0.1, got %f", weights.Recency)
	}
}

func TestScorer_Score_NilEntity(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())
	candidate := &Candidate{
		EntityID:    entity.EntityID("test"),
		QueryVector: []float32{1.0, 0.0, 0.0},
		Entity:      nil,
	}

	score := scorer.Score(candidate)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 for nil entity, got %f", score)
	}
}

func TestScorer_Score_NoEmbedding(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())
	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	// Don't add any embeddings

	candidate := &Candidate{
		EntityID:    entity.EntityID("test"),
		QueryVector: []float32{1.0, 0.0, 0.0},
		Entity:      ent,
	}

	score := scorer.Score(candidate)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 for entity without embedding, got %f", score)
	}
}

func TestScorer_Score_CosineSimilarity(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 1.0,
		GraphWeight:      0.0, // Disable other factors
		Importance:       0.0,
		Recency:          0.0,
	}
	scorer := NewScorer(weights)

	// Create entity with embedding
	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})

	query := []float32{1.0, 0.0, 0.0} // Identical vector
	candidate := &Candidate{
		EntityID:    entity.EntityID("test"),
		QueryVector: query,
		Entity:      ent,
		GraphPath:   []entity.EntityID{},
		GraphWeights: []float64{},
	}

	score := scorer.Score(candidate)
	// Identical vectors should have cosine similarity of 1.0, normalized to 1.0
	if score < 0.99 {
		t.Errorf("Expected score close to 1.0 for identical vectors, got %f", score)
	}
}

func TestScorer_Score_OrthogonalVectors(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 1.0,
		GraphWeight:      0.0,
		Importance:       0.0,
		Recency:          0.0,
	}
	scorer := NewScorer(weights)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})

	query := []float32{0.0, 1.0, 0.0} // Orthogonal vector
	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  query,
		Entity:       ent,
		GraphPath:    []entity.EntityID{},
		GraphWeights: []float64{},
	}

	score := scorer.Score(candidate)
	// Orthogonal vectors should have cosine similarity of 0.0, normalized to 0.5
	if math.Abs(score-0.5) > 0.01 {
		t.Errorf("Expected score close to 0.5 for orthogonal vectors, got %f", score)
	}
}

func TestScorer_Score_GraphWeights(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 0.0, // Disable cosine
		GraphWeight:      1.0,
		Importance:       0.0,
		Recency:          0.0,
	}
	scorer := NewScorer(weights)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})

	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  []float32{1.0, 0.0, 0.0},
		Entity:       ent,
		GraphPath:    []entity.EntityID{entity.EntityID("a"), entity.EntityID("test")},
		GraphWeights: []float64{0.8}, // Single edge with weight 0.8
	}

	score := scorer.Score(candidate)
	if math.Abs(score-0.8) > 0.01 {
		t.Errorf("Expected score 0.8 for graph weight, got %f", score)
	}
}

func TestScorer_Score_NoGraphPath(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 0.0,
		GraphWeight:      1.0,
		Importance:       0.0,
		Recency:          0.0,
	}
	scorer := NewScorer(weights)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})

	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  []float32{1.0, 0.0, 0.0},
		Entity:       ent,
		GraphPath:    []entity.EntityID{},
		GraphWeights: []float64{}, // No graph path
	}

	score := scorer.Score(candidate)
	// No graph path should return neutral score of 1.0
	if math.Abs(score-1.0) > 0.01 {
		t.Errorf("Expected score 1.0 for no graph path, got %f", score)
	}
}

func TestScorer_Score_Importance(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 0.0,
		GraphWeight:      0.0,
		Importance:       1.0,
		Recency:          0.0,
	}
	scorer := NewScorer(weights)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})
	ent.SetImportance(2.0) // High importance

	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  []float32{1.0, 0.0, 0.0},
		Entity:       ent,
		GraphPath:    []entity.EntityID{},
		GraphWeights: []float64{},
	}

	score := scorer.Score(candidate)
	// Importance of 2.0 should map to sigmoid(2.0) ≈ 0.88
	expected := 1.0 / (1.0 + math.Exp(-2.0))
	if math.Abs(score-expected) > 0.01 {
		t.Errorf("Expected score %f for importance 2.0, got %f", expected, score)
	}
}

func TestScorer_Score_Recency(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 0.0,
		GraphWeight:      0.0,
		Importance:       0.0,
		Recency:          1.0,
	}
	now := time.Now()
	scorer := NewScorer(weights)
	scorer.UpdateTime(now)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})
	// Entity updated just now (very recent)
	ent.SetImportance(0.0) // Reset importance

	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  []float32{1.0, 0.0, 0.0},
		Entity:       ent,
		GraphPath:    []entity.EntityID{},
		GraphWeights: []float64{},
	}

	score := scorer.Score(candidate)
	// Very recent entity should have recency score close to 1.0
	if score < 0.99 {
		t.Errorf("Expected score close to 1.0 for very recent entity, got %f", score)
	}
}

func TestScorer_Score_OldEntity(t *testing.T) {
	weights := ScoringWeights{
		CosineSimilarity: 0.0,
		GraphWeight:      0.0,
		Importance:       0.0,
		Recency:          1.0,
	}
	now := time.Now()
	scorer := NewScorer(weights)
	scorer.UpdateTime(now)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})
	// Simulate old entity by manually setting UpdatedAt (we can't do this directly,
	// but we can test with a scorer that has old time)
	oldTime := now.Add(-100 * 24 * time.Hour) // 100 days ago
	scorer.UpdateTime(oldTime.Add(100 * 24 * time.Hour)) // Set scorer time to now
	// The entity's UpdatedAt will be recent, so this test is limited
	// We'd need to modify entity to allow setting UpdatedAt for full test

	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  []float32{1.0, 0.0, 0.0},
		Entity:       ent,
		GraphPath:    []entity.EntityID{},
		GraphWeights: []float64{},
	}

	score := scorer.Score(candidate)
	// Recent entity should still score high
	_ = score
}

func TestScorer_Score_Combined(t *testing.T) {
	weights := DefaultScoringWeights()
	scorer := NewScorer(weights)

	ent := entity.NewUnifiedEntity(entity.EntityID("test"), "tenant", entity.EntityType("type"))
	ent.AddEmbedding([]float32{1.0, 0.0, 0.0})
	ent.SetImportance(1.0)

	candidate := &Candidate{
		EntityID:     entity.EntityID("test"),
		QueryVector:  []float32{1.0, 0.0, 0.0}, // Identical
		Entity:       ent,
		GraphPath:    []entity.EntityID{entity.EntityID("a"), entity.EntityID("test")},
		GraphWeights: []float64{0.8},
	}

	score := scorer.Score(candidate)
	// Should combine all factors
	if score <= 0.0 {
		t.Errorf("Expected positive combined score, got %f", score)
	}
	if score > 10.0 {
		t.Errorf("Expected reasonable combined score, got %f", score)
	}
}

func TestScorer_ScoreBatch(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())

	candidates := make([]*Candidate, 3)
	for i := 0; i < 3; i++ {
		ent := entity.NewUnifiedEntity(entity.EntityID(fmt.Sprintf("entity%d", i)), "tenant", entity.EntityType("type"))
		ent.AddEmbedding([]float32{float32(i), 0.0, 0.0})
		candidates[i] = &Candidate{
			EntityID:     entity.EntityID(fmt.Sprintf("entity%d", i)),
			QueryVector:  []float32{1.0, 0.0, 0.0},
			Entity:       ent,
			GraphPath:    []entity.EntityID{},
			GraphWeights: []float64{},
		}
	}

	scores := scorer.ScoreBatch(candidates)
	if len(scores) != 3 {
		t.Fatalf("Expected 3 scores, got %d", len(scores))
	}

	// All scores should be positive
	for i, score := range scores {
		if score <= 0.0 {
			t.Errorf("Expected positive score for candidate %d, got %f", i, score)
		}
	}
}

func TestScorer_UpdateTime(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())
	initialTime := scorer.now

	newTime := initialTime.Add(1 * time.Hour)
	scorer.UpdateTime(newTime)

	if !scorer.now.Equal(newTime) {
		t.Errorf("Expected time to be updated to %v, got %v", newTime, scorer.now)
	}
}

func TestScorer_ComputeGraphScore_MultipleWeights(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())

	weights := []float64{0.8, 0.6, 0.9}
	score := scorer.computeGraphScore(weights)

	expected := (0.8 + 0.6 + 0.9) / 3.0
	if math.Abs(score-expected) > 0.01 {
		t.Errorf("Expected average graph score %f, got %f", expected, score)
	}
}

func TestScorer_ComputeGraphScore_Clamp(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())

	// Test with weight > 1.0 (should be clamped)
	weights := []float64{1.5}
	score := scorer.computeGraphScore(weights)
	if score > 1.0 {
		t.Errorf("Expected clamped score <= 1.0, got %f", score)
	}

	// Test with weight < 0.0 (should be clamped)
	weights = []float64{-0.5}
	score = scorer.computeGraphScore(weights)
	if score < 0.0 {
		t.Errorf("Expected clamped score >= 0.0, got %f", score)
	}
}

func TestScorer_ComputeImportanceScore(t *testing.T) {
	scorer := NewScorer(DefaultScoringWeights())

	// Test with importance 0
	score := scorer.computeImportanceScore(0.0)
	expected := 1.0 / (1.0 + math.Exp(0.0))
	if math.Abs(score-expected) > 0.01 {
		t.Errorf("Expected importance score %f for 0.0, got %f", expected, score)
	}

	// Test with high importance
	score = scorer.computeImportanceScore(10.0)
	if score <= 0.5 {
		t.Errorf("Expected high importance score > 0.5, got %f", score)
	}
}

func TestScorer_ComputeRecencyScore(t *testing.T) {
	now := time.Now()
	scorer := NewScorer(DefaultScoringWeights())
	scorer.UpdateTime(now)

	// Very recent (just now)
	score := scorer.computeRecencyScore(now)
	if score < 0.99 {
		t.Errorf("Expected recency score close to 1.0 for recent time, got %f", score)
	}

	// 30 days ago (half-life)
	halfLifeAgo := now.Add(-30 * 24 * time.Hour)
	score = scorer.computeRecencyScore(halfLifeAgo)
	expected := math.Exp(-1.0) // e^(-1) ≈ 0.368
	if math.Abs(score-expected) > 0.05 {
		t.Errorf("Expected recency score ~%f for half-life, got %f", expected, score)
	}

	// Very old (100 days ago)
	oldTime := now.Add(-100 * 24 * time.Hour)
	score = scorer.computeRecencyScore(oldTime)
	if score > 0.1 {
		t.Errorf("Expected low recency score < 0.1 for old time, got %f", score)
	}
}

