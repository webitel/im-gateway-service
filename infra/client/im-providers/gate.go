package improviders

import (
	"context"
	"fmt"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

type GateClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.GateServiceClient]
}

func NewGateClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*GateClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.GateServiceClient {
		return providerv1.NewGateServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-gate-client] initialization failed: %w", err)
	}

	return &GateClient{logger: logger, rpc: c}, nil
}

func (c *GateClient) ListGates(ctx context.Context, in *providerv1.ProviderListGatesRequest, opts ...grpc.CallOption) (*providerv1.ProviderListGatesResponse, error) {
	var resp *providerv1.ProviderListGatesResponse

	err := c.rpc.Execute(ctx, func(api providerv1.GateServiceClient) error {
		var err error
		resp, err = api.ListGates(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *GateClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
