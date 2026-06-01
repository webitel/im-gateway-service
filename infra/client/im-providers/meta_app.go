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

type MetaAppClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[providerv1.MetaAppServiceClient]
}

func NewMetaAppClient(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*MetaAppClient, error) {
	factory := func(conn *grpc.ClientConn) providerv1.MetaAppServiceClient {
		return providerv1.NewMetaAppServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-providers-meta-app-client] initialization failed: %w", err)
	}

	return &MetaAppClient{logger: logger, rpc: c}, nil
}

func (c *MetaAppClient) CreateMetaApp(ctx context.Context, in *providerv1.ProviderCreateMetaAppRequest, opts ...grpc.CallOption) (*providerv1.ProviderCreateMetaAppResponse, error) {
	var resp *providerv1.ProviderCreateMetaAppResponse

	err := c.rpc.Execute(ctx, func(api providerv1.MetaAppServiceClient) error {
		var err error
		resp, err = api.CreateMetaApp(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *MetaAppClient) GetMetaApp(ctx context.Context, in *providerv1.ProviderGetMetaAppRequest, opts ...grpc.CallOption) (*providerv1.ProviderGetMetaAppResponse, error) {
	var resp *providerv1.ProviderGetMetaAppResponse

	err := c.rpc.Execute(ctx, func(api providerv1.MetaAppServiceClient) error {
		var err error
		resp, err = api.GetMetaApp(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *MetaAppClient) UpdateMetaApp(ctx context.Context, in *providerv1.ProviderUpdateMetaAppRequest, opts ...grpc.CallOption) (*providerv1.ProviderUpdateMetaAppResponse, error) {
	var resp *providerv1.ProviderUpdateMetaAppResponse

	err := c.rpc.Execute(ctx, func(api providerv1.MetaAppServiceClient) error {
		var err error
		resp, err = api.UpdateMetaApp(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *MetaAppClient) DeleteMetaApp(ctx context.Context, in *providerv1.ProviderDeleteMetaAppRequest, opts ...grpc.CallOption) (*providerv1.ProviderDeleteMetaAppResponse, error) {
	var resp *providerv1.ProviderDeleteMetaAppResponse

	err := c.rpc.Execute(ctx, func(api providerv1.MetaAppServiceClient) error {
		var err error
		resp, err = api.DeleteMetaApp(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *MetaAppClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
