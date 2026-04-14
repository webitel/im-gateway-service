package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
)

// var _ impb.ThreadPermissionServer = (*ThreadPermissionServer)(nil)

type ThreadPermissionServer struct {
	log *slog.Logger
}

// Get implements [api.ThreadPermissionServer].
func (t *ThreadPermissionServer) Get(context.Context, *impb.GetThreadPermissionsRequest) (*impb.GetThreadPermissionsResponse, error) {
	panic("unimplemented")
}

// Update implements [api.ThreadPermissionServer].
func (t *ThreadPermissionServer) Update(context.Context, *impb.UpdateThreadPermissionsRequest) (*impb.ThreadPermissions, error) {
	panic("unimplemented")
}
