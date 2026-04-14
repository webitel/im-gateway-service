package imthread

import (
	"context"
	"fmt"
	"log/slog"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

// [GENERIC_INTERFACE_GUARD] Ensures Client matches the generated gRPC client interface.
var (
	_ threadv1.MessageClient = (*Client)(nil)
)

type ThreadPermissionClient struct {
	logger *slog.Logger
	// [GENERIC_RPC] Underlying go-kit RPC client using the generated MessageClient stub
	rpc *rpc.Client[threadv1.ThreadPermissionManagementClient]
	tls *infratls.Config
}

func NewThreadPermissionClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*ThreadPermissionClient, error) {
	factory := func(conn *grpc.ClientConn) threadv1.ThreadPermissionManagementClient{
		return threadv1.NewThreadPermissionManagementClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-thread-permission-client] initialization failed: %w", err)
	}

	return &ThreadPermissionClient{
		logger: logger,
		rpc:    c,
	}, nil
}

func (c *ThreadPermissionClient) GetThreadPermissions(ctx context.Context, in *threadv1.GetThreadPermissionsRequest, opts ...grpc.CallOption) (*threadv1.GetThreadPermissionsResponse, error) {
	var resp *threadv1.GetThreadPermissionsResponse
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadPermissionManagementClient) error {
		var err error
		resp, err = api.GetThreadPermissions(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadPermissionClient) UpdateThreadPermissions(ctx context.Context, in *threadv1.UpdateThreadPermissionsRequest, opts ...grpc.CallOption) (*threadv1.ThreadPermissions, error) {
	var resp *threadv1.ThreadPermissions
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadPermissionManagementClient) error {
		var err error
		resp, err = api.UpdateThreadPermissions(ctx, in, opts...)
		return err
	})

	return resp, err
}