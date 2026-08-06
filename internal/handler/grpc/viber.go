package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.ViberServiceServer = (*ViberServiceHandler)(nil)

type ViberServiceHandler struct {
	providerv1.UnimplementedViberServiceServer

	logger *slog.Logger
	client *improviders.ViberClient
}

func NewViberServiceHandler(logger *slog.Logger, client *improviders.ViberClient) *ViberServiceHandler {
	return &ViberServiceHandler{
		logger: logger,
		client: client,
	}
}

func (h *ViberServiceHandler) CreateViberGate(ctx context.Context, req *providerv1.ProviderCreateViberGateRequest) (*providerv1.ProviderCreateViberGateResponse, error) {
	resp, err := h.client.CreateViberGate(ctx, req)
	if err != nil {
		h.logger.Error("ViberService.CreateViberGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberServiceHandler) GetViberGate(ctx context.Context, req *providerv1.ProviderGetViberGateRequest) (*providerv1.ProviderGetViberGateResponse, error) {
	resp, err := h.client.GetViberGate(ctx, req)
	if err != nil {
		h.logger.Error("ViberService.GetViberGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberServiceHandler) UpdateViberGate(ctx context.Context, req *providerv1.ProviderUpdateViberGateRequest) (*providerv1.ProviderUpdateViberGateResponse, error) {
	resp, err := h.client.UpdateViberGate(ctx, req)
	if err != nil {
		h.logger.Error("ViberService.UpdateViberGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberServiceHandler) DeleteViberGate(ctx context.Context, req *providerv1.ProviderDeleteViberGateRequest) (*providerv1.ProviderDeleteViberGateResponse, error) {
	resp, err := h.client.DeleteViberGate(ctx, req)
	if err != nil {
		h.logger.Error("ViberService.DeleteViberGate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}
