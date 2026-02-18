package service

import (
	"context"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc/metadata"

	imauth "github.com/webitel/im-gateway-service/infra/client/im-auth"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

var _ Accounter = (*AccountService)(nil)

// Accounter defines the behavior for session validation.
type Accounter interface {
	Token(ctx context.Context, request *dto.TokenRequest) (*dto.Authorization, error)
	Inspect(ctx context.Context, headers metadata.MD) (*dto.Authorization, error)
	Logout(ctx context.Context, headers metadata.MD) error
}

type AccountService struct {
	client *imauth.Client
}

func NewAccountService(client *imauth.Client) *AccountService {
	return &AccountService{client: client}
}

func (s *AccountService) Inspect(ctx context.Context, headers metadata.MD) (*dto.Authorization, error) {
	if headers == nil {
		return nil, errors.New("headers required for inspect")
	}

	outCtx := metadata.NewOutgoingContext(ctx, headers)

	return s.client.Inspect(outCtx)
}

func (s *AccountService) Token(ctx context.Context, request *dto.TokenRequest) (*dto.Authorization, error) {
	if len(request.Headers) == 0 {
		return nil, errors.New("headers required for token")
	}
	outCtx := metadata.NewOutgoingContext(ctx, request.Headers)
	return s.client.Token(outCtx, request)
}

func (s *AccountService) Logout(ctx context.Context, headers metadata.MD) error {
	if headers == nil {
		return errors.New("headers required for logout")
	}

	outCtx := metadata.NewOutgoingContext(ctx, headers)

	return s.client.Logout(outCtx)
}
