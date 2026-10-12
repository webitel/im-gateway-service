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

type ThreadPreviewClient struct {
	logger *slog.Logger
	// [GENERIC_RPC] Underlying go-kit RPC client using the generated ThreadPreviewServiceClient stub
	rpc *rpc.Client[threadv1.ThreadPreviewServiceClient]
	tls *infratls.Config
}

func NewThreadPreviewClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*ThreadPreviewClient, error) {
	factory := func(conn *grpc.ClientConn) threadv1.ThreadPreviewServiceClient {
		return threadv1.NewThreadPreviewServiceClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-thread-preview-client] initialization failed: %w", err)
	}

	return &ThreadPreviewClient{
		logger: logger,
		rpc:    c,
	}, nil
}

func (c *ThreadPreviewClient) GivePreview(ctx context.Context, in *threadv1.GivePreviewRequest, opts ...grpc.CallOption) (*threadv1.GivePreviewResponse, error) {
	var resp *threadv1.GivePreviewResponse
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadPreviewServiceClient) error {
		var err error
		resp, err = api.GivePreview(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadPreviewClient) RemoveFromPreview(ctx context.Context, in *threadv1.RemoveFromPreviewRequest, opts ...grpc.CallOption) (*threadv1.RemoveFromPreviewResponse, error) {
	var resp *threadv1.RemoveFromPreviewResponse
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadPreviewServiceClient) error {
		var err error
		resp, err = api.RemoveFromPreview(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadPreviewClient) UpgradePreviewToFullMember(ctx context.Context, in *threadv1.UpgradePreviewToFullMemberRequest, opts ...grpc.CallOption) (*threadv1.UpgradePreviewToFullMemberResponse, error) {
	var resp *threadv1.UpgradePreviewToFullMemberResponse
	err := c.rpc.Execute(ctx, func(api threadv1.ThreadPreviewServiceClient) error {
		var err error
		resp, err = api.UpgradePreviewToFullMember(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *ThreadPreviewClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}
