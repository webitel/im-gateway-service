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

type WhatsAppClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.WhatsAppServiceClient]
}

func NewWhatsAppClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*WhatsAppClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.WhatsAppServiceClient {
		return providerv1.NewWhatsAppServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-whatsapp-client] initialization failed: %w", err)
	}

	return &WhatsAppClient{logger: logger, rpc: c}, nil
}

func (c *WhatsAppClient) CreateWhatsAppGate(ctx context.Context, in *providerv1.CreateGateRequest, opts ...grpc.CallOption) (*providerv1.GateResponse, error) {
	var resp *providerv1.GateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.WhatsAppServiceClient) error {
		var err error
		resp, err = api.CreateWhatsAppGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *WhatsAppClient) GetWhatsAppGate(ctx context.Context, in *providerv1.ProviderGetWhatsAppGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetWhatsAppGateResponse, error) {
	var resp *providerv1.ProviderGetWhatsAppGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.WhatsAppServiceClient) error {
		var err error
		resp, err = api.GetWhatsAppGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *WhatsAppClient) UpdateWhatsAppGate(ctx context.Context, in *providerv1.ProviderUpdateWhatsAppGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateWhatsAppGateResponse, error) {
	var resp *providerv1.ProviderUpdateWhatsAppGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.WhatsAppServiceClient) error {
		var err error
		resp, err = api.UpdateWhatsAppGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *WhatsAppClient) DeleteWhatsAppGate(ctx context.Context, in *providerv1.ProviderDeleteWhatsAppGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteWhatsAppGateResponse, error) {
	var resp *providerv1.ProviderDeleteWhatsAppGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.WhatsAppServiceClient) error {
		var err error
		resp, err = api.DeleteWhatsAppGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *WhatsAppClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
