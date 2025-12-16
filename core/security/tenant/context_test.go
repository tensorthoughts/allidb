package tenant

import (
	"context"
	"testing"

	"github.com/tensorthoughts25/allidb/core/security/auth"
)

func TestTenantFromContext(t *testing.T) {
	ctx := context.Background()

	// No tenant in context
	_, ok := TenantFromContext(ctx)
	if ok {
		t.Error("Expected no tenant in empty context")
	}

	// With tenant
	ctx = WithTenant(ctx, "tenant1")
	tenantID, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("Expected tenant in context")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestRequireTenant(t *testing.T) {
	ctx := context.Background()

	// No tenant in context
	_, err := RequireTenant(ctx)
	if err == nil {
		t.Fatal("Expected error when tenant not in context")
	}
	if err != ErrNoTenant {
		t.Errorf("Expected ErrNoTenant, got %v", err)
	}

	// With tenant
	ctx = WithTenant(ctx, "tenant1")
	tenantID, err := RequireTenant(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestWithTenant(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenant(ctx, "tenant1")

	tenantID, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("Expected tenant in context")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestMustRequireTenant(t *testing.T) {
	ctx := context.Background()

	// No tenant - should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when tenant not in context")
			}
		}()
		_ = MustRequireTenant(ctx)
	}()

	// With tenant - should not panic
	ctx = WithTenant(ctx, "tenant1")
	tenantID := MustRequireTenant(ctx)
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestTenantFromContext_IntegrationWithAuth(t *testing.T) {
	// Test integration with auth package
	ctx := context.Background()
	ctx = auth.WithTenantID(ctx, "tenant1")

	tenantID, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("Expected tenant from auth package")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestRequireTenant_IntegrationWithAuth(t *testing.T) {
	// Test integration with auth package
	ctx := context.Background()
	ctx = auth.WithTenantID(ctx, "tenant1")

	tenantID, err := RequireTenant(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestTenantContext_Nested(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenant(ctx, "tenant1")

	// Nested context should preserve tenant
	ctx2 := context.WithValue(ctx, "other_key", "other_value")
	tenantID, ok := TenantFromContext(ctx2)
	if !ok {
		t.Fatal("Expected tenant in nested context")
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestTenantContext_Override(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenant(ctx, "tenant1")

	// Override tenant
	ctx = WithTenant(ctx, "tenant2")
	tenantID, ok := TenantFromContext(ctx)
	if !ok {
		t.Fatal("Expected tenant in context")
	}
	if tenantID != "tenant2" {
		t.Errorf("Expected 'tenant2', got '%s'", tenantID)
	}
}

