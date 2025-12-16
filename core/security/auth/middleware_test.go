package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTenantID(t *testing.T) {
	ctx := context.Background()

	// No tenant ID in context
	_, ok := GetTenantID(ctx)
	if ok {
		t.Error("Expected no tenant ID in empty context")
	}

	// With tenant ID
	ctx = WithTenantID(ctx, "tenant1")
	tenantID, ok := GetTenantID(ctx)
	if !ok {
		t.Fatal("Expected tenant ID in context")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestWithTenantID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tenant1")

	tenantID, ok := GetTenantID(ctx)
	if !ok {
		t.Fatal("Expected tenant ID")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestHTTPAuthMiddleware_ValidKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey("valid-key", "tenant1")

	middleware := HTTPAuthMiddleware(store)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := GetTenantID(r.Context())
		if !ok {
			t.Error("Expected tenant ID in context")
			return
		}
		if tenantID != "tenant1" {
			t.Errorf("Expected 'tenant1', got '%s'", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(APIKeyHeader, "valid-key")
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHTTPAuthMiddleware_MissingKey(t *testing.T) {
	store := NewAPIKeyStore()
	middleware := HTTPAuthMiddleware(store)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when key is missing")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No API key header
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestHTTPAuthMiddleware_InvalidKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey("valid-key", "tenant1")

	middleware := HTTPAuthMiddleware(store)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when key is invalid")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(APIKeyHeader, "invalid-key")
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestHTTPAuthMiddleware_EmptyKey(t *testing.T) {
	store := NewAPIKeyStore()
	middleware := HTTPAuthMiddleware(store)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when key is empty")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(APIKeyHeader, "")
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestHTTPAuthMiddleware_MultipleKeys(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey("key1", "tenant1")
	store.AddKey("key2", "tenant2")

	middleware := HTTPAuthMiddleware(store)

	// Test key1
	handler1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := GetTenantID(r.Context())
		if tenantID != "tenant1" {
			t.Errorf("Expected 'tenant1', got '%s'", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set(APIKeyHeader, "key1")
	rr1 := httptest.NewRecorder()
	middleware(handler1).ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr1.Code)
	}

	// Test key2
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := GetTenantID(r.Context())
		if tenantID != "tenant2" {
			t.Errorf("Expected 'tenant2', got '%s'", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	})

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set(APIKeyHeader, "key2")
	rr2 := httptest.NewRecorder()
	middleware(handler2).ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr2.Code)
	}
}

func TestHTTPAuthMiddleware_ContextPreservation(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey("valid-key", "tenant1")

	middleware := HTTPAuthMiddleware(store)

	// Add custom value to context
	type customKey string
	const customValueKey customKey = "custom"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check tenant ID
		tenantID, ok := GetTenantID(r.Context())
		if !ok {
			t.Error("Expected tenant ID")
			return
		}
		if tenantID != "tenant1" {
			t.Errorf("Expected 'tenant1', got '%s'", tenantID)
		}

		// Check custom value is preserved
		customValue := r.Context().Value(customValueKey)
		if customValue != "test-value" {
			t.Errorf("Expected custom value 'test-value', got '%v'", customValue)
		}

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(APIKeyHeader, "valid-key")
	ctx := context.WithValue(req.Context(), customValueKey, "test-value")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHTTPAuthMiddleware_ConcurrentRequests(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey("key1", "tenant1")
	store.AddKey("key2", "tenant2")

	middleware := HTTPAuthMiddleware(store)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := GetTenantID(r.Context())
		if !ok {
			t.Error("Expected tenant ID")
			return
		}
		_ = tenantID
		w.WriteHeader(http.StatusOK)
	})

	// Concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			req := httptest.NewRequest("GET", "/test", nil)
			if id%2 == 0 {
				req.Header.Set(APIKeyHeader, "key1")
			} else {
				req.Header.Set(APIKeyHeader, "key2")
			}
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHTTPAuthMiddleware_CaseSensitiveHeader(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey("valid-key", "tenant1")

	middleware := HTTPAuthMiddleware(store)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := GetTenantID(r.Context())
		if !ok {
			t.Error("Expected tenant ID")
			return
		}
		if tenantID != "tenant1" {
			t.Errorf("Expected 'tenant1', got '%s'", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Test with exact header name
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(APIKeyHeader, "valid-key")
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

