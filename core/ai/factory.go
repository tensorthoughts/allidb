package ai

import (
	"fmt"
	"time"

	"github.com/tensorthoughts25/allidb/core/config"
	"github.com/tensorthoughts25/allidb/core/llm"
	"github.com/tensorthoughts25/allidb/core/security/secrets"
)

// Factory creates AI providers based on configuration.
type Factory struct {
	config        config.AIConfig
	secretProvider secrets.Provider
}

// NewFactory creates a new AI provider factory.
func NewFactory(aiConfig config.AIConfig, secretProvider secrets.Provider) *Factory {
	return &Factory{
		config:        aiConfig,
		secretProvider: secretProvider,
	}
}

// CreateEmbedder creates an embedder based on the embeddings configuration.
func (f *Factory) CreateEmbedder() (Embedder, error) {
	provider := f.config.Embeddings.Provider
	model := f.config.Embeddings.Model
	dim := f.config.Embeddings.Dim
	timeout := time.Duration(f.config.Embeddings.TimeoutMs) * time.Millisecond

	switch provider {
	case "openai":
		return f.createOpenAIEmbedder(model, dim, timeout)
	case "ollama":
		return f.createOllamaEmbedder(model, dim, timeout)
	case "local":
		return f.createLocalEmbedder(model, dim, timeout)
	case "custom":
		return f.createCustomEmbedder(model, dim, timeout)
	default:
		return nil, fmt.Errorf("unsupported embeddings provider: %s", provider)
	}
}

// CreateLLM creates an LLM client based on the LLM configuration.
func (f *Factory) CreateLLM() (LLM, error) {
	provider := f.config.LLM.Provider
	model := f.config.LLM.Model
	temperature := f.config.LLM.Temperature
	timeout := time.Duration(f.config.LLM.TimeoutMs) * time.Millisecond

	switch provider {
	case "openai":
		return f.createOpenAILLM(model, temperature, timeout)
	case "ollama":
		return f.createOllamaLLM(model, temperature, timeout)
	case "local":
		return f.createLocalLLM(model, temperature, timeout)
	case "custom":
		return f.createCustomLLM(model, temperature, timeout)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

// CreateEntityExtractor creates an entity extractor based on the configuration.
func (f *Factory) CreateEntityExtractor() (EntityExtractor, error) {
	if !f.config.EntityExtraction.Enabled {
		return nil, fmt.Errorf("entity extraction is not enabled")
	}

	// Create LLM backend for the extractor
	llmClient, err := f.CreateLLM()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM for extractor: %w", err)
	}

	// Wrap LLM interface to llm.Backend interface
	backend := &llmBackendAdapter{llm: llmClient}

	// Create extractor config
	extractorConfig := llm.DefaultConfig(backend)
	extractorConfig.MaxRetries = f.config.EntityExtraction.MaxRetries

	// Create and return extractor
	extractor := llm.NewExtractor(extractorConfig)
	return &entityExtractorAdapter{extractor: extractor}, nil
}

// createOpenAIEmbedder creates an OpenAI embedder.
func (f *Factory) createOpenAIEmbedder(model string, dim int, timeout time.Duration) (Embedder, error) {
	// Get API key from secret provider
	apiKey, err := f.secretProvider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAI API key: %w", err)
	}

	return NewOpenAIEmbedder(OpenAIEmbedderConfig{
		APIKey:  apiKey,
		Model:   model,
		Dim:     dim,
		Timeout: timeout,
	}), nil
}

// createOllamaEmbedder creates an Ollama embedder.
func (f *Factory) createOllamaEmbedder(model string, dim int, timeout time.Duration) (Embedder, error) {
	return NewOllamaEmbedder(OllamaEmbedderConfig{
		BaseURL: "http://localhost:11434", // Default Ollama URL
		Model:   model,
		Dim:     dim,
		Timeout: timeout,
	}), nil
}

// createLocalEmbedder creates a local embedder.
func (f *Factory) createLocalEmbedder(model string, dim int, timeout time.Duration) (Embedder, error) {
	return NewLocalEmbedder(LocalEmbedderConfig{
		BaseURL: "http://localhost:8080", // Default local URL
		Model:   model,
		Dim:     dim,
		Timeout: timeout,
	}), nil
}

// createCustomEmbedder creates a custom embedder.
func (f *Factory) createCustomEmbedder(model string, dim int, timeout time.Duration) (Embedder, error) {
	// Get custom API key from secret provider
	apiKey, err := f.secretProvider.GetSecret("CUSTOM_AI_KEY")
	if err != nil {
		return nil, fmt.Errorf("failed to get custom AI key: %w", err)
	}

	return NewCustomEmbedder(CustomEmbedderConfig{
		APIKey:  apiKey,
		Model:   model,
		Dim:     dim,
		Timeout: timeout,
	}), nil
}

// createOpenAILLM creates an OpenAI LLM client.
func (f *Factory) createOpenAILLM(model string, temperature float64, timeout time.Duration) (LLM, error) {
	// Get API key from secret provider
	apiKey, err := f.secretProvider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAI API key: %w", err)
	}

	backend := llm.NewOpenAIBackend(llm.OpenAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com/v1",
		Model:   model,
	})

	// Set timeout if provided
	if timeout > 0 {
		backend.Client.Timeout = timeout
	}

	// Note: Temperature is stored in the adapter but OpenAI backend currently hardcodes it to 0.0
	// In production, the backend should be updated to accept temperature as a parameter
	return &llmAdapter{backend: backend, temperature: temperature}, nil
}

// createOllamaLLM creates an Ollama LLM client.
func (f *Factory) createOllamaLLM(model string, temperature float64, timeout time.Duration) (LLM, error) {
	backend := llm.NewOllamaBackend(llm.OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   model,
	})

	// Set timeout if provided
	if timeout > 0 {
		backend.Client.Timeout = timeout
	}

	return &llmAdapter{backend: backend, temperature: temperature}, nil
}

// createLocalLLM creates a local LLM client.
func (f *Factory) createLocalLLM(model string, temperature float64, timeout time.Duration) (LLM, error) {
	backend := llm.NewLocalLLMBackend(llm.LocalLLMConfig{
		BaseURL: "http://localhost:8080",
	})

	// Set timeout if provided
	if timeout > 0 {
		backend.Client.Timeout = timeout
	}

	return &llmAdapter{backend: backend, temperature: temperature}, nil
}

// createCustomLLM creates a custom LLM client.
func (f *Factory) createCustomLLM(model string, temperature float64, timeout time.Duration) (LLM, error) {
	// Get custom API key from secret provider
	apiKey, err := f.secretProvider.GetSecret("CUSTOM_AI_KEY")
	if err != nil {
		return nil, fmt.Errorf("failed to get custom AI key: %w", err)
	}

	// For custom, we'll use OpenAI-style backend with custom base URL
	// In production, this would be configurable
	backend := llm.NewOpenAIBackend(llm.OpenAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.custom-ai.com/v1", // Placeholder
		Model:   model,
	})

	// Set timeout if provided
	if timeout > 0 {
		backend.Client.Timeout = timeout
	}

	return &llmAdapter{backend: backend, temperature: temperature}, nil
}

// llmAdapter adapts llm.Backend to ai.LLM interface.
type llmAdapter struct {
	backend     llm.Backend
	temperature float64
}

// Complete implements the LLM interface.
func (a *llmAdapter) Complete(prompt string) (string, error) {
	return a.backend.Generate(prompt)
}

// llmBackendAdapter adapts ai.LLM to llm.Backend interface.
type llmBackendAdapter struct {
	llm LLM
}

// Generate implements the llm.Backend interface.
func (a *llmBackendAdapter) Generate(prompt string) (string, error) {
	return a.llm.Complete(prompt)
}

// entityExtractorAdapter adapts llm.Extractor to ai.EntityExtractor interface.
type entityExtractorAdapter struct {
	extractor *llm.Extractor
}

// Extract implements the EntityExtractor interface.
func (a *entityExtractorAdapter) Extract(text string) (*llm.EntityGraph, error) {
	return a.extractor.Extract(text)
}

