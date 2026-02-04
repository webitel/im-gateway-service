package imauth

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"

	authv1 "github.com/webitel/im-gateway-service/gen/go/auth/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	"github.com/webitel/im-gateway-service/infra/client/im-auth/mapper"
	"github.com/webitel/im-gateway-service/infra/client/im-auth/mapper/generated"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

const ServiceName string = "im-account-service"

type Client struct {
	logger    *slog.Logger
	rpc       *rpc.Client[authv1.AccountClient]
	tls       *infratls.Config
	inMapper  mapper.InMapper
	outMapper mapper.OutMapper
}

// New initializes a resilient gRPC client for the Auth service.
func New(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*Client, error) {
	factory := func(conn *grpc.ClientConn) authv1.AccountClient {
		return authv1.NewAccountClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-auth-client] initialization failed: %w", err)
	}

	return &Client{
		logger:    logger,
		rpc:       c,
		inMapper:  &generated.InMapperImpl{},
		outMapper: &generated.OutMapperImpl{},
	}, nil
}

// Inspect validates the access token.
func (c *Client) Inspect(ctx context.Context, opts ...grpc.CallOption) (*dto.Authorization, error) {
	var resp *authv1.Authorization

	err := c.rpc.Execute(ctx, func(api authv1.AccountClient) error {
		var err error

		resp, err = api.Inspect(ctx, &authv1.InspectRequest{}, opts...)

		return err
	})
	if err != nil {
		return nil, err
	}
	return c.inMapper.ToAuthorization(resp)
}

func (c *Client) Token(ctx context.Context, in *dto.TokenRequest, opts ...grpc.CallOption) (*dto.Authorization, error) {
	var (
		resp *authv1.Authorization
		req  = c.outMapper.ToTokenRequest(in)
	)

	err := mapper.SetTokenGrant(req, in.GrantType)
	if err != nil {
		return nil, err
	}

	err = c.rpc.Execute(ctx, func(api authv1.AccountClient) error {
		var err error

		resp, err = api.Token(ctx, req, opts...)

		return err
	})
	if err != nil {
		return nil, err
	}

	return c.inMapper.ToAuthorization(resp)
}

func (c *Client) Logout(ctx context.Context, opts ...grpc.CallOption) error {
	err := c.rpc.Execute(ctx, func(api authv1.AccountClient) error {
		var err error

		_, err = api.Logout(ctx, &authv1.LogoutRequest{}, opts...)

		return err
	})

	return err
}

func (c *Client) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}

func (c *Client) maskToken(t string) string {
	if len(t) <= 8 {
		return "****"
	}
	return t[:4] + "..." + t[len(t)-4:]
}
