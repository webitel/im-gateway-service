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
