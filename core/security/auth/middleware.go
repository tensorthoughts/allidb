package auth

import (
	"context"
	"net/http"

	"github.com/tensorthoughts25/allidb/core/observability"
)

// ContextKey is a type for context keys.
type ContextKey string

// logger is a package-level logger for auth middleware.
var logger = observability.DefaultLogger().WithField("component", "auth_middleware")

const (
	// TenantIDKey is the context key for tenant ID.
	TenantIDKey ContextKey = "tenant_id"
	// APIKeyHeader is the HTTP header name for API key.
	APIKeyHeader = "X-ALLIDB-API-KEY"
)


// GetTenantID retrieves the tenant ID from context.
func GetTenantID(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(TenantIDKey).(string)
	return tenantID, ok
}

// WithTenantID adds tenant ID to context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// HTTPAuthMiddleware creates HTTP middleware for API key authentication.
func HTTPAuthMiddleware(store *APIKeyStore) func(http.Handler) http.Handler {
	return HTTPAuthMiddlewareWithHeader(store, APIKeyHeader)
}

// HTTPAuthMiddlewareWithHeader creates HTTP middleware for API key authentication
// using the provided header name. If header is empty, APIKeyHeader is used.
func HTTPAuthMiddlewareWithHeader(store *APIKeyStore, header string) func(http.Handler) http.Handler {
	if header == "" {
		header = APIKeyHeader
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from header
			apiKey := r.Header.Get(header)
			if apiKey == "" {
				logger.Warn("Missing API key", observability.Fields{
					"header": header,
					"path":   r.URL.Path,
					"method": r.Method,
					"remote": r.RemoteAddr,
				})
				http.Error(w, "Missing API key", http.StatusUnauthorized)
				return
			}

			// Get tenant ID from store
			tenantID, ok := store.GetTenantID(apiKey)
			if !ok {
				logger.Warn("Invalid API key", observability.Fields{
					"header": header,
					"path":   r.URL.Path,
					"method": r.Method,
					"remote": r.RemoteAddr,
				})
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Inject tenant ID into request context
			ctx := WithTenantID(r.Context(), tenantID)
			logger.Info("Authenticated request==========", observability.Fields{
				"tenant_id": tenantID,
				"path":      r.URL.Path,
				"method":    r.Method,
				"remote":    r.RemoteAddr,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}


