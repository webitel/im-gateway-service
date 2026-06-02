package imcontact

import (
	"context"
	"log/slog"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type ViaClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[contact.ViasClient]
}

func NewViaClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*ViaClient, error) {
	factory := func(conn *grpc.ClientConn) contact.ViasClient {
		return contact.NewViasClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			return nil, errors.New("[CLIENT:VIA] initialization", errors.WithCause(err), errors.WithID("im_contact.settings.new_via_client"), errors.WithCode(s.Code()))
		}
		return nil, errors.Internal("[CLIENT:VIA] initialization", errors.WithCause(err), errors.WithID("im_contact.settings.new_via_client"))
	}

	return &ViaClient{
		logger: logger,
		rpc:    c,
	}, nil
}

func (client *ViaClient) Create(ctx context.Context, in *contact.CreateViaRequest) (*contact.Via, error) {
	var response *contact.Via

	err := client.rpc.Execute(ctx, func(vc contact.ViasClient) error {
		via, err := vc.Create(ctx, in)
		if err != nil {
			return err
		}

		response = via

		return nil
	})

	return response, err
}

func (client *ViaClient) Update(ctx context.Context, in *contact.UpdateViaRequest) (*contact.Via, error) {
	var response *contact.Via
	err := client.rpc.Execute(ctx, func(vc contact.ViasClient) error {
		via, err := vc.Update(ctx, in)
		if err != nil {
			return err
		}

		response = via

		return nil
	})

	return response, err
}

func (client *ViaClient) PartialUpdate(ctx context.Context, in *contact.PartialUpdateViaRequest) (*contact.Via, error) {
	var response *contact.Via
	err := client.rpc.Execute(ctx, func(vc contact.ViasClient) error {
		via, err := vc.PartialUpdate(ctx, in)
		if err != nil {
			return err
		}

		response = via

		return nil
	})

	return response, err
}

func (client *ViaClient) Search(ctx context.Context, in *contact.SearchViaRequest) (*contact.SearchViaResponse, error) {
	var response *contact.SearchViaResponse
	err := client.rpc.Execute(ctx, func(vc contact.ViasClient) error {
		vias, err := vc.Search(ctx, in)
		if err != nil {
			return err
		}

		response = vias

		return nil
	})

	return response, err
}

func (client *ViaClient) Close() error {
	if client.rpc != nil {
		return client.rpc.Close()
	}
	return nil
}
