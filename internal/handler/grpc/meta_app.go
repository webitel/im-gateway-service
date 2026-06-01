package grpc

import (
	"context"
	"log/slog"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
)

var _ providerv1.MetaAppServiceServer = (*MetaAppServiceHandler)(nil)

type MetaAppServiceHandler struct {
	providerv1.UnimplementedMetaAppServiceServer
	logger *slog.Logger
	client *improviders.MetaAppClient
}

func NewMetaAppServiceHandler(logger *slog.Logger, client *improviders.MetaAppClient) *MetaAppServiceHandler {
	return &MetaAppServiceHandler{logger: logger, client: client}
}

func (h *MetaAppServiceHandler) CreateMetaApp(ctx context.Context, req *providerv1.ProviderCreateMetaAppRequest) (*providerv1.ProviderCreateMetaAppResponse, error) {
	return h.client.CreateMetaApp(ctx, req)
}

func (h *MetaAppServiceHandler) GetMetaApp(ctx context.Context, req *providerv1.ProviderGetMetaAppRequest) (*providerv1.ProviderGetMetaAppResponse, error) {
	return h.client.GetMetaApp(ctx, req)
}

func (h *MetaAppServiceHandler) UpdateMetaApp(ctx context.Context, req *providerv1.ProviderUpdateMetaAppRequest) (*providerv1.ProviderUpdateMetaAppResponse, error) {
	return h.client.UpdateMetaApp(ctx, req)
}

func (h *MetaAppServiceHandler) DeleteMetaApp(ctx context.Context, req *providerv1.ProviderDeleteMetaAppRequest) (*providerv1.ProviderDeleteMetaAppResponse, error) {
	return h.client.DeleteMetaApp(ctx, req)
}
