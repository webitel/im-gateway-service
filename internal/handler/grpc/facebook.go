package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.FacebookServiceServer = (*FacebookServiceHandler)(nil)

type FacebookServiceHandler struct {
	providerv1.UnimplementedFacebookServiceServer

	logger *slog.Logger
	client *improviders.FacebookClient
}

func NewFacebookServiceHandler(logger *slog.Logger, client *improviders.FacebookClient) *FacebookServiceHandler {
	return &FacebookServiceHandler{
		logger: logger,
		client: client,
	}
}

func (h *FacebookServiceHandler) CreateFacebookGate(ctx context.Context, req *providerv1.ProviderCreateFacebookGateRequest) (*providerv1.ProviderCreateFacebookGateResponse, error) {
	resp, err := h.client.CreateFacebookGate(ctx, req)
	if err != nil {
		h.logger.Error("FacebookService.CreateFacebookGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}

func (h *FacebookServiceHandler) GetFacebookGate(ctx context.Context, req *providerv1.ProviderGetFacebookGateRequest) (*providerv1.ProviderGetFacebookGateResponse, error) {
	resp, err := h.client.GetFacebookGate(ctx, req)
	if err != nil {
		h.logger.Error("FacebookService.GetFacebookGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}

func (h *FacebookServiceHandler) UpdateFacebookGate(ctx context.Context, req *providerv1.ProviderUpdateFacebookGateRequest) (*providerv1.ProviderUpdateFacebookGateResponse, error) {
	resp, err := h.client.UpdateFacebookGate(ctx, req)
	if err != nil {
		h.logger.Error("FacebookService.UpdateFacebookGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}

func (h *FacebookServiceHandler) DeleteFacebookGate(ctx context.Context, req *providerv1.ProviderDeleteFacebookGateRequest) (*providerv1.ProviderDeleteFacebookGateResponse, error) {
	resp, err := h.client.DeleteFacebookGate(ctx, req)
	if err != nil {
		h.logger.Error("FacebookService.DeleteFacebookGate", slog.Any("err", err))
		return nil, err
	}

	return resp, nil
}
