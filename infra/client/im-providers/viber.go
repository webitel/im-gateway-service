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

type ViberClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.ViberServiceClient]
}

func NewViberClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*ViberClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.ViberServiceClient {
		return providerv1.NewViberServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-viber-client] initialization failed: %w", err)
	}

	return &ViberClient{logger: logger, rpc: c}, nil
}

func (c *ViberClient) CreateViberGate(ctx context.Context, in *providerv1.ProviderCreateViberGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderCreateViberGateResponse, error) {
	var resp *providerv1.ProviderCreateViberGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberServiceClient) error {
		var err error
		resp, err = api.CreateViberGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberClient) GetViberGate(ctx context.Context, in *providerv1.ProviderGetViberGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetViberGateResponse, error) {
	var resp *providerv1.ProviderGetViberGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberServiceClient) error {
		var err error
		resp, err = api.GetViberGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberClient) UpdateViberGate(ctx context.Context, in *providerv1.ProviderUpdateViberGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateViberGateResponse, error) {
	var resp *providerv1.ProviderUpdateViberGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberServiceClient) error {
		var err error
		resp, err = api.UpdateViberGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberClient) DeleteViberGate(ctx context.Context, in *providerv1.ProviderDeleteViberGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteViberGateResponse, error) {
	var resp *providerv1.ProviderDeleteViberGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberServiceClient) error {
		var err error
		resp, err = api.DeleteViberGate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
