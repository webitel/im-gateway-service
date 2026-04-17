package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var _ impb.ThreadPermissionServer = (*ThreadPermissionServer)(nil)

type ThreadPermissionServer struct {
	impb.UnimplementedThreadPermissionServer
	log *slog.Logger

	permissioner service.ThreadPermissioner
}

func NewThreadPermissionServer(log *slog.Logger, permissioner service.ThreadPermissioner) *ThreadPermissionServer {
	return &ThreadPermissionServer{
		log:          log,
		permissioner: permissioner,
	}
}

// Get implements [api.ThreadPermissionServer].
func (t *ThreadPermissionServer) Get(ctx context.Context, req *impb.GetThreadPermissionsRequest) (*impb.GetThreadPermissionsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.permissioner.Get(ctx, req)
}

// Update implements [api.ThreadPermissionServer].
func (t *ThreadPermissionServer) Update(ctx context.Context, req *impb.UpdateThreadPermissionsRequest) (*impb.UpdateThreadPermissionsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.permissioner.Update(ctx, req)
}
