package improviders

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
)

type CustomClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.CustomServiceClient]
}

func NewCustomClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*CustomClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.CustomServiceClient {
		return providerv1.NewCustomServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-custom-client] initialization failed: %w", err)
	}

	return &CustomClient{logger: logger, rpc: c}, nil
}

func (c *CustomClient) CreateCustomGate(ctx context.Context, in *providerv1.ProviderCreateCustomGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderCreateCustomGateResponse, error) {
	var resp *providerv1.ProviderCreateCustomGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.CustomServiceClient) error {
		var err error
		resp, err = api.CreateCustomGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *CustomClient) GetCustomGate(ctx context.Context, in *providerv1.ProviderGetCustomGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetCustomGateResponse, error) {
	var resp *providerv1.ProviderGetCustomGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.CustomServiceClient) error {
		var err error
		resp, err = api.GetCustomGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *CustomClient) UpdateCustomGate(ctx context.Context, in *providerv1.ProviderUpdateCustomGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateCustomGateResponse, error) {
	var resp *providerv1.ProviderUpdateCustomGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.CustomServiceClient) error {
		var err error
		resp, err = api.UpdateCustomGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *CustomClient) DeleteCustomGate(ctx context.Context, in *providerv1.ProviderDeleteCustomGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteCustomGateResponse, error) {
	var resp *providerv1.ProviderDeleteCustomGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.CustomServiceClient) error {
		var err error
		resp, err = api.DeleteCustomGate(ctx, in, opts...)
		return err
	})
	return resp, err
}
