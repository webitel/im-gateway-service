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

type MetaOAuthClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.MetaOAuthServiceClient]
}

func NewMetaOAuthClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*MetaOAuthClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.MetaOAuthServiceClient {
		return providerv1.NewMetaOAuthServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-meta-oauth-client] initialization failed: %w", err)
	}

	return &MetaOAuthClient{logger: logger, rpc: c}, nil
}

func (c *MetaOAuthClient) StartMetaOAuth(ctx context.Context, in *providerv1.ProviderMetaOAuthStartRequest, opts ...grpc.CallOption) (*providerv1.ProviderMetaOAuthStartResponse, error) {
	var resp *providerv1.ProviderMetaOAuthStartResponse

	err := c.rpc.Execute(ctx, func(api providerv1.MetaOAuthServiceClient) error {
		var err error
		resp, err = api.StartMetaOAuth(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *MetaOAuthClient) MetaOAuthCallback(ctx context.Context, in *providerv1.ProviderMetaOAuthCallbackRequest, opts ...grpc.CallOption) (*providerv1.ProviderMetaOAuthCallbackResponse, error) {
	var resp *providerv1.ProviderMetaOAuthCallbackResponse

	err := c.rpc.Execute(ctx, func(api providerv1.MetaOAuthServiceClient) error {
		var err error
		resp, err = api.MetaOAuthCallback(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *MetaOAuthClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
