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
	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const ServiceName string = "im-thread-service"

var (
	_ threadv1.MessageClient = (*Client)(nil)
)

type Client struct {
	logger *slog.Logger
	rpc    *rpc.Client[threadv1.MessageClient]
	tls    *infratls.Config
}

func New(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*Client, error) {
	factory := func(conn *grpc.ClientConn) threadv1.MessageClient {
		return threadv1.NewMessageClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-thread-client] initialization failed: %w", err)
	}

	return &Client{
		logger: logger,
		rpc:    c,
	}, nil
}

func (c *Client) SendContact(ctx context.Context, in *threadv1.SendContactRequest, opts ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	var resp *threadv1.SendMessageResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		var err error
		resp, err = api.SendContact(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *Client) SendInteractive(ctx context.Context, in *threadv1.SendInteractiveMessageRequest, opts ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	var resp *threadv1.SendMessageResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		var err error
		resp, err = api.SendInteractive(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *Client) SendInteractiveCallback(ctx context.Context, in *threadv1.InteractiveCallbackRequest, opts ...grpc.CallOption) (*threadv1.InteractiveCallbackResponse, error) {
	var resp *threadv1.InteractiveCallbackResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		var err error
		resp, err = api.SendInteractiveCallback(ctx, in, opts...)
		return err
	})

	return resp, err
}

func (c *Client) SendLocation(ctx context.Context, in *threadv1.SendLocationRequest, opts ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	var resp *threadv1.SendMessageResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		var err error
		resp, err = api.SendLocation(ctx, in, opts...)
		return err
	})

	return resp, err
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

func (c *Client) SendSystemMessage(ctx context.Context, in *threadv1.SendSystemMessageRequest, opts ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	var resp *threadv1.SendMessageResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		c.logger.Debug("THREAD.SEND_SYSTEM_MESSAGE", slog.Any("req", in))

		var err error
		resp, err = api.SendSystemMessage(ctx, in, opts...)
		return err
	})

	return resp, err
}

// Read implements [thread.MessageClient].
func (c *Client) Read(ctx context.Context, in *threadv1.ReadMessageRequest, opts ...grpc.CallOption) (*threadv1.ReadMessageResponse, error) {
	var resp *threadv1.ReadMessageResponse

	err := c.rpc.Execute(ctx, func(api threadv1.MessageClient) error {
		c.logger.Debug("THREAD.READ_MESSAGE", slog.Any("req", in))

		var err error
		resp, err = api.Read(ctx, in, opts...)
		return err
	})

	return resp, err
}

// SendSystemMessage implements thread.MessageClient.
func (c *Client) SendSystemMessage(ctx context.Context, in *threadv1.SendSystemMessageRequest, opts ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	return nil, errors.New("unimplemented", errors.WithCode(codes.Unimplemented))
}

// Close gracefully shuts down the underlying gRPC connection pool.
func (c *Client) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
