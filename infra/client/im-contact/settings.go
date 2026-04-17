package imcontact

import (
	"context"
	"fmt"
	"log/slog"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

type ContactSettingsClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[contactv1.ContactSettingsClient]
	tls    *infratls.Config
}

func NewPrivacyClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*ContactSettingsClient, error) {
	factory := func(conn *grpc.ClientConn) contactv1.ContactSettingsClient {
		return contactv1.NewContactSettingsClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-contact-client] initialization failed: %w", err)
	}

	return &ContactSettingsClient{
		logger: logger,
		rpc:    c,
	}, nil
}

func (c *ContactSettingsClient) UpdateSettings(ctx context.Context, req *contactv1.UpdateContactSettingsRequest) (*contactv1.Settings, error) {
	var resp *contactv1.Settings
	err := c.rpc.Execute(ctx, func(api contactv1.ContactSettingsClient) error {
		c.logger.Debug("CONTACTS.UPDATE_SETTINGS", slog.Any("req", req))

		var err error
		resp, err = api.Update(ctx, req)
		return err
	})

	return resp, err
}
func (c *ContactSettingsClient) GetSettings(ctx context.Context, req *contactv1.GetContactSettingsRequest) (*contactv1.Settings, error) {
	var resp *contactv1.Settings
	err := c.rpc.Execute(ctx, func(api contactv1.ContactSettingsClient) error {
		c.logger.Debug("CONTACTS.GET_SETTINGS", slog.Any("req", req))

		var err error
		resp, err = api.Get(ctx, req)
		return err
	})

	return resp, err
}
func (c *ContactSettingsClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
