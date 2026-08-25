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

type InstagramClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.InstagramServiceClient]
}

func NewInstagramClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*InstagramClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.InstagramServiceClient {
		return providerv1.NewInstagramServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-instagram-client] initialization failed: %w", err)
	}

	return &InstagramClient{logger: logger, rpc: c}, nil
}

func (c *InstagramClient) CreateInstagramGate(ctx context.Context, in *providerv1.ProviderCreateInstagramGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderCreateInstagramGateResponse, error) {
	var resp *providerv1.ProviderCreateInstagramGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.InstagramServiceClient) error {
		var err error
		resp, err = api.CreateInstagramGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *InstagramClient) GetInstagramGate(ctx context.Context, in *providerv1.ProviderGetInstagramGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetInstagramGateResponse, error) {
	var resp *providerv1.ProviderGetInstagramGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.InstagramServiceClient) error {
		var err error
		resp, err = api.GetInstagramGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *InstagramClient) UpdateInstagramGate(ctx context.Context, in *providerv1.ProviderUpdateInstagramGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateInstagramGateResponse, error) {
	var resp *providerv1.ProviderUpdateInstagramGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.InstagramServiceClient) error {
		var err error
		resp, err = api.UpdateInstagramGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *InstagramClient) DeleteInstagramGate(ctx context.Context, in *providerv1.ProviderDeleteInstagramGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteInstagramGateResponse, error) {
	var resp *providerv1.ProviderDeleteInstagramGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.InstagramServiceClient) error {
		var err error
		resp, err = api.DeleteInstagramGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *InstagramClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
