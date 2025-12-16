package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIEmbedderConfig holds configuration for OpenAI embedder.
type OpenAIEmbedderConfig struct {
	APIKey  string
	Model   string
	Dim     int
	Timeout time.Duration
}

// OpenAIEmbedder implements the Embedder interface for OpenAI embeddings API.
type OpenAIEmbedder struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewOpenAIEmbedder creates a new OpenAI embedder.
func NewOpenAIEmbedder(config OpenAIEmbedderConfig) *OpenAIEmbedder {
	baseURL := "https://api.openai.com/v1"
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &OpenAIEmbedder{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		model:   config.Model,
		dim:     config.Dim,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Embed implements the Embedder interface.
func (e *OpenAIEmbedder) Embed(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/embeddings", e.baseURL)

	requestBody := map[string]interface{}{
		"model": e.model,
		"input": text,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	// Convert float64 to float32
	embedding := make([]float32, len(response.Data[0].Embedding))
	for i, v := range response.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Dimension implements the Embedder interface.
func (e *OpenAIEmbedder) Dimension() int {
	return e.dim
}

// OllamaEmbedderConfig holds configuration for Ollama embedder.
type OllamaEmbedderConfig struct {
	BaseURL string
	Model   string
	Dim     int
	Timeout time.Duration
}

// OllamaEmbedder implements the Embedder interface for Ollama embeddings API.
type OllamaEmbedder struct {
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewOllamaEmbedder creates a new Ollama embedder.
func NewOllamaEmbedder(config OllamaEmbedderConfig) *OllamaEmbedder {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   config.Model,
		dim:     config.Dim,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Embed implements the Embedder interface.
func (e *OllamaEmbedder) Embed(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embeddings", e.baseURL)

	requestBody := map[string]interface{}{
		"model": e.model,
		"prompt": text,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(response.Embedding))
	for i, v := range response.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Dimension implements the Embedder interface.
func (e *OllamaEmbedder) Dimension() int {
	return e.dim
}

// LocalEmbedderConfig holds configuration for local embedder.
type LocalEmbedderConfig struct {
	BaseURL string
	Model   string
	Dim     int
	Timeout time.Duration
}

// LocalEmbedder implements the Embedder interface for local embeddings API.
type LocalEmbedder struct {
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewLocalEmbedder creates a new local embedder.
func NewLocalEmbedder(config LocalEmbedderConfig) *LocalEmbedder {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &LocalEmbedder{
		baseURL: baseURL,
		model:   config.Model,
		dim:     config.Dim,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Embed implements the Embedder interface.
func (e *LocalEmbedder) Embed(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/embeddings", e.baseURL)

	requestBody := map[string]interface{}{
		"model": e.model,
		"input": text,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(response.Embedding))
	for i, v := range response.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Dimension implements the Embedder interface.
func (e *LocalEmbedder) Dimension() int {
	return e.dim
}

// CustomEmbedderConfig holds configuration for custom embedder.
type CustomEmbedderConfig struct {
	APIKey  string
	Model   string
	Dim     int
	Timeout time.Duration
}

// CustomEmbedder implements the Embedder interface for custom embeddings API.
// Uses OpenAI-compatible API format.
type CustomEmbedder struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewCustomEmbedder creates a new custom embedder.
func NewCustomEmbedder(config CustomEmbedderConfig) *CustomEmbedder {
	baseURL := "https://api.custom-ai.com/v1" // Placeholder, should be configurable
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &CustomEmbedder{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		model:   config.Model,
		dim:     config.Dim,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Embed implements the Embedder interface.
func (e *CustomEmbedder) Embed(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/embeddings", e.baseURL)

	requestBody := map[string]interface{}{
		"model": e.model,
		"input": text,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	// Convert float64 to float32
	embedding := make([]float32, len(response.Data[0].Embedding))
	for i, v := range response.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Dimension implements the Embedder interface.
func (e *CustomEmbedder) Dimension() int {
	return e.dim
}

