package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.ViberBmServiceServer = (*ViberBmServiceHandler)(nil)

type ViberBmServiceHandler struct {
	providerv1.UnimplementedViberBmServiceServer

	logger *slog.Logger
	client *improviders.ViberBmClient
}

func NewViberBmServiceHandler(logger *slog.Logger, client *improviders.ViberBmClient) *ViberBmServiceHandler {
	return &ViberBmServiceHandler{
		logger: logger,
		client: client,
	}
}

func (h *ViberBmServiceHandler) CreateViberBm(ctx context.Context, req *providerv1.ProviderCreateViberBmGateRequest) (*providerv1.ProviderCreateViberBmGateResponse, error) {
	resp, err := h.client.CreateViberBm(ctx, req)
	if err != nil {
		h.logger.Error("ViberBmService.CreateViberBm", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberBmServiceHandler) GetViberBm(ctx context.Context, req *providerv1.ProviderGetViberBmGateRequest) (*providerv1.ProviderGetViberBmGateResponse, error) {
	resp, err := h.client.GetViberBm(ctx, req)
	if err != nil {
		h.logger.Error("ViberBmService.GetViberBm", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberBmServiceHandler) UpdateViberBm(ctx context.Context, req *providerv1.ProviderUpdateViberBmGateRequest) (*providerv1.ProviderUpdateViberBmGateResponse, error) {
	resp, err := h.client.UpdateViberBm(ctx, req)
	if err != nil {
		h.logger.Error("ViberBmService.UpdateViberBm", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberBmServiceHandler) DeleteViberBm(ctx context.Context, req *providerv1.ProviderDeleteViberBmGateRequest) (*providerv1.ProviderDeleteViberBmGateResponse, error) {
	resp, err := h.client.DeleteViberBm(ctx, req)
	if err != nil {
		h.logger.Error("ViberBmService.DeleteViberBm", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberBmServiceHandler) ListViberBm(ctx context.Context, req *providerv1.ProviderListViberBmGatesRequest) (*providerv1.ProviderListViberBmGatesResponse, error) {
	resp, err := h.client.ListViberBm(ctx, req)
	if err != nil {
		h.logger.Error("ViberBmService.ListViberBm", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}

func (h *ViberBmServiceHandler) SendViberBmTemplate(ctx context.Context, req *providerv1.SendViberBmTemplateRequest) (*providerv1.SendViberBmTemplateResponse, error) {
	resp, err := h.client.SendViberBmTemplate(ctx, req)
	if err != nil {
		h.logger.Error("ViberBmService.SendViberBmTemplate", slog.Any("err", err))
		return nil, err
	}
	return resp, nil
}
