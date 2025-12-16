package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIBackend implements the Backend interface for OpenAI API.
type OpenAIBackend struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

// OpenAIConfig holds configuration for OpenAI backend.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // Optional, defaults to https://api.openai.com/v1
	Model   string // Optional, defaults to gpt-3.5-turbo
}

// NewOpenAIBackend creates a new OpenAI backend.
func NewOpenAIBackend(config OpenAIConfig) *OpenAIBackend {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	
	model := config.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	
	return &OpenAIBackend{
		APIKey:  config.APIKey,
		BaseURL: baseURL,
		Model:   model,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Generate generates a response using OpenAI API.
func (b *OpenAIBackend) Generate(prompt string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", b.BaseURL)
	
	requestBody := map[string]interface{}{
		"model": b.Model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.0, // Deterministic output
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b.APIKey))
	
	resp, err := b.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	
	return response.Choices[0].Message.Content, nil
}

// OllamaBackend implements the Backend interface for Ollama API.
type OllamaBackend struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

// OllamaConfig holds configuration for Ollama backend.
type OllamaConfig struct {
	BaseURL string // Optional, defaults to http://localhost:11434
	Model   string // Required, e.g., "llama2", "mistral"
}

// NewOllamaBackend creates a new Ollama backend.
func NewOllamaBackend(config OllamaConfig) *OllamaBackend {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	
	if config.Model == "" {
		config.Model = "llama2"
	}
	
	return &OllamaBackend{
		BaseURL: baseURL,
		Model:   config.Model,
		Client: &http.Client{
			Timeout: 60 * time.Second, // Ollama can be slower
		},
	}
}

// Generate generates a response using Ollama API.
func (b *OllamaBackend) Generate(prompt string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", b.BaseURL)
	
	requestBody := map[string]interface{}{
		"model":  b.Model,
		"prompt": prompt,
		"stream": false,
		"format": "json", // Request JSON format
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := b.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Response string `json:"response"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	return response.Response, nil
}

// LocalLLMBackend implements the Backend interface for local LLM via HTTP.
// This is a generic interface for any local LLM server that follows a simple API.
type LocalLLMBackend struct {
	BaseURL string
	Client  *http.Client
}

// LocalLLMConfig holds configuration for local LLM backend.
type LocalLLMConfig struct {
	BaseURL string // Required, e.g., "http://localhost:8080"
}

// NewLocalLLMBackend creates a new local LLM backend.
func NewLocalLLMBackend(config LocalLLMConfig) *LocalLLMBackend {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080"
	}
	
	return &LocalLLMBackend{
		BaseURL: config.BaseURL,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Generate generates a response using local LLM API.
// Assumes a simple POST /generate endpoint with {"prompt": "..."} and {"response": "..."}
func (b *LocalLLMBackend) Generate(prompt string) (string, error) {
	url := fmt.Sprintf("%s/generate", b.BaseURL)
	
	requestBody := map[string]string{
		"prompt": prompt,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := b.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Response string `json:"response"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	return response.Response, nil
}

// MockBackend is a mock backend for testing.
type MockBackend struct {
	Responses []string
	Index     int
}

// NewMockBackend creates a new mock backend.
func NewMockBackend() *MockBackend {
	return &MockBackend{
		Responses: make([]string, 0),
		Index:     0,
	}
}

// AddResponse adds a response to the mock backend.
func (b *MockBackend) AddResponse(response string) {
	b.Responses = append(b.Responses, response)
}

// Generate generates a mock response.
func (b *MockBackend) Generate(prompt string) (string, error) {
	if b.Index >= len(b.Responses) {
		return "", fmt.Errorf("no more mock responses")
	}
	
	response := b.Responses[b.Index]
	b.Index++
	return response, nil
}

