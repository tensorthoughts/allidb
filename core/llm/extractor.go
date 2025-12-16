package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EntityGraph represents extracted entities and their relationships.
type EntityGraph struct {
	Entities  []ExtractedEntity  `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// ExtractedEntity represents an entity extracted from text.
type ExtractedEntity struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// ExtractedRelation represents a relationship between entities.
type ExtractedRelation struct {
	FromID   string  `json:"from_id"`
	ToID     string  `json:"to_id"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight,omitempty"`
}

// Backend is the interface for LLM backends.
type Backend interface {
	// Generate generates a response from the LLM given a prompt.
	Generate(prompt string) (string, error)
}

// Config holds configuration for the extractor.
type Config struct {
	Backend        Backend
	MaxRetries     int           // Maximum retries on malformed responses (default: 3)
	RetryDelay     time.Duration // Delay between retries (default: 1s)
	Schema         string        // JSON schema for validation (optional)
	ExtractionPrompt string      // Custom extraction prompt (optional)
}

// DefaultConfig returns a default extractor configuration.
func DefaultConfig(backend Backend) Config {
	return Config{
		Backend:        backend,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
		ExtractionPrompt: defaultExtractionPrompt,
	}
}

// Extractor extracts entities and relations from text using LLM.
type Extractor struct {
	config Config
}

// NewExtractor creates a new extractor with the given configuration.
func NewExtractor(config Config) *Extractor {
	return &Extractor{
		config: config,
	}
}

// Extract extracts entities and relations from the given text.
func (e *Extractor) Extract(text string) (*EntityGraph, error) {
	prompt := e.buildPrompt(text)
	
	var graph *EntityGraph
	var err error
	
	// Retry loop
	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(e.config.RetryDelay)
		}
		
		// Generate response from LLM
		response, genErr := e.config.Backend.Generate(prompt)
		if genErr != nil {
			return nil, fmt.Errorf("LLM generation failed: %w", genErr)
		}
		
		// Parse and validate JSON
		graph, err = e.parseResponse(response)
		if err == nil {
			// Validate schema if provided
			if e.config.Schema != "" {
				if err := e.validateSchema(graph); err != nil {
					err = fmt.Errorf("schema validation failed: %w", err)
					continue // Retry
				}
			}
			
			// Success
			return graph, nil
		}
		
		// If this was the last attempt, return the error
		if attempt == e.config.MaxRetries {
			return nil, fmt.Errorf("failed after %d attempts: %w", e.config.MaxRetries+1, err)
		}
	}
	
	return nil, fmt.Errorf("unexpected error in retry loop")
}

// buildPrompt builds the extraction prompt.
func (e *Extractor) buildPrompt(text string) string {
	if e.config.ExtractionPrompt != "" {
		return fmt.Sprintf("%s\n\nText:\n%s", e.config.ExtractionPrompt, text)
	}
	return fmt.Sprintf("%s\n\nText:\n%s", defaultExtractionPrompt, text)
}

// parseResponse parses the LLM response into an EntityGraph.
func (e *Extractor) parseResponse(response string) (*EntityGraph, error) {
	// Try to extract JSON from response (may have markdown code blocks)
	jsonStr := extractJSON(response)
	
	var graph EntityGraph
	if err := json.Unmarshal([]byte(jsonStr), &graph); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Basic validation
	if err := e.validateGraph(&graph); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}
	
	return &graph, nil
}

// validateGraph performs basic validation on the extracted graph.
func (e *Extractor) validateGraph(graph *EntityGraph) error {
	// Check for duplicate entity IDs
	entityIDs := make(map[string]bool)
	for _, entity := range graph.Entities {
		if entity.ID == "" {
			return fmt.Errorf("entity missing ID")
		}
		if entityIDs[entity.ID] {
			return fmt.Errorf("duplicate entity ID: %s", entity.ID)
		}
		entityIDs[entity.ID] = true
	}
	
	// Validate relations reference existing entities
	for _, relation := range graph.Relations {
		if relation.FromID == "" || relation.ToID == "" {
			return fmt.Errorf("relation missing from_id or to_id")
		}
		if !entityIDs[relation.FromID] {
			return fmt.Errorf("relation references non-existent entity: %s", relation.FromID)
		}
		if !entityIDs[relation.ToID] {
			return fmt.Errorf("relation references non-existent entity: %s", relation.ToID)
		}
		if relation.Relation == "" {
			return fmt.Errorf("relation missing relation type")
		}
	}
	
	return nil
}

// validateSchema validates the graph against a JSON schema.
func (e *Extractor) validateSchema(graph *EntityGraph) error {
	// Basic schema validation - in a real implementation, you'd use a JSON schema library
	// For now, we do basic checks
	if e.config.Schema == "" {
		return nil
	}
	
	// This is a placeholder - full schema validation would require a library like
	// github.com/xeipuuv/gojsonschema
	// For now, we rely on validateGraph for basic validation
	return nil
}

// defaultExtractionPrompt is the default prompt for entity extraction.
const defaultExtractionPrompt = `Extract entities and their relationships from the following text. 
Return a JSON object with the following structure:
{
  "entities": [
    {
      "id": "unique_entity_id",
      "type": "entity_type",
      "name": "entity_name",
      "description": "optional_description",
      "properties": {"key": "value"}
    }
  ],
  "relations": [
    {
      "from_id": "entity_id",
      "to_id": "entity_id",
      "relation": "relation_type",
      "weight": 0.0-1.0
    }
  ]
}

Return ONLY valid JSON, no additional text or markdown formatting.`

// extractJSON extracts JSON from a response that may contain markdown code blocks.
func extractJSON(response string) string {
	// Remove markdown code blocks if present
	response = removeMarkdownCodeBlocks(response)
	
	// Try to find JSON object boundaries
	start := -1
	end := -1
	depth := 0
	
	for i, char := range response {
		if char == '{' {
			if start == -1 {
				start = i
			}
			depth++
		} else if char == '}' {
			depth--
			if depth == 0 && start != -1 {
				end = i + 1
				break
			}
		}
	}
	
	if start != -1 && end != -1 {
		return response[start:end]
	}
	
	// If no JSON object found, return the whole response
	return response
}

// removeMarkdownCodeBlocks removes markdown code blocks from text.
func removeMarkdownCodeBlocks(text string) string {
	// Remove ```json ... ``` or ``` ... ``` blocks
	result := text
	
	// Try to find ```json first
	for {
		start := findString(result, "```json")
		if start == -1 {
			break
		}
		
		// Find closing ```
		end := findString(result[start+7:], "```")
		if end == -1 {
			// No closing found, remove the opening
			result = result[:start] + result[start+7:]
			break
		}
		
		// Extract content between code blocks
		content := result[start+7 : start+7+end]
		// Remove the code block markers
		result = result[:start] + content + result[start+7+end+3:]
	}
	
	// Try to find generic ``` blocks
	for {
		start := findString(result, "```")
		if start == -1 {
			break
		}
		
		// Find closing ```
		end := findString(result[start+3:], "```")
		if end == -1 {
			// No closing found, remove the opening
			result = result[:start] + result[start+3:]
			break
		}
		
		// Extract content between code blocks
		content := result[start+3 : start+3+end]
		// Remove the code block markers
		result = result[:start] + content + result[start+3+end+3:]
	}
	
	return result
}

// findString finds the first occurrence of a substring.
func findString(s, substr string) int {
	return strings.Index(s, substr)
}

