package auth

import (
	"crypto/subtle"
	"sync"
)

// APIKeyStore stores API keys and their associated tenant IDs.
type APIKeyStore struct {
	mu sync.RWMutex
	// Map from API key to tenant ID
	keys map[string]string
}

// NewAPIKeyStore creates a new API key store.
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys: make(map[string]string),
	}
}

// AddKey adds an API key with its associated tenant ID.
func (s *APIKeyStore) AddKey(apiKey, tenantID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[apiKey] = tenantID
}

// RemoveKey removes an API key.
func (s *APIKeyStore) RemoveKey(apiKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, apiKey)
}

// GetTenantID retrieves the tenant ID for an API key using constant-time comparison.
func (s *APIKeyStore) GetTenantID(apiKey string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use constant-time comparison to prevent timing attacks
	for storedKey, tenantID := range s.keys {
		if constantTimeCompare(apiKey, storedKey) {
			return tenantID, true
		}
	}

	return "", false
}

// constantTimeCompare performs a constant-time comparison of two strings.
// This prevents timing attacks that could reveal information about valid API keys.
func constantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	// Convert to byte slices for constant-time comparison
	aBytes := []byte(a)
	bBytes := []byte(b)

	// Use crypto/subtle for constant-time comparison
	return subtle.ConstantTimeCompare(aBytes, bBytes) == 1
}

// HasKey checks if an API key exists.
func (s *APIKeyStore) HasKey(apiKey string) bool {
	_, exists := s.GetTenantID(apiKey)
	return exists
}

// GetAllKeys returns all API keys (for management purposes).
// Note: In production, this should be restricted or removed.
func (s *APIKeyStore) GetAllKeys() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string, len(s.keys))
	for key, tenantID := range s.keys {
		result[key] = tenantID
	}
	return result
}

// Count returns the number of API keys.
func (s *APIKeyStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.keys)
}

