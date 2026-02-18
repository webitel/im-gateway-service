package grpc

import (
	"context"
	"log/slog"

	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper/generated"
	"google.golang.org/grpc/metadata"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

var _ impb.AccountServer = (*AccountService)(nil)

type AccountService struct {
	impb.UnimplementedAccountServer

	logger    *slog.Logger
	accounter service.Accounter
	outMapper mapper.AccountToPbMapper
	inMapper  mapper.AccountToDtoMapper
}

func (a *AccountService) Token(ctx context.Context, request *impb.TokenRequest) (*impb.Authorization, error) {
	req := a.inMapper.ToTokenRequest(request)
	req.GrantType = mapper.ParseGrantType(request)

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("headers required for token")
	}
	req.Headers = md

	auth, err := a.accounter.Token(ctx, req)
	if err != nil {
		return nil, err
	}

	return a.outMapper.ToAuthorization(auth)
}

func (a *AccountService) Logout(ctx context.Context, request *impb.LogoutRequest) (*impb.LogoutResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.Forbidden("logout failed, headers required")
	}

	err := a.accounter.Logout(ctx, md)
	return nil, err
}

func (a *AccountService) Inspect(ctx context.Context, request *impb.InspectRequest) (*impb.Authorization, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.Forbidden("inspect failed, headers required")
	}

	auth, err := a.accounter.Inspect(ctx, md)
	if err != nil {
		return nil, err
	}

	return a.outMapper.ToAuthorization(auth)
}

func (a *AccountService) RegisterDevice(ctx context.Context, request *impb.RegisterDeviceRequest) (*impb.RegisterDeviceResponse, error) {
	// TODO implement me
	return nil, nil
}

func (a *AccountService) UnregisterDevice(ctx context.Context, request *impb.UnregisterDeviceRequest) (*impb.UnregisterDeviceResponse, error) {
	// TODO implement me
	return nil, nil
}

func NewAccountService(logger *slog.Logger, accounter service.Accounter) *AccountService {
	return &AccountService{
		logger:    logger,
		accounter: accounter,
		inMapper:  &generated.AccountToDtoMapperImpl{},
		outMapper: &generated.AccountToPbMapperImpl{},
	}
}
