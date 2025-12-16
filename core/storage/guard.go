package storage

import (
	"context"
	"fmt"

	"github.com/tensorthoughts25/allidb/core/security/tenant"
)

// TenantGuard enforces tenant isolation at the storage layer.
type TenantGuard struct {
	// Whether to fail hard (panic) on violations
	failHard bool
}

// NewTenantGuard creates a new tenant guard.
func NewTenantGuard(failHard bool) *TenantGuard {
	return &TenantGuard{
		failHard: failHard,
	}
}

// RequireTenant requires a tenant in the context.
// If failHard is true, panics on violation. Otherwise returns an error.
func (g *TenantGuard) RequireTenant(ctx context.Context) (string, error) {
	tenantID, err := tenant.RequireTenant(ctx)
	if err != nil {
		if g.failHard {
			panic(fmt.Sprintf("TENANT ISOLATION VIOLATION: %v", err))
		}
		return "", fmt.Errorf("tenant isolation violation: %w", err)
	}
	return tenantID, nil
}

// ValidateTenant validates that the context tenant matches the expected tenant.
// Fails hard if tenant mismatch is detected.
func (g *TenantGuard) ValidateTenant(ctx context.Context, expectedTenantID string) error {
	actualTenantID, err := g.RequireTenant(ctx)
	if err != nil {
		return err
	}

	if actualTenantID != expectedTenantID {
		violation := fmt.Sprintf("tenant isolation violation: expected tenant %s, got %s", expectedTenantID, actualTenantID)
		if g.failHard {
			panic("TENANT ISOLATION VIOLATION: " + violation)
		}
		return fmt.Errorf("TENANT ISOLATION VIOLATION: %s", violation)
	}

	return nil
}

// ValidateEntityTenant validates that an entity belongs to the tenant in context.
func (g *TenantGuard) ValidateEntityTenant(ctx context.Context, entityTenantID string) error {
	contextTenantID, err := g.RequireTenant(ctx)
	if err != nil {
		return err
	}

	if entityTenantID != contextTenantID {
		violation := fmt.Sprintf("tenant isolation violation: entity belongs to tenant %s, but context has tenant %s", entityTenantID, contextTenantID)
		if g.failHard {
			panic("TENANT ISOLATION VIOLATION: " + violation)
		}
		return fmt.Errorf("TENANT ISOLATION VIOLATION: %s", violation)
	}

	return nil
}

// MustRequireTenant requires a tenant and panics if not found.
func (g *TenantGuard) MustRequireTenant(ctx context.Context) string {
	tenantID, err := g.RequireTenant(ctx)
	if err != nil {
		panic(fmt.Sprintf("TENANT ISOLATION VIOLATION: %v", err))
	}
	return tenantID
}

// DefaultTenantGuard is a default tenant guard that fails hard.
var DefaultTenantGuard = NewTenantGuard(true)

// RequireTenant is a convenience function using the default guard.
func RequireTenant(ctx context.Context) (string, error) {
	return DefaultTenantGuard.RequireTenant(ctx)
}

// ValidateTenant is a convenience function using the default guard.
func ValidateTenant(ctx context.Context, expectedTenantID string) error {
	return DefaultTenantGuard.ValidateTenant(ctx, expectedTenantID)
}

// ValidateEntityTenant is a convenience function using the default guard.
func ValidateEntityTenant(ctx context.Context, entityTenantID string) error {
	return DefaultTenantGuard.ValidateEntityTenant(ctx, entityTenantID)
}

// MustRequireTenant is a convenience function using the default guard.
func MustRequireTenant(ctx context.Context) string {
	return DefaultTenantGuard.MustRequireTenant(ctx)
}

