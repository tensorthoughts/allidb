package llm

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	mockBackend := NewMockBackend()
	config := DefaultConfig(mockBackend)

	if config.Backend != mockBackend {
		t.Error("Backend not set correctly")
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if config.RetryDelay != 1*time.Second {
		t.Errorf("Expected RetryDelay 1s, got %v", config.RetryDelay)
	}
	if config.ExtractionPrompt == "" {
		t.Error("Expected non-empty extraction prompt")
	}
}

func TestNewExtractor(t *testing.T) {
	mockBackend := NewMockBackend()
	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	if extractor == nil {
		t.Fatal("NewExtractor returned nil")
	}
	if extractor.config.Backend != mockBackend {
		t.Error("Extractor backend not set correctly")
	}
}

func TestExtractor_Extract_ValidJSON(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{
				"id": "entity1",
				"type": "Person",
				"name": "John Doe",
				"description": "A person"
			}
		],
		"relations": []
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John Doe is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}
	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(graph.Entities))
	}
	if graph.Entities[0].ID != "entity1" {
		t.Errorf("Expected entity ID 'entity1', got '%s'", graph.Entities[0].ID)
	}
}

func TestExtractor_Extract_WithRelations(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"},
			{"id": "e2", "type": "Company", "name": "Microsoft"}
		],
		"relations": [
			{"from_id": "e1", "to_id": "e2", "relation": "works_at", "weight": 0.9}
		]
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John works at Microsoft.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(graph.Entities))
	}
	if len(graph.Relations) != 1 {
		t.Errorf("Expected 1 relation, got %d", len(graph.Relations))
	}
	if graph.Relations[0].FromID != "e1" {
		t.Errorf("Expected relation from_id 'e1', got '%s'", graph.Relations[0].FromID)
	}
	if graph.Relations[0].Relation != "works_at" {
		t.Errorf("Expected relation type 'works_at', got '%s'", graph.Relations[0].Relation)
	}
}

func TestExtractor_Extract_WithMarkdownCodeBlock(t *testing.T) {
	mockBackend := NewMockBackend()
	jsonInMarkdown := "```json\n{\n  \"entities\": [{\"id\": \"e1\", \"type\": \"Person\", \"name\": \"John\"}],\n  \"relations\": []\n}\n```"
	mockBackend.AddResponse(jsonInMarkdown)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(graph.Entities))
	}
}

func TestExtractor_Extract_WithJSONCodeBlock(t *testing.T) {
	mockBackend := NewMockBackend()
	jsonInMarkdown := "```json\n{\"entities\": [{\"id\": \"e1\", \"type\": \"Person\", \"name\": \"John\"}], \"relations\": []}\n```"
	mockBackend.AddResponse(jsonInMarkdown)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(graph.Entities))
	}
}

func TestExtractor_Extract_InvalidJSON_Retry(t *testing.T) {
	mockBackend := NewMockBackend()
	// First response is invalid
	mockBackend.AddResponse("This is not JSON")
	// Second response is valid
	mockBackend.AddResponse(`{"entities": [{"id": "e1", "type": "Person", "name": "John"}], "relations": []}`)

	config := DefaultConfig(mockBackend)
	config.MaxRetries = 2
	config.RetryDelay = 10 * time.Millisecond // Fast retry for testing
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed after retry: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity after retry, got %d", len(graph.Entities))
	}
}

func TestExtractor_Extract_MaxRetriesExceeded(t *testing.T) {
	mockBackend := NewMockBackend()
	// All responses are invalid
	for i := 0; i < 5; i++ {
		mockBackend.AddResponse("Invalid JSON response")
	}

	config := DefaultConfig(mockBackend)
	config.MaxRetries = 2
	config.RetryDelay = 10 * time.Millisecond
	extractor := NewExtractor(config)

	_, err := extractor.Extract("Some text.")
	if err == nil {
		t.Fatal("Expected error after max retries exceeded")
	}
}

func TestExtractor_Extract_MissingEntityID(t *testing.T) {
	mockBackend := NewMockBackend()
	invalidJSON := `{
		"entities": [
			{"type": "Person", "name": "John"}
		],
		"relations": []
	}`
	mockBackend.AddResponse(invalidJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	_, err := extractor.Extract("John is a person.")
	if err == nil {
		t.Fatal("Expected error for missing entity ID")
	}
}

func TestExtractor_Extract_DuplicateEntityID(t *testing.T) {
	mockBackend := NewMockBackend()
	invalidJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"},
			{"id": "e1", "type": "Person", "name": "Jane"}
		],
		"relations": []
	}`
	mockBackend.AddResponse(invalidJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	_, err := extractor.Extract("John and Jane are people.")
	if err == nil {
		t.Fatal("Expected error for duplicate entity ID")
	}
}

func TestExtractor_Extract_InvalidRelation_NonExistentEntity(t *testing.T) {
	mockBackend := NewMockBackend()
	invalidJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"}
		],
		"relations": [
			{"from_id": "e1", "to_id": "e2", "relation": "knows"}
		]
	}`
	mockBackend.AddResponse(invalidJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	_, err := extractor.Extract("John knows someone.")
	if err == nil {
		t.Fatal("Expected error for relation referencing non-existent entity")
	}
}

func TestExtractor_Extract_MissingRelationFields(t *testing.T) {
	mockBackend := NewMockBackend()
	invalidJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"},
			{"id": "e2", "type": "Person", "name": "Jane"}
		],
		"relations": [
			{"from_id": "e1", "to_id": "e2"}
		]
	}`
	mockBackend.AddResponse(invalidJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	_, err := extractor.Extract("John knows Jane.")
	if err == nil {
		t.Fatal("Expected error for missing relation type")
	}
}

func TestExtractor_Extract_EmptyEntities(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [],
		"relations": []
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("Some text with no entities.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 0 {
		t.Errorf("Expected 0 entities, got %d", len(graph.Entities))
	}
}

func TestExtractor_Extract_EntityWithProperties(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{
				"id": "e1",
				"type": "Person",
				"name": "John",
				"properties": {
					"age": "30",
					"city": "New York"
				}
			}
		],
		"relations": []
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is 30 years old and lives in New York.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(graph.Entities))
	}

	entity := graph.Entities[0]
	if len(entity.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(entity.Properties))
	}
	if entity.Properties["age"] != "30" {
		t.Errorf("Expected age property '30', got '%s'", entity.Properties["age"])
	}
}

func TestExtractor_Extract_CustomPrompt(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{"entities": [{"id": "e1", "type": "Person", "name": "John"}], "relations": []}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	config.ExtractionPrompt = "Custom extraction prompt: Extract entities from the text."
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(graph.Entities))
	}
}

func TestExtractor_Extract_JSONWithWhitespace(t *testing.T) {
	mockBackend := NewMockBackend()
	jsonWithWhitespace := `   {
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"}
		],
		"relations": []
	}   `
	mockBackend.AddResponse(jsonWithWhitespace)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(graph.Entities))
	}
}

func TestExtractor_Extract_MultipleRelations(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"},
			{"id": "e2", "type": "Company", "name": "Microsoft"},
			{"id": "e3", "type": "City", "name": "Seattle"}
		],
		"relations": [
			{"from_id": "e1", "to_id": "e2", "relation": "works_at", "weight": 0.9},
			{"from_id": "e2", "to_id": "e3", "relation": "located_in", "weight": 1.0}
		]
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John works at Microsoft. Microsoft is located in Seattle.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Relations) != 2 {
		t.Errorf("Expected 2 relations, got %d", len(graph.Relations))
	}
}

func TestExtractor_Extract_BackendError(t *testing.T) {
	mockBackend := NewMockBackend()
	// Don't add any responses, so Generate will fail

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	_, err := extractor.Extract("Some text.")
	if err == nil {
		t.Fatal("Expected error when backend fails")
	}
}

func TestExtractor_Extract_ComplexGraph(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "Alice", "description": "Software engineer"},
			{"id": "e2", "type": "Person", "name": "Bob", "description": "Product manager"},
			{"id": "e3", "type": "Company", "name": "TechCorp", "description": "Technology company"},
			{"id": "e4", "type": "Project", "name": "ProjectX", "description": "Secret project"}
		],
		"relations": [
			{"from_id": "e1", "to_id": "e3", "relation": "works_at", "weight": 0.95},
			{"from_id": "e2", "to_id": "e3", "relation": "works_at", "weight": 0.9},
			{"from_id": "e1", "to_id": "e4", "relation": "works_on", "weight": 0.8},
			{"from_id": "e2", "to_id": "e4", "relation": "manages", "weight": 0.85},
			{"from_id": "e1", "to_id": "e2", "relation": "reports_to", "weight": 0.7}
		]
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("Alice and Bob work at TechCorp. Alice works on ProjectX and reports to Bob, who manages it.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 4 {
		t.Errorf("Expected 4 entities, got %d", len(graph.Entities))
	}
	if len(graph.Relations) != 5 {
		t.Errorf("Expected 5 relations, got %d", len(graph.Relations))
	}
}

func TestExtractor_Extract_RelationWithoutWeight(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"},
			{"id": "e2", "type": "Person", "name": "Jane"}
		],
		"relations": [
			{"from_id": "e1", "to_id": "e2", "relation": "knows"}
		]
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John knows Jane.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Relations) != 1 {
		t.Fatalf("Expected 1 relation, got %d", len(graph.Relations))
	}

	// Weight should default to 0.0 if not provided
	if graph.Relations[0].Weight != 0.0 {
		t.Logf("Note: Weight is %f (may be set by JSON unmarshaling)", graph.Relations[0].Weight)
	}
}

func TestExtractor_Extract_EntityWithoutDescription(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"}
		],
		"relations": []
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(graph.Entities))
	}

	// Description is optional
	if graph.Entities[0].Description != "" {
		t.Logf("Description: %s", graph.Entities[0].Description)
	}
}

func TestExtractor_Extract_EntityWithoutProperties(t *testing.T) {
	mockBackend := NewMockBackend()
	validJSON := `{
		"entities": [
			{"id": "e1", "type": "Person", "name": "John"}
		],
		"relations": []
	}`
	mockBackend.AddResponse(validJSON)

	config := DefaultConfig(mockBackend)
	extractor := NewExtractor(config)

	graph, err := extractor.Extract("John is a person.")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(graph.Entities))
	}

	// Properties are optional
	if graph.Entities[0].Properties == nil {
		t.Log("Properties is nil (optional field)")
	}
}

func TestExtractor_Extract_RetryDelay(t *testing.T) {
	mockBackend := NewMockBackend()
	mockBackend.AddResponse("Invalid")
	mockBackend.AddResponse(`{"entities": [{"id": "e1", "type": "Person", "name": "John"}], "relations": []}`)

	config := DefaultConfig(mockBackend)
	config.MaxRetries = 1
	config.RetryDelay = 50 * time.Millisecond
	extractor := NewExtractor(config)

	start := time.Now()
	graph, err := extractor.Extract("John is a person.")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(graph.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(graph.Entities))
	}

	// Should have waited at least the retry delay
	if duration < config.RetryDelay {
		t.Errorf("Expected delay of at least %v, got %v", config.RetryDelay, duration)
	}
}

