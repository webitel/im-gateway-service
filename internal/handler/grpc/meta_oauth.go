package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.MetaOAuthServiceServer = (*MetaOAuthServiceHandler)(nil)

type MetaOAuthServiceHandler struct {
	providerv1.UnimplementedMetaOAuthServiceServer
	logger *slog.Logger
	client *improviders.MetaOAuthClient
}

func NewMetaOAuthServiceHandler(logger *slog.Logger, client *improviders.MetaOAuthClient) *MetaOAuthServiceHandler {
	return &MetaOAuthServiceHandler{logger: logger, client: client}
}

func (h *MetaOAuthServiceHandler) StartMetaOAuth(ctx context.Context, req *providerv1.ProviderMetaOAuthStartRequest) (*providerv1.ProviderMetaOAuthStartResponse, error) {
	return h.client.StartMetaOAuth(ctx, req)
}

func (h *MetaOAuthServiceHandler) MetaOAuthCallback(ctx context.Context, req *providerv1.ProviderMetaOAuthCallbackRequest) (*providerv1.ProviderMetaOAuthCallbackResponse, error) {
	return h.client.MetaOAuthCallback(ctx, req)
}
