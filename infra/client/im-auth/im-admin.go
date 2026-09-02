package imauth

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
)

type AdminClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[adminv1.ApplicationsClient]
}

// SearchApps implements the Applications service SearchApps method.
func (c *AdminClient) SearchApps(ctx context.Context, in *adminv1.SearchAppRequest, opts ...grpc.CallOption) (*adminv1.ApplicationList, error) {
	var resp *adminv1.ApplicationList

	err := c.rpc.Execute(ctx, func(api adminv1.ApplicationsClient) error {
		var err error

		resp, err = api.SearchApps(ctx, in, opts...)

		return err
	})

	return resp, err
}

// NewAdmin initializes a resilient gRPC client for the Admin service.
func NewAdmin(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*AdminClient, error) {
	factory := func(conn *grpc.ClientConn) adminv1.ApplicationsClient {
		return adminv1.NewApplicationsClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-admin-client] initialization failed: %w", err)
	}

	return &AdminClient{
		logger: logger,
		rpc:    c,
	}, nil
}

func (c *AdminClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}
