package service

import (
	"context"
	"log/slog"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
)

// Interface guard
var _ Contacter = (*ContactService)(nil)

type Contacter interface {
	SearchContact(context.Context, *contactv1.SearchContactRequest) (*contactv1.ContactList, error)
	CreateContact(context.Context, *contactv1.CreateContactRequest) (*contactv1.Contact, error)
	Locate(ctx context.Context, in *contactv1.LocateContactRequest) (*contactv1.LocateContactResponse, error)
}

type ContactService struct {
	logger        *slog.Logger
	contactClient *imcontact.Client
}

func NewContactService(logger *slog.Logger, contactClient *imcontact.Client) *ContactService {
	return &ContactService{
		logger:        logger,
		contactClient: contactClient,
	}
}

func (m *ContactService) SearchContact(ctx context.Context, in *contactv1.SearchContactRequest) (*contactv1.ContactList, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	in.DomainId = int32(identity.GetDomainID())
	return m.contactClient.SearchContact(ctx, in)
}

func (m *ContactService) CreateContact(ctx context.Context, in *contactv1.CreateContactRequest) (*contactv1.Contact, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	in.DomainId = int32(identity.GetDomainID())
	return m.contactClient.CreateContact(ctx, in)
}

func (m *ContactService) Locate(ctx context.Context, in *contactv1.LocateContactRequest) (*contactv1.LocateContactResponse, error) {
	if _, ok := auth.GetIdentityFromContext(ctx); !ok {
		return nil, auth.IdentityNotFoundErr
	}

	contact, err := m.contactClient.LocateContact(ctx, in)
	if err != nil {
		return nil, err
	}

	return contact, nil
}
