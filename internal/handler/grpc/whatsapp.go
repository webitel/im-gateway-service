package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.WhatsAppServiceServer = (*WhatsAppServiceHandler)(nil)

type WhatsAppServiceHandler struct {
	providerv1.UnimplementedWhatsAppServiceServer
	logger *slog.Logger
	client *improviders.WhatsAppClient
}

func NewWhatsAppServiceHandler(logger *slog.Logger, client *improviders.WhatsAppClient) *WhatsAppServiceHandler {
	return &WhatsAppServiceHandler{logger: logger, client: client}
}

func (h *WhatsAppServiceHandler) CreateWhatsAppGate(ctx context.Context, req *providerv1.CreateGateRequest) (*providerv1.GateResponse, error) {
	return h.client.CreateWhatsAppGate(ctx, req)
}

func (h *WhatsAppServiceHandler) GetWhatsAppGate(ctx context.Context, req *providerv1.ProviderGetWhatsAppGateRequest) (*providerv1.ProviderGetWhatsAppGateResponse, error) {
	return h.client.GetWhatsAppGate(ctx, req)
}

func (h *WhatsAppServiceHandler) UpdateWhatsAppGate(ctx context.Context, req *providerv1.ProviderUpdateWhatsAppGateRequest) (*providerv1.ProviderUpdateWhatsAppGateResponse, error) {
	return h.client.UpdateWhatsAppGate(ctx, req)
}

func (h *WhatsAppServiceHandler) DeleteWhatsAppGate(ctx context.Context, req *providerv1.ProviderDeleteWhatsAppGateRequest) (*providerv1.ProviderDeleteWhatsAppGateResponse, error) {
	return h.client.DeleteWhatsAppGate(ctx, req)
}
