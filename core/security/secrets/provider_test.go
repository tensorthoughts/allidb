package secrets

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNewEnvProvider_NoRequiredSecrets(t *testing.T) {
	provider, err := NewEnvProvider([]string{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if provider == nil {
		t.Fatal("NewEnvProvider returned nil")
	}
}

func TestNewEnvProvider_RequiredSecretPresent(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key-123")
	defer os.Unsetenv("OPENAI_API_KEY")

	provider, err := NewEnvProvider([]string{"OPENAI_API_KEY"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if provider == nil {
		t.Fatal("NewEnvProvider returned nil")
	}

	// Should be able to get the secret
	value, err := provider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if value != "test-key-123" {
		t.Errorf("Expected 'test-key-123', got '%s'", value)
	}
}

func TestNewEnvProvider_RequiredSecretMissing(t *testing.T) {
	// Ensure the secret is not set
	os.Unsetenv("OPENAI_API_KEY")

	// Should return error on creation if required secret is missing (fail fast)
	provider, err := NewEnvProvider([]string{"OPENAI_API_KEY"})
	if err == nil {
		t.Fatal("Expected error when required secret is missing")
	}
	if provider != nil {
		t.Fatal("Expected nil provider when error occurs")
	}
	if !strings.Contains(err.Error(), "required secret") {
		t.Errorf("Expected error about required secret, got: %v", err)
	}
}

func TestEnvProvider_GetSecret_Supported(t *testing.T) {
	provider, err := NewEnvProvider([]string{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Test OPENAI_API_KEY
	os.Setenv("OPENAI_API_KEY", "openai-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	value, err := provider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if value != "openai-key" {
		t.Errorf("Expected 'openai-key', got '%s'", value)
	}

	// Test AZURE_OPENAI_KEY
	os.Setenv("AZURE_OPENAI_KEY", "azure-key")
	defer os.Unsetenv("AZURE_OPENAI_KEY")

	value2, err := provider.GetSecret("AZURE_OPENAI_KEY")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if value2 != "azure-key" {
		t.Errorf("Expected 'azure-key', got '%s'", value2)
	}

	// Test CUSTOM_AI_KEY
	os.Setenv("CUSTOM_AI_KEY", "custom-key")
	defer os.Unsetenv("CUSTOM_AI_KEY")

	value3, err := provider.GetSecret("CUSTOM_AI_KEY")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if value3 != "custom-key" {
		t.Errorf("Expected 'custom-key', got '%s'", value3)
	}
}

func TestEnvProvider_GetSecret_Unsupported(t *testing.T) {
	provider, err := NewEnvProvider([]string{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	_, err = provider.GetSecret("UNSUPPORTED_SECRET")
	if err == nil {
		t.Fatal("Expected error for unsupported secret name")
	}
	if err.Error() != "unsupported secret name: UNSUPPORTED_SECRET" {
		t.Errorf("Expected 'unsupported secret name' error, got: %v", err)
	}
}

func TestEnvProvider_GetSecret_NotFound(t *testing.T) {
	provider, err := NewEnvProvider([]string{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	os.Unsetenv("OPENAI_API_KEY")

	_, err = provider.GetSecret("OPENAI_API_KEY")
	if err == nil {
		t.Fatal("Expected error when secret not found")
	}
	if err.Error() != "secret OPENAI_API_KEY not found in environment" {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestEnvProvider_GetSecret_RequiredNotFound(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	provider, err := NewEnvProvider([]string{"OPENAI_API_KEY"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Try to get a required secret that was removed after creation
	os.Unsetenv("OPENAI_API_KEY")
	// This should still work from cache
	value, err := provider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Expected no error (from cache), got: %v", err)
	}
	if value != "test-key" {
		t.Errorf("Expected 'test-key', got '%s'", value)
	}
}

func TestEnvProvider_GetSecret_Caching(t *testing.T) {
	provider, err := NewEnvProvider([]string{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	os.Setenv("OPENAI_API_KEY", "original-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// First call should load from env
	value1, err := provider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Change the env var
	os.Setenv("OPENAI_API_KEY", "changed-key")

	// Second call should return cached value
	value2, err := provider.GetSecret("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if value1 != value2 {
		t.Errorf("Expected cached value, but got different values: %s vs %s", value1, value2)
	}
	if value1 != "original-key" {
		t.Errorf("Expected 'original-key', got '%s'", value1)
	}
}

func TestGetSecretName(t *testing.T) {
	tests := []struct {
		secretType string
		expected   string
	}{
		{"openai", "OPENAI_API_KEY"},
		{"azure_openai", "AZURE_OPENAI_KEY"},
		{"custom_ai", "CUSTOM_AI_KEY"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := GetSecretName(tt.secretType)
		if result != tt.expected {
			t.Errorf("GetSecretName(%s) = %s, expected %s", tt.secretType, result, tt.expected)
		}
	}
}

func TestRedactSecret(t *testing.T) {
	tests := []struct {
		secret   string
		expected string
	}{
		{"", "[EMPTY]"},
		{"short", "[REDACTED]"},
		{"12345678", "[REDACTED]"},
		{"123456789", "1234...6789"},
		{"abcdefghijklmnop", "abcd...mnop"},
		{"very-long-secret-key-12345", "very...2345"},
	}

	for _, tt := range tests {
		result := RedactSecret(tt.secret)
		if result != tt.expected {
			t.Errorf("RedactSecret(%s) = %s, expected %s", tt.secret, result, tt.expected)
		}
		// Ensure the actual secret is not in the redacted version
		if len(tt.secret) > 8 && result == tt.secret {
			t.Errorf("RedactSecret(%s) should not return the full secret", tt.secret)
		}
	}
}

func TestSafeError(t *testing.T) {
	err := SafeError("GetSecret", "OPENAI_API_KEY", fmt.Errorf("not found"))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	errMsg := err.Error()
	// Should not contain any secret values
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
	// Should contain the operation and secret name
	if !strings.Contains(errMsg, "GetSecret") {
		t.Error("Error message should contain operation name")
	}
	if !strings.Contains(errMsg, "OPENAI_API_KEY") {
		t.Error("Error message should contain secret name")
	}
}

