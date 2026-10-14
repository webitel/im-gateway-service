package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.CustomServiceServer = (*CustomServiceHandler)(nil)

type CustomServiceHandler struct {
	providerv1.UnimplementedCustomServiceServer

	logger *slog.Logger
	client *improviders.CustomClient
}

func NewCustomServiceHandler(logger *slog.Logger, client *improviders.CustomClient) *CustomServiceHandler {
	return &CustomServiceHandler{
		logger: logger,
		client: client,
	}
}

func (h *CustomServiceHandler) CreateCustomGate(ctx context.Context, req *providerv1.ProviderCreateCustomGateRequest) (*providerv1.ProviderCreateCustomGateResponse, error) {
	resp, err := h.client.CreateCustomGate(ctx, req)
	if err != nil {
		h.logger.Error("CustomService.CreateCustomGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *CustomServiceHandler) GetCustomGate(ctx context.Context, req *providerv1.ProviderGetCustomGateRequest) (*providerv1.ProviderGetCustomGateResponse, error) {
	resp, err := h.client.GetCustomGate(ctx, req)
	if err != nil {
		h.logger.Error("CustomService.GetCustomGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *CustomServiceHandler) UpdateCustomGate(ctx context.Context, req *providerv1.ProviderUpdateCustomGateRequest) (*providerv1.ProviderUpdateCustomGateResponse, error) {
	resp, err := h.client.UpdateCustomGate(ctx, req)
	if err != nil {
		h.logger.Error("CustomService.UpdateCustomGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *CustomServiceHandler) DeleteCustomGate(ctx context.Context, req *providerv1.ProviderDeleteCustomGateRequest) (*providerv1.ProviderDeleteCustomGateResponse, error) {
	resp, err := h.client.DeleteCustomGate(ctx, req)
	if err != nil {
		h.logger.Error("CustomService.DeleteCustomGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}
