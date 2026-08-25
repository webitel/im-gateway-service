package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.InstagramServiceServer = (*InstagramServiceHandler)(nil)

type InstagramServiceHandler struct {
	providerv1.UnimplementedInstagramServiceServer

	logger *slog.Logger
	client *improviders.InstagramClient
}

func NewInstagramServiceHandler(logger *slog.Logger, client *improviders.InstagramClient) *InstagramServiceHandler {
	return &InstagramServiceHandler{
		logger: logger,
		client: client,
	}
}

func (h *InstagramServiceHandler) CreateInstagramGate(ctx context.Context, req *providerv1.ProviderCreateInstagramGateRequest) (*providerv1.ProviderCreateInstagramGateResponse, error) {
	resp, err := h.client.CreateInstagramGate(ctx, req)
	if err != nil {
		h.logger.Error("InstagramService.CreateInstagramGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}

func (h *InstagramServiceHandler) GetInstagramGate(ctx context.Context, req *providerv1.ProviderGetInstagramGateRequest) (*providerv1.ProviderGetInstagramGateResponse, error) {
	resp, err := h.client.GetInstagramGate(ctx, req)
	if err != nil {
		h.logger.Error("InstagramService.GetInstagramGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}

func (h *InstagramServiceHandler) UpdateInstagramGate(ctx context.Context, req *providerv1.ProviderUpdateInstagramGateRequest) (*providerv1.ProviderUpdateInstagramGateResponse, error) {
	resp, err := h.client.UpdateInstagramGate(ctx, req)
	if err != nil {
		h.logger.Error("InstagramService.UpdateInstagramGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}

func (h *InstagramServiceHandler) DeleteInstagramGate(ctx context.Context, req *providerv1.ProviderDeleteInstagramGateRequest) (*providerv1.ProviderDeleteInstagramGateResponse, error) {
	resp, err := h.client.DeleteInstagramGate(ctx, req)
	if err != nil {
		h.logger.Error("InstagramService.DeleteInstagramGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}
