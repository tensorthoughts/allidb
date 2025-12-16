package auth

import (
	"testing"
)

func TestNewAPIKeyStore(t *testing.T) {
	store := NewAPIKeyStore()

	if store == nil {
		t.Fatal("NewAPIKeyStore returned nil")
	}
	if store.Count() != 0 {
		t.Errorf("Expected 0 keys, got %d", store.Count())
	}
}

func TestAPIKeyStore_AddKey(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")
	store.AddKey("key2", "tenant2")

	if store.Count() != 2 {
		t.Errorf("Expected 2 keys, got %d", store.Count())
	}
}

func TestAPIKeyStore_GetTenantID(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")
	store.AddKey("key2", "tenant2")

	tenantID, ok := store.GetTenantID("key1")
	if !ok {
		t.Fatal("Expected to find tenant ID for key1")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}

	tenantID, ok = store.GetTenantID("key2")
	if !ok {
		t.Fatal("Expected to find tenant ID for key2")
	}
	if tenantID != "tenant2" {
		t.Errorf("Expected 'tenant2', got '%s'", tenantID)
	}
}

func TestAPIKeyStore_GetTenantID_NotFound(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")

	_, ok := store.GetTenantID("nonexistent")
	if ok {
		t.Error("Expected not to find tenant ID for nonexistent key")
	}
}

func TestAPIKeyStore_RemoveKey(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")
	store.AddKey("key2", "tenant2")

	store.RemoveKey("key1")

	if store.Count() != 1 {
		t.Errorf("Expected 1 key after removal, got %d", store.Count())
	}

	_, ok := store.GetTenantID("key1")
	if ok {
		t.Error("Key1 should be removed")
	}

	_, ok = store.GetTenantID("key2")
	if !ok {
		t.Error("Key2 should still exist")
	}
}

func TestAPIKeyStore_HasKey(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")

	if !store.HasKey("key1") {
		t.Error("Expected HasKey to return true for key1")
	}
	if store.HasKey("nonexistent") {
		t.Error("Expected HasKey to return false for nonexistent key")
	}
}

func TestAPIKeyStore_GetAllKeys(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")
	store.AddKey("key2", "tenant2")

	allKeys := store.GetAllKeys()

	if len(allKeys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(allKeys))
	}
	if allKeys["key1"] != "tenant1" {
		t.Errorf("Expected 'tenant1' for key1, got '%s'", allKeys["key1"])
	}
	if allKeys["key2"] != "tenant2" {
		t.Errorf("Expected 'tenant2' for key2, got '%s'", allKeys["key2"])
	}
}

func TestAPIKeyStore_ConstantTimeComparison(t *testing.T) {
	store := NewAPIKeyStore()

	// Add a key
	store.AddKey("correct-key", "tenant1")

	// Test that comparison is constant-time
	// This is hard to test directly, but we can verify it works correctly
	_, ok := store.GetTenantID("correct-key")
	if !ok {
		t.Error("Expected to find correct key")
	}

	_, ok = store.GetTenantID("wrong-key")
	if ok {
		t.Error("Expected not to find wrong key")
	}

	// Test with different length keys
	_, ok = store.GetTenantID("wrong-key-extra-long")
	if ok {
		t.Error("Expected not to find wrong key with different length")
	}
}

func TestAPIKeyStore_ConcurrentAccess(t *testing.T) {
	store := NewAPIKeyStore()

	// Concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "key" + string(rune(id))
			tenantID := "tenant" + string(rune(id))
			store.AddKey(key, tenantID)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if store.Count() != 10 {
		t.Errorf("Expected 10 keys, got %d", store.Count())
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "key" + string(rune(id))
			_, ok := store.GetTenantID(key)
			if !ok {
				t.Errorf("Expected to find key %s", key)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAPIKeyStore_UpdateKey(t *testing.T) {
	store := NewAPIKeyStore()

	store.AddKey("key1", "tenant1")
	store.AddKey("key1", "tenant2") // Update with same key

	tenantID, ok := store.GetTenantID("key1")
	if !ok {
		t.Fatal("Expected to find key1")
	}
	if tenantID != "tenant2" {
		t.Errorf("Expected 'tenant2' (updated), got '%s'", tenantID)
	}
}

func TestConstantTimeCompare(t *testing.T) {
	// Test equal strings
	if !constantTimeCompare("test", "test") {
		t.Error("Expected equal strings to compare as equal")
	}

	// Test different strings
	if constantTimeCompare("test", "wrong") {
		t.Error("Expected different strings to compare as different")
	}

	// Test different lengths
	if constantTimeCompare("test", "test-long") {
		t.Error("Expected different length strings to compare as different")
	}

	// Test empty strings
	if !constantTimeCompare("", "") {
		t.Error("Expected empty strings to compare as equal")
	}

	// Test one empty
	if constantTimeCompare("test", "") {
		t.Error("Expected non-empty and empty to compare as different")
	}
}

