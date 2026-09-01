package imthread

import (
	"context"
	"fmt"
	"log/slog"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

type ThreadTagClient struct {
	logger *slog.Logger
	// [GENERIC_RPC] Underlying go-kit RPC client using the generated ThreadTagManagementClient stub
	rpc *rpc.Client[threadv1.ThreadTagManagementClient]
	tls *infratls.Config
}

func NewThreadTagClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*ThreadTagClient, error) {
	factory := func(conn *grpc.ClientConn) threadv1.ThreadTagManagementClient {
		return threadv1.NewThreadTagManagementClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-thread-tag-client] initialization failed: %w", err)
	}

	return &ThreadTagClient{
		logger: logger,
		rpc:    c,
	}, nil
}

func (c *ThreadTagClient) AddTag(ctx context.Context, in *threadv1.AddThreadTagRequest, opts ...grpc.CallOption) (*threadv1.ChatTag, error) {
	var resp *threadv1.ChatTag
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadTagManagementClient) error {
		var err error
		resp, err = api.AddTag(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadTagClient) RemoveTag(ctx context.Context, in *threadv1.RemoveThreadTagRequest, opts ...grpc.CallOption) (*threadv1.RemoveThreadTagResponse, error) {
	var resp *threadv1.RemoveThreadTagResponse
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadTagManagementClient) error {
		var err error
		resp, err = api.RemoveTag(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadTagClient) Search(ctx context.Context, in *threadv1.SearchThreadTagsRequest, opts ...grpc.CallOption) (*threadv1.SearchThreadTagsResponse, error) {
	var resp *threadv1.SearchThreadTagsResponse
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadTagManagementClient) error {
		var err error
		resp, err = api.Search(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadTagClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}
