package tenant

import (
	"context"
	"errors"

	"github.com/tensorthoughts25/allidb/core/observability"
	"github.com/tensorthoughts25/allidb/core/security/auth"
)

var (
	// ErrNoTenant is returned when tenant is not found in context.
	ErrNoTenant = errors.New("tenant not found in context")
)
var logger = observability.DefaultLogger().WithField("component", "tenant_context")

// TenantFromContext retrieves the tenant ID from context.
// Returns the tenant ID and true if found, empty string and false otherwise.
func TenantFromContext(ctx context.Context) (string, bool) {
	tenantID, ok := auth.GetTenantID(ctx)
	if !ok || tenantID == "" {
		return "", false
	}
	return tenantID, true
}



// WithTenant adds tenant ID to context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return auth.WithTenantID(ctx, tenantID)
}

// RequireTenant retrieves the tenant ID from context or returns an error if not found.
// Use this for operations that must validate tenant but can return an error instead of panicking.
func RequireTenant(ctx context.Context) (string, error) {
	tenantID, ok := TenantFromContext(ctx)
	if !ok || tenantID == "" {
		logger.Warn("Tenant required but not found in context", observability.Fields{})
		return "", ErrNoTenant
	}
	return tenantID, nil
}

// MustRequireTenant retrieves the tenant ID from context and panics if not found.
// Use this for operations that must never proceed without a tenant.
func MustRequireTenant(ctx context.Context) string {
	tenantID, err := RequireTenant(ctx)
	if err != nil {
		panic("tenant required but not found in context: " + err.Error())
	}
	return tenantID
}

