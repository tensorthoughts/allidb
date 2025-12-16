package ai

import (
	"testing"

	"github.com/tensorthoughts25/allidb/core/config"
	"github.com/tensorthoughts25/allidb/core/security/secrets"
)

func TestNewFactory(t *testing.T) {
	aiConfig := config.AIConfig{
		Embeddings: config.EmbeddingsConfig{
			Provider:  "openai",
			Model:     "text-embedding-3-large",
			Dim:       3072,
			TimeoutMs: 3000,
		},
		LLM: config.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.0,
			MaxTokens:   2048,
			TimeoutMs:   5000,
		},
		EntityExtraction: config.EntityExtractionConfig{
			Enabled:   true,
			MaxRetries: 2,
		},
	}

	secretProvider, err := secrets.NewEnvProvider([]string{})
	if err != nil {
		t.Fatalf("Failed to create secret provider: %v", err)
	}

	factory := NewFactory(aiConfig, secretProvider)
	if factory == nil {
		t.Fatal("NewFactory returned nil")
	}
}

func TestFactory_CreateEmbedder_UnsupportedProvider(t *testing.T) {
	aiConfig := config.AIConfig{
		Embeddings: config.EmbeddingsConfig{
			Provider: "unsupported",
			Model:    "test-model",
			Dim:      1536,
		},
	}

	secretProvider, _ := secrets.NewEnvProvider([]string{})
	factory := NewFactory(aiConfig, secretProvider)

	_, err := factory.CreateEmbedder()
	if err == nil {
		t.Fatal("Expected error for unsupported provider")
	}
}

func TestFactory_CreateLLM_UnsupportedProvider(t *testing.T) {
	aiConfig := config.AIConfig{
		LLM: config.LLMConfig{
			Provider: "unsupported",
			Model:    "test-model",
		},
	}

	secretProvider, _ := secrets.NewEnvProvider([]string{})
	factory := NewFactory(aiConfig, secretProvider)

	_, err := factory.CreateLLM()
	if err == nil {
		t.Fatal("Expected error for unsupported provider")
	}
}

func TestFactory_CreateEntityExtractor_NotEnabled(t *testing.T) {
	aiConfig := config.AIConfig{
		EntityExtraction: config.EntityExtractionConfig{
			Enabled: false,
		},
	}

	secretProvider, _ := secrets.NewEnvProvider([]string{})
	factory := NewFactory(aiConfig, secretProvider)

	_, err := factory.CreateEntityExtractor()
	if err == nil {
		t.Fatal("Expected error when entity extraction is not enabled")
	}
}

func TestFactory_CreateEmbedder_OpenAI_MissingSecret(t *testing.T) {
	aiConfig := config.AIConfig{
		Embeddings: config.EmbeddingsConfig{
			Provider:  "openai",
			Model:     "text-embedding-3-large",
			Dim:       3072,
			TimeoutMs: 3000,
		},
	}

	secretProvider, _ := secrets.NewEnvProvider([]string{})
	factory := NewFactory(aiConfig, secretProvider)

	_, err := factory.CreateEmbedder()
	if err == nil {
		t.Fatal("Expected error when API key is missing")
	}
}

func TestFactory_CreateLLM_OpenAI_MissingSecret(t *testing.T) {
	aiConfig := config.AIConfig{
		LLM: config.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.0,
			TimeoutMs:   5000,
		},
	}

	secretProvider, _ := secrets.NewEnvProvider([]string{})
	factory := NewFactory(aiConfig, secretProvider)

	_, err := factory.CreateLLM()
	if err == nil {
		t.Fatal("Expected error when API key is missing")
	}
}

func TestFactory_CreateEntityExtractor_RequiresLLM(t *testing.T) {
	aiConfig := config.AIConfig{
		LLM: config.LLMConfig{
			Provider: "unsupported", // This will fail
			Model:    "test-model",
		},
		EntityExtraction: config.EntityExtractionConfig{
			Enabled:   true,
			MaxRetries: 2,
		},
	}

	secretProvider, _ := secrets.NewEnvProvider([]string{})
	factory := NewFactory(aiConfig, secretProvider)

	_, err := factory.CreateEntityExtractor()
	if err == nil {
		t.Fatal("Expected error when LLM creation fails")
	}
}

