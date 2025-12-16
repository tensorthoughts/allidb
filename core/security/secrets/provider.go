package secrets

import (
	"fmt"
	"os"
	"sync"
)

// Provider is an interface for secret providers.
// This abstraction allows for different secret storage backends
// (environment variables, vault, AWS Secrets Manager, etc.)
type Provider interface {
	// GetSecret retrieves a secret by name.
	// Returns an error if the secret is not found or cannot be retrieved.
	GetSecret(name string) (string, error)
}

// EnvProvider loads secrets from environment variables.
type EnvProvider struct {
	mu sync.RWMutex
	// Cache of loaded secrets to avoid repeated env lookups
	cache map[string]string
	// Set of required secrets that must be present
	required map[string]bool
}

// NewEnvProvider creates a new environment variable-based secret provider.
// requiredSecrets is a list of secret names that must be present.
// If a required secret is missing, returns an error (fail fast).
func NewEnvProvider(requiredSecrets []string) (*EnvProvider, error) {
	required := make(map[string]bool)
	for _, name := range requiredSecrets {
		// Validate secret name is supported
		if !isSupportedSecret(name) {
			return nil, fmt.Errorf("unsupported required secret name: %s", name)
		}
		required[name] = true
	}

	provider := &EnvProvider{
		cache:    make(map[string]string),
		required: required,
	}

	// Pre-validate required secrets on creation (fail fast)
	for name := range required {
		value := os.Getenv(name)
		if value == "" {
			return nil, fmt.Errorf("required secret %s is missing from environment", name)
		}
		provider.cache[name] = value
	}

	return provider, nil
}

// GetSecret retrieves a secret from environment variables.
// Returns an error if:
// - The secret is required but not found
// - The secret name is unknown/unsupported
func (p *EnvProvider) GetSecret(name string) (string, error) {
	// Validate secret name is supported
	if !isSupportedSecret(name) {
		return "", fmt.Errorf("unsupported secret name: %s", name)
	}

	p.mu.RLock()
	// Check cache first
	if value, ok := p.cache[name]; ok {
		p.mu.RUnlock()
		return value, nil
	}
	p.mu.RUnlock()

	// Load from environment
	value := os.Getenv(name)

	// If required and missing, return error
	p.mu.RLock()
	isRequired := p.required[name]
	p.mu.RUnlock()

	if value == "" {
		if isRequired {
			return "", fmt.Errorf("required secret %s is missing from environment", name)
		}
		return "", fmt.Errorf("secret %s not found in environment", name)
	}

	// Cache the value
	p.mu.Lock()
	p.cache[name] = value
	p.mu.Unlock()

	return value, nil
}

// isSupportedSecret checks if a secret name is supported.
// Currently supports:
// - OPENAI_API_KEY
// - AZURE_OPENAI_KEY
// - CUSTOM_AI_KEY
func isSupportedSecret(name string) bool {
	supported := map[string]bool{
		"OPENAI_API_KEY":   true,
		"AZURE_OPENAI_KEY": true,
		"CUSTOM_AI_KEY":    true,
	}
	return supported[name]
}

// GetSecretName returns the environment variable name for a given secret type.
// This is a helper function for common secret types.
func GetSecretName(secretType string) string {
	secretMap := map[string]string{
		"openai":      "OPENAI_API_KEY",
		"azure_openai": "AZURE_OPENAI_KEY",
		"custom_ai":   "CUSTOM_AI_KEY",
	}
	if name, ok := secretMap[secretType]; ok {
		return name
	}
	return ""
}

// RedactSecret returns a redacted version of a secret for logging.
// Never log actual secret values.
func RedactSecret(secret string) string {
	if secret == "" {
		return "[EMPTY]"
	}
	if len(secret) <= 8 {
		return "[REDACTED]"
	}
	// Show first 4 and last 4 characters, redact the middle
	return secret[:4] + "..." + secret[len(secret)-4:]
}

// SafeError creates an error message that doesn't expose secret values.
// Use this when logging errors related to secrets.
func SafeError(operation, secretName string, err error) error {
	return fmt.Errorf("%s failed for secret %s: %v", operation, secretName, err)
}

