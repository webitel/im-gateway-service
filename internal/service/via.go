package service

import (
	"context"
	"log/slog"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type Via interface {
	Search(ctx context.Context, req *contact.SearchViaRequest) (*contact.SearchViaResponse, error)
	Create(ctx context.Context, req *contact.CreateViaRequest) (*contact.Via, error)
	PartialUpdate(ctx context.Context, req *contact.PartialUpdateViaRequest) (*contact.Via, error)
	Update(ctx context.Context, req *contact.UpdateViaRequest) (*contact.Via, error)
}

type via struct {
	viaClient *imcontact.ViaClient
	logger    *slog.Logger
}

func newVia(logger *slog.Logger, viaClient *imcontact.ViaClient) *via {
	return &via{logger: logger.With("component", "via"), viaClient: viaClient}
}

func (via *via) Search(ctx context.Context, req *contact.SearchViaRequest) (*contact.SearchViaResponse, error) {
	log := via.logger.With("operation", "search")
	response, err := via.viaClient.Search(ctx, req)
	if err != nil {
		log.Error("executing search contact vias request", "error", err)
		return nil, errors.Wrap(err, errors.WithID("services.via.search"))
	}

	return response, nil
}

func (via *via) Update(ctx context.Context, req *contact.UpdateViaRequest) (*contact.Via, error) {
	log := via.logger.With("operation", "update")
	response, err := via.viaClient.Update(ctx, req)
	if err != nil {
		log.Error("updating client via", "error", err)
		return nil, errors.Wrap(err, errors.WithID("service.via.update"))
	}

	return response, nil
}

func (via *via) Create(ctx context.Context, req *contact.CreateViaRequest) (*contact.Via, error) {
	log := via.logger.With("operation", "create")
	response, err := via.viaClient.Create(ctx, req)
	if err != nil {
		log.Error("creating ")
		return nil, errors.Wrap(err, errors.WithID("service.via.create"))
	}

	return response, nil
}

func (via *via) PartialUpdate(ctx context.Context, req *contact.PartialUpdateViaRequest) (*contact.Via, error) {
	log := via.logger.With("operation", "partial_update")
	response, err := via.viaClient.PartialUpdate(ctx, req)
	if err != nil {
		log.Error("executing core API request", "error", err)
		return nil, errors.Wrap(err, errors.WithID("service.via.partial_update"))
	}

	return response, nil
}
