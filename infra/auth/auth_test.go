package auth

import (
	"context"
	"testing"
)

func TestIsSystemCall(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "schema call (call_center/flow_manager) is a system call",
			ctx:  WithAuthType(context.Background(), XWebitelTypeSchema),
			want: true,
		},
		{
			name: "engine call is a system call",
			ctx:  WithAuthType(context.Background(), XWebitelTypeEngine),
			want: true,
		},
		{
			name: "plain context (regular user) is not a system call",
			ctx:  context.Background(),
			want: false,
		},
		{
			name: "provider service call is not a system call",
			ctx:  WithAuthType(context.Background(), XWebitelTypeProvider),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSystemCall(tt.ctx); got != tt.want {
				t.Errorf("IsSystemCall() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthTypeFromContext(t *testing.T) {
	t.Run("absent on plain context", func(t *testing.T) {
		if _, ok := AuthTypeFromContext(context.Background()); ok {
			t.Fatal("expected no auth type on a plain context")
		}
	})

	t.Run("round-trips the stored type", func(t *testing.T) {
		ctx := WithAuthType(context.Background(), XWebitelTypeEngine)
		got, ok := AuthTypeFromContext(ctx)
		if !ok {
			t.Fatal("expected auth type to be present")
		}
		if got != XWebitelTypeEngine {
			t.Errorf("AuthTypeFromContext() = %q, want %q", got, XWebitelTypeEngine)
		}
	})
}

type permissionTestIdentity struct{ permissions []string }

func (i permissionTestIdentity) GetContactID() string     { return "" }
func (i permissionTestIdentity) GetDomainID() int64       { return 1 }
func (i permissionTestIdentity) GetIssuer() string        { return "" }
func (i permissionTestIdentity) GetName() string          { return "" }
func (i permissionTestIdentity) GetChatName() string      { return "" }
func (i permissionTestIdentity) GetApplicationID() string { return "" }
func (i permissionTestIdentity) GetVia() string           { return "" }
func (i permissionTestIdentity) GetViaPtr() *string       { return nil }
func (i permissionTestIdentity) GetViaName() string       { return "" }
func (i permissionTestIdentity) GetViaType() string       { return "" }
func (i permissionTestIdentity) HasPermission(id string) bool {
	for _, p := range i.permissions {
		if p == id {
			return true
		}
	}
	return false
}

func TestHasPermission(t *testing.T) {
	withIdentity := func(permissions ...string) context.Context {
		return context.WithValue(context.Background(), AuthContextKey, Identifier(permissionTestIdentity{permissions: permissions}))
	}

	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "no identity in context",
			ctx:  context.Background(),
			want: false,
		},
		{
			name: "identity without the permission",
			ctx:  withIdentity("read"),
			want: false,
		},
		{
			name: "identity granted the permission",
			ctx:  withIdentity("read", PermissionIMReadAll),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.ctx, PermissionIMReadAll); got != tt.want {
				t.Errorf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
