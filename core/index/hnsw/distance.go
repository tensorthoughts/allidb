package hnsw

import (
	"math"
)

// CosineDistance computes the cosine distance between two float32 vectors.
// Cosine distance = 1 - cosine similarity
// Returns a value in [0, 2] where 0 means identical vectors.
// This function is optimized for performance and does not allocate.
func CosineDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return math.MaxFloat32 // Return max distance for mismatched dimensions
	}

	var dotProduct float64
	var normA float64
	var normB float64

	// Compute dot product and norms in a single pass
	for i := 0; i < len(a); i++ {
		ai := float64(a[i])
		bi := float64(b[i])
		dotProduct += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	// Avoid division by zero
	if normA == 0 || normB == 0 {
		return 1.0 // Orthogonal or zero vectors
	}

	// Cosine similarity = dotProduct / (sqrt(normA) * sqrt(normB))
	// Cosine distance = 1 - cosine similarity
	similarity := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	
	// Clamp similarity to [-1, 1] to handle floating point errors
	if similarity > 1.0 {
		similarity = 1.0
	} else if similarity < -1.0 {
		similarity = -1.0
	}

	return float32(1.0 - similarity)
}

// CosineSimilarity computes the cosine similarity between two float32 vectors.
// Returns a value in [-1, 1] where 1 means identical vectors.
// This function is optimized for performance and does not allocate.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return -1.0 // Return minimum similarity for mismatched dimensions
	}

	var dotProduct float64
	var normA float64
	var normB float64

	// Compute dot product and norms in a single pass
	for i := 0; i < len(a); i++ {
		ai := float64(a[i])
		bi := float64(b[i])
		dotProduct += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	// Avoid division by zero
	if normA == 0 || normB == 0 {
		return 0.0 // Orthogonal or zero vectors
	}

	// Cosine similarity = dotProduct / (sqrt(normA) * sqrt(normB))
	similarity := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	
	// Clamp similarity to [-1, 1] to handle floating point errors
	if similarity > 1.0 {
		similarity = 1.0
	} else if similarity < -1.0 {
		similarity = -1.0
	}

	return float32(similarity)
}

