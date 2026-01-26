package imthread

import (
	"context"
	"fmt"
	"log/slog"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

const ServiceName string = "im-thread-service"

// [GENERIC_INTERFACE_GUARD] Ensures Client matches the generated gRPC client interface.
var _ threadv1.MessageClient = (*Client)(nil)

type Client struct {
	logger *slog.Logger
	// [GENERIC_RPC] Underlying go-kit RPC client using the generated MessageClient stub
	rpc *rpc.Client[threadv1.MessageClient]
}

// New initializes a resilient gRPC client for the Message service.
func New(logger *slog.Logger, discovery discovery.DiscoveryProvider) (*Client, error) {
	// [FACTORY] Helper to instantiate the gRPC stub upon connection
	factory := func(conn *grpc.ClientConn) threadv1.MessageClient {
		return threadv1.NewMessageClient(conn)
	}

	// [INIT] Create the base gRPC client with discovery and circuit breaker
	c, err := webitel.New(logger, discovery, ServiceName, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-thread-client] initialization failed: %w", err)
	}

	return &Client{
		logger: logger,
		rpc:    c,
	}, nil
}

// SendText delivers a plain text message through the Thread service.
func (c *Client) SendText(ctx context.Context, in *threadv1.SendTextRequest, opts ...grpc.CallOption) (*threadv1.SendTextResponse, error) {
	var resp *threadv1.SendTextResponse

	// [EXECUTE] go-kit's Execute handles load balancing, retries, and error mapping
	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		c.logger.Debug("THREAD.SEND_TEXT", slog.Any("req", in))

		var err error
		resp, err = api.SendText(ctx, in, opts...)
		return err
	})

	return resp, err
}

// SendDocument delivers a document message through the Thread service.
func (c *Client) SendDocument(ctx context.Context, in *threadv1.SendDocumentRequest, opts ...grpc.CallOption) (*threadv1.SendDocumentResponse, error) {
	var resp *threadv1.SendDocumentResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		c.logger.Debug("THREAD.SEND_DOCUMENT", slog.Any("req", in))

		var err error
		resp, err = api.SendDocument(ctx, in, opts...)
		return err
	})

	return resp, err
}

// SendImage delivers an image message through the Thread service.
func (c *Client) SendImage(ctx context.Context, in *threadv1.SendImageRequest, opts ...grpc.CallOption) (*threadv1.SendImageResponse, error) {
	var resp *threadv1.SendImageResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		c.logger.Debug("THREAD.SEND_IMAGE", slog.Any("req", in))

		var err error
		resp, err = api.SendImage(ctx, in, opts...)
		return err
	})

	return resp, err
}

// Close gracefully shuts down the underlying gRPC connection pool.
func (c *Client) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
