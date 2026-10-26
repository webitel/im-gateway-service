package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/proto"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
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
	scoped, err := scopeListGatesRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	return h.client.ListGates(ctx, scoped)
}

// scopeListGatesRequest pins the listing to the caller's domain. im-providers
// trusts domain_id on service calls, so a client-supplied value must never
// pass through.
func scopeListGatesRequest(ctx context.Context, req *providerv1.ProviderListGatesRequest) (*providerv1.ProviderListGatesRequest, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	scoped := proto.CloneOf(req)
	scoped.DomainId = identity.GetDomainID()

	return scoped, nil
}
