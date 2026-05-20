package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.GateServiceServer = (*GateServiceHandler)(nil)

type GateServiceHandler struct {
	providerv1.UnimplementedGateServiceServer
	logger *slog.Logger
	client *improviders.GateClient
}

func NewGateServiceHandler(logger *slog.Logger, client *improviders.GateClient) *GateServiceHandler {
	return &GateServiceHandler{logger: logger, client: client}
}

func (h *GateServiceHandler) ListGates(ctx context.Context, req *providerv1.ProviderListGatesRequest) (*providerv1.ProviderListGatesResponse, error) {
	return h.client.ListGates(ctx, req)
}
