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

const ServiceName = "im-providers-service"

type FacebookClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.FacebookServiceClient]
}

func NewFacebookClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*FacebookClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.FacebookServiceClient {
		return providerv1.NewFacebookServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-facebook-client] initialization failed: %w", err)
	}

	return &FacebookClient{logger: logger, rpc: c}, nil
}

func (c *FacebookClient) CreateFacebookGate(ctx context.Context, in *providerv1.ProviderCreateFacebookGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderCreateFacebookGateResponse, error) {
	var resp *providerv1.ProviderCreateFacebookGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.FacebookServiceClient) error {
		var err error
		resp, err = api.CreateFacebookGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *FacebookClient) GetFacebookGate(ctx context.Context, in *providerv1.ProviderGetFacebookGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetFacebookGateResponse, error) {
	var resp *providerv1.ProviderGetFacebookGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.FacebookServiceClient) error {
		var err error
		resp, err = api.GetFacebookGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *FacebookClient) UpdateFacebookGate(ctx context.Context, in *providerv1.ProviderUpdateFacebookGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateFacebookGateResponse, error) {
	var resp *providerv1.ProviderUpdateFacebookGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.FacebookServiceClient) error {
		var err error
		resp, err = api.UpdateFacebookGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *FacebookClient) DeleteFacebookGate(ctx context.Context, in *providerv1.ProviderDeleteFacebookGateRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteFacebookGateResponse, error) {
	var resp *providerv1.ProviderDeleteFacebookGateResponse

	err := c.rpc.Execute(ctx, func(api providerv1.FacebookServiceClient) error {
		var err error
		resp, err = api.DeleteFacebookGate(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *FacebookClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
