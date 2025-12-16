package ai

import (
	"github.com/tensorthoughts25/allidb/core/llm"
)

// Embedder is the interface for text embedding providers.
// Embedders convert text into vector embeddings for similarity search.
type Embedder interface {
	// Embed converts text into a vector embedding.
	// Returns a slice of float32 values representing the embedding.
	Embed(text string) ([]float32, error)

	// Dimension returns the dimension of embeddings produced by this embedder.
	// This is useful for validating that embeddings match expected dimensions.
	Dimension() int
}

// LLM is the interface for Large Language Model providers.
// LLMs generate text completions based on prompts.
type LLM interface {
	// Complete generates a text completion from a prompt.
	// Returns the generated text or an error if generation fails.
	Complete(prompt string) (string, error)
}

// EntityExtractor is the interface for entity and relation extraction.
// Extractors analyze text and extract structured entities and their relationships.
type EntityExtractor interface {
	// Extract analyzes text and extracts entities and their relationships.
	// Returns an EntityGraph containing extracted entities and relations.
	Extract(text string) (*llm.EntityGraph, error)
}

