package service

import (
	"context"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc/metadata"

	"github.com/webitel/im-gateway-service/gen/go/auth/v1"
	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	stdauth "github.com/webitel/im-gateway-service/infra/auth"
	imauth "github.com/webitel/im-gateway-service/infra/client/im-auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

var _ Accounter = (*AccountService)(nil)

// Accounter defines the behavior for session validation.
type Accounter interface {
	Token(ctx context.Context, request *dto.TokenRequest) (*dto.Authorization, error)
	Inspect(ctx context.Context, headers metadata.MD) (*dto.Authorization, error)
	Logout(ctx context.Context, headers metadata.MD) error
	RegisterDevice(ctx context.Context, headers metadata.MD, request *dto.RegisterDeviceRequest) error
	UnregisterDevice(ctx context.Context, headers metadata.MD, request *dto.UnregisterDeviceRequest) error
	GetAuthorizations(ctx context.Context, request *impb.AccountGetAuthorizationsRequest) (*impb.AccountGetAuthorizationsResponse, error)
}

type AccountService struct {
	client        *imauth.Client
	contactClient *imcontact.Client
}

func (s *AccountService) GetAuthorizations(ctx context.Context, request *impb.AccountGetAuthorizationsRequest) (*impb.AccountGetAuthorizationsResponse, error) {
	session, ok := stdauth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, stdauth.IdentityNotFoundErr
	}

	ouboundRequest := &auth.GetAuthorizationRequest{Dc: session.GetDomainID()}

	mapper.ConvertByProtoReflect(request.ProtoReflect(), ouboundRequest.ProtoReflect())

	internalResponse, err := s.client.GetAuthorizations(ctx, ouboundRequest)
	if err != nil {
		return nil, err
	}

	parsed := &impb.AccountGetAuthorizationsResponse{}
	mapper.ConvertByProtoReflect(internalResponse.ProtoReflect(), parsed.ProtoReflect())

	return parsed, nil
}

func NewAccountService(client *imauth.Client, contactClient *imcontact.Client) *AccountService {
	return &AccountService{client: client, contactClient: contactClient}
}

func (s *AccountService) Inspect(ctx context.Context, headers metadata.MD) (*dto.Authorization, error) {
	if headers == nil {
		return nil, errors.New("headers required for inspect")
	}

	outCtx := metadata.NewOutgoingContext(ctx, headers)

	auth, err := s.client.Inspect(outCtx)
	if err != nil {
		return nil, err
	}
	s.enrichContactType(ctx, auth)
	return auth, nil
}

func (s *AccountService) Token(ctx context.Context, request *dto.TokenRequest) (*dto.Authorization, error) {
	if len(request.Headers) == 0 {
		return nil, errors.New("headers required for token")
	}
	outCtx := metadata.NewOutgoingContext(ctx, request.Headers)
	auth, err := s.client.Token(outCtx, request)
	if err != nil {
		return nil, err
	}
	s.enrichContactType(ctx, auth)
	return auth, nil
}

// enrichContactType fetches contact.type from the contact service and sets it on the authorization.
// The auth service does not populate this field, so we look it up by contact ID.
func (s *AccountService) enrichContactType(ctx context.Context, auth *dto.Authorization) {
	if auth == nil || auth.Contact == nil || auth.Contact.Id == "" {
		return
	}
	res, err := s.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Ids:      []string{auth.Contact.Id},
		Fields:   []string{"id", "type"},
		DomainId: int32(auth.Dc),
		Size:     1,
	})
	if err != nil || len(res.GetContacts()) == 0 {
		return
	}
	auth.Contact.Type = res.GetContacts()[0].GetType()
}

func (s *AccountService) Logout(ctx context.Context, headers metadata.MD) error {
	if headers == nil {
		return errors.New("headers required for logout")
	}

	outCtx := metadata.NewOutgoingContext(ctx, headers)

	return s.client.Logout(outCtx)
}

func (s *AccountService) RegisterDevice(ctx context.Context, headers metadata.MD, request *dto.RegisterDeviceRequest) error {
	if headers == nil {
		return errors.New("headers required for register device")
	}
	outCtx := metadata.NewOutgoingContext(ctx, headers)
	return s.client.RegisterDevice(outCtx, request)
}

func (s *AccountService) UnregisterDevice(ctx context.Context, headers metadata.MD, request *dto.UnregisterDeviceRequest) error {
	if headers == nil {
		return errors.New("headers required for unregister device")
	}
	outCtx := metadata.NewOutgoingContext(ctx, headers)
	return s.client.UnregisterDevice(outCtx, request)
}
