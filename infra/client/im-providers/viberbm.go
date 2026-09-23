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

type ViberBmClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.ViberBmServiceClient]
}

func NewViberBmClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*ViberBmClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.ViberBmServiceClient {
		return providerv1.NewViberBmServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-viber-bm-client] initialization failed: %w", err)
	}

	return &ViberBmClient{logger: logger, rpc: c}, nil
}

func (c *ViberBmClient) CreateViberBm(ctx context.Context, in *providerv1.ProviderCreateViberBmGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderCreateViberBmGateResponse, error) {
	var resp *providerv1.ProviderCreateViberBmGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberBmServiceClient) error {
		var err error
		resp, err = api.CreateViberBm(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberBmClient) GetViberBm(ctx context.Context, in *providerv1.ProviderGetViberBmGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetViberBmGateResponse, error) {
	var resp *providerv1.ProviderGetViberBmGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberBmServiceClient) error {
		var err error
		resp, err = api.GetViberBm(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberBmClient) UpdateViberBm(ctx context.Context, in *providerv1.ProviderUpdateViberBmGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateViberBmGateResponse, error) {
	var resp *providerv1.ProviderUpdateViberBmGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberBmServiceClient) error {
		var err error
		resp, err = api.UpdateViberBm(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberBmClient) DeleteViberBm(ctx context.Context, in *providerv1.ProviderDeleteViberBmGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteViberBmGateResponse, error) {
	var resp *providerv1.ProviderDeleteViberBmGateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberBmServiceClient) error {
		var err error
		resp, err = api.DeleteViberBm(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberBmClient) ListViberBm(ctx context.Context, in *providerv1.ProviderListViberBmGatesRequest, opts ...grpc.CallOption) (*providerv1.ProviderListViberBmGatesResponse, error) {
	var resp *providerv1.ProviderListViberBmGatesResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberBmServiceClient) error {
		var err error
		resp, err = api.ListViberBm(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberBmClient) SendViberBmTemplate(ctx context.Context, in *providerv1.SendViberBmTemplateRequest, opts ...grpc.CallOption) (*providerv1.SendViberBmTemplateResponse, error) {
	var resp *providerv1.SendViberBmTemplateResponse
	err := c.rpc.Execute(ctx, func(api providerv1.ViberBmServiceClient) error {
		var err error
		resp, err = api.SendViberBmTemplate(ctx, in, opts...)
		return err
	})
	return resp, err
}

func (c *ViberBmClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
