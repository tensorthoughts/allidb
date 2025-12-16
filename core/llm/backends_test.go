package llm

import (
	"testing"
)

func TestNewMockBackend(t *testing.T) {
	backend := NewMockBackend()
	if backend == nil {
		t.Fatal("NewMockBackend returned nil")
	}
	if backend.Responses == nil {
		t.Error("Responses slice not initialized")
	}
	if backend.Index != 0 {
		t.Error("Index should start at 0")
	}
}

func TestMockBackend_AddResponse(t *testing.T) {
	backend := NewMockBackend()
	backend.AddResponse("response1")
	backend.AddResponse("response2")

	if len(backend.Responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(backend.Responses))
	}
	if backend.Responses[0] != "response1" {
		t.Errorf("Expected first response 'response1', got '%s'", backend.Responses[0])
	}
}

func TestMockBackend_Generate(t *testing.T) {
	backend := NewMockBackend()
	backend.AddResponse("response1")
	backend.AddResponse("response2")

	response1, err := backend.Generate("prompt1")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if response1 != "response1" {
		t.Errorf("Expected 'response1', got '%s'", response1)
	}

	response2, err := backend.Generate("prompt2")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if response2 != "response2" {
		t.Errorf("Expected 'response2', got '%s'", response2)
	}
}

func TestMockBackend_Generate_NoMoreResponses(t *testing.T) {
	backend := NewMockBackend()
	// Don't add any responses

	_, err := backend.Generate("prompt")
	if err == nil {
		t.Fatal("Expected error when no responses available")
	}
}

func TestNewOpenAIBackend(t *testing.T) {
	config := OpenAIConfig{
		APIKey: "test-key",
	}
	backend := NewOpenAIBackend(config)

	if backend == nil {
		t.Fatal("NewOpenAIBackend returned nil")
	}
	if backend.APIKey != "test-key" {
		t.Errorf("Expected APIKey 'test-key', got '%s'", backend.APIKey)
	}
	if backend.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected default BaseURL, got '%s'", backend.BaseURL)
	}
	if backend.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected default Model 'gpt-3.5-turbo', got '%s'", backend.Model)
	}
	if backend.Client == nil {
		t.Error("HTTP client not initialized")
	}
}

func TestNewOpenAIBackend_CustomConfig(t *testing.T) {
	config := OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://custom.openai.com/v1",
		Model:   "gpt-4",
	}
	backend := NewOpenAIBackend(config)

	if backend.BaseURL != "https://custom.openai.com/v1" {
		t.Errorf("Expected custom BaseURL, got '%s'", backend.BaseURL)
	}
	if backend.Model != "gpt-4" {
		t.Errorf("Expected custom Model 'gpt-4', got '%s'", backend.Model)
	}
}

func TestNewOllamaBackend(t *testing.T) {
	config := OllamaConfig{
		Model: "llama2",
	}
	backend := NewOllamaBackend(config)

	if backend == nil {
		t.Fatal("NewOllamaBackend returned nil")
	}
	if backend.Model != "llama2" {
		t.Errorf("Expected Model 'llama2', got '%s'", backend.Model)
	}
	if backend.BaseURL != "http://localhost:11434" {
		t.Errorf("Expected default BaseURL, got '%s'", backend.BaseURL)
	}
	if backend.Client == nil {
		t.Error("HTTP client not initialized")
	}
}

func TestNewOllamaBackend_CustomConfig(t *testing.T) {
	config := OllamaConfig{
		BaseURL: "http://custom:11434",
		Model:   "mistral",
	}
	backend := NewOllamaBackend(config)

	if backend.BaseURL != "http://custom:11434" {
		t.Errorf("Expected custom BaseURL, got '%s'", backend.BaseURL)
	}
	if backend.Model != "mistral" {
		t.Errorf("Expected custom Model 'mistral', got '%s'", backend.Model)
	}
}

func TestNewOllamaBackend_DefaultModel(t *testing.T) {
	config := OllamaConfig{}
	backend := NewOllamaBackend(config)

	if backend.Model != "llama2" {
		t.Errorf("Expected default Model 'llama2', got '%s'", backend.Model)
	}
}

func TestNewLocalLLMBackend(t *testing.T) {
	config := LocalLLMConfig{
		BaseURL: "http://localhost:8080",
	}
	backend := NewLocalLLMBackend(config)

	if backend == nil {
		t.Fatal("NewLocalLLMBackend returned nil")
	}
	if backend.BaseURL != "http://localhost:8080" {
		t.Errorf("Expected BaseURL 'http://localhost:8080', got '%s'", backend.BaseURL)
	}
	if backend.Client == nil {
		t.Error("HTTP client not initialized")
	}
}

func TestNewLocalLLMBackend_DefaultURL(t *testing.T) {
	config := LocalLLMConfig{}
	backend := NewLocalLLMBackend(config)

	if backend.BaseURL != "http://localhost:8080" {
		t.Errorf("Expected default BaseURL, got '%s'", backend.BaseURL)
	}
}

func TestBackendInterface(t *testing.T) {
	// Test that all backends implement the Backend interface
	var backend Backend

	// MockBackend
	mockBackend := NewMockBackend()
	mockBackend.AddResponse("test")
	backend = mockBackend
	_, err := backend.Generate("test")
	if err != nil {
		t.Errorf("MockBackend.Generate failed: %v", err)
	}

	// OpenAI backend (we can't test actual API calls without a key)
	openAIBackend := NewOpenAIBackend(OpenAIConfig{APIKey: "test"})
	backend = openAIBackend
	_ = backend

	// Ollama backend
	ollamaBackend := NewOllamaBackend(OllamaConfig{Model: "llama2"})
	backend = ollamaBackend
	_ = backend

	// Local LLM backend
	localBackend := NewLocalLLMBackend(LocalLLMConfig{BaseURL: "http://localhost:8080"})
	backend = localBackend
	_ = backend
}

