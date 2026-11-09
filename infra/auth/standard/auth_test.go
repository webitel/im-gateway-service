package standard

import (
	"testing"

	interfaces "github.com/webitel/im-gateway-service/infra/auth"
)

func TestIdentity_HasPermission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{name: "no permissions", permissions: nil, want: false},
		{name: "other permissions", permissions: []string{"read", "write"}, want: false},
		{name: "granted", permissions: []string{"read", interfaces.PermissionIMReadAll}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := &Identity{Permissions: tt.permissions}
			if got := id.HasPermission(interfaces.PermissionIMReadAll); got != tt.want {
				t.Fatalf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
