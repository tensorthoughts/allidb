package storage

import (
	"context"
	"testing"

	"github.com/tensorthoughts25/allidb/core/security/tenant"
)

func TestNewTenantGuard(t *testing.T) {
	guard := NewTenantGuard(true)
	if guard == nil {
		t.Fatal("NewTenantGuard returned nil")
	}
	if !guard.failHard {
		t.Error("Expected failHard to be true")
	}

	guard2 := NewTenantGuard(false)
	if guard2.failHard {
		t.Error("Expected failHard to be false")
	}
}

func TestTenantGuard_RequireTenant_Success(t *testing.T) {
	guard := NewTenantGuard(false)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	tenantID, err := guard.RequireTenant(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestTenantGuard_RequireTenant_NoTenant_FailHard(t *testing.T) {
	guard := NewTenantGuard(true)
	ctx := context.Background()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when tenant not in context and failHard is true")
			} else {
				panicMsg := r.(string)
				if panicMsg == "" {
					t.Error("Expected non-empty panic message")
				}
			}
		}()
		_, _ = guard.RequireTenant(ctx)
	}()
}

func TestTenantGuard_RequireTenant_NoTenant_NoFailHard(t *testing.T) {
	guard := NewTenantGuard(false)
	ctx := context.Background()

	_, err := guard.RequireTenant(ctx)
	if err == nil {
		t.Fatal("Expected error when tenant not in context")
	}
}

func TestTenantGuard_ValidateTenant_Success(t *testing.T) {
	guard := NewTenantGuard(false)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	err := guard.ValidateTenant(ctx, "tenant1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestTenantGuard_ValidateTenant_Mismatch_FailHard(t *testing.T) {
	guard := NewTenantGuard(true)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on tenant mismatch with failHard")
			} else {
				panicMsg := r.(string)
				if panicMsg == "" {
					t.Error("Expected non-empty panic message")
				}
			}
		}()
		_ = guard.ValidateTenant(ctx, "tenant2")
	}()
}

func TestTenantGuard_ValidateTenant_Mismatch_NoFailHard(t *testing.T) {
	guard := NewTenantGuard(false)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	err := guard.ValidateTenant(ctx, "tenant2")
	if err == nil {
		t.Fatal("Expected error on tenant mismatch")
	}
}

func TestTenantGuard_ValidateEntityTenant_Success(t *testing.T) {
	guard := NewTenantGuard(false)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	err := guard.ValidateEntityTenant(ctx, "tenant1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestTenantGuard_ValidateEntityTenant_Mismatch_FailHard(t *testing.T) {
	guard := NewTenantGuard(true)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on entity tenant mismatch with failHard")
			}
		}()
		_ = guard.ValidateEntityTenant(ctx, "tenant2")
	}()
}

func TestTenantGuard_ValidateEntityTenant_Mismatch_NoFailHard(t *testing.T) {
	guard := NewTenantGuard(false)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	err := guard.ValidateEntityTenant(ctx, "tenant2")
	if err == nil {
		t.Fatal("Expected error on entity tenant mismatch")
	}
}

func TestTenantGuard_MustRequireTenant(t *testing.T) {
	guard := NewTenantGuard(true)
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	tenantID := guard.MustRequireTenant(ctx)
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestTenantGuard_MustRequireTenant_Panic(t *testing.T) {
	guard := NewTenantGuard(true)
	ctx := context.Background()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when tenant not in context")
			}
		}()
		_ = guard.MustRequireTenant(ctx)
	}()
}

func TestDefaultTenantGuard(t *testing.T) {
	if DefaultTenantGuard == nil {
		t.Fatal("DefaultTenantGuard should not be nil")
	}
	if !DefaultTenantGuard.failHard {
		t.Error("DefaultTenantGuard should fail hard")
	}
}

func TestRequireTenant_Convenience(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	tenantID, err := RequireTenant(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestRequireTenant_Convenience_NoTenant(t *testing.T) {
	ctx := context.Background()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when tenant not in context (default guard fails hard)")
			}
		}()
		_, _ = RequireTenant(ctx)
	}()
}

func TestValidateTenant_Convenience(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	err := ValidateTenant(ctx, "tenant1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestValidateTenant_Convenience_Mismatch(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on tenant mismatch (default guard fails hard)")
			}
		}()
		_ = ValidateTenant(ctx, "tenant2")
	}()
}

func TestValidateEntityTenant_Convenience(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	err := ValidateEntityTenant(ctx, "tenant1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestValidateEntityTenant_Convenience_Mismatch(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on entity tenant mismatch (default guard fails hard)")
			}
		}()
		_ = ValidateEntityTenant(ctx, "tenant2")
	}()
}

func TestMustRequireTenant_Convenience(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), "tenant1")

	tenantID := MustRequireTenant(ctx)
	if tenantID != "tenant1" {
		t.Errorf("Expected 'tenant1', got '%s'", tenantID)
	}
}

func TestMustRequireTenant_Convenience_Panic(t *testing.T) {
	ctx := context.Background()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when tenant not in context")
			}
		}()
		_ = MustRequireTenant(ctx)
	}()
}

func TestTenantGuard_ConcurrentAccess(t *testing.T) {
	guard := NewTenantGuard(false)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx := tenant.WithTenant(context.Background(), "tenant1")
			tenantID, err := guard.RequireTenant(ctx)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if tenantID != "tenant1" {
				t.Errorf("Expected 'tenant1', got '%s'", tenantID)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

