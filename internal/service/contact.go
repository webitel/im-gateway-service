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

type contactBackend interface {
	SearchContact(ctx context.Context, req *contactv1.SearchContactRequest) (*contactv1.ContactList, error)
	CreateContact(ctx context.Context, req *contactv1.CreateContactRequest) (*contactv1.Contact, error)
	LocateContact(ctx context.Context, req *contactv1.LocateContactRequest) (*contactv1.LocateContactResponse, error)
}

type ContactService struct {
	logger        *slog.Logger
	contactClient contactBackend
	appConfig     AppConfigProvider
}

func NewContactService(logger *slog.Logger, contactClient *imcontact.Client, appConfig AppConfigProvider) *ContactService {
	return &ContactService{
		logger:        logger,
		contactClient: contactClient,
		appConfig:     appConfig,
	}
}

func (m *ContactService) SearchContact(ctx context.Context, in *contactv1.SearchContactRequest) (*contactv1.ContactList, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	in.DomainId = int32(identity.GetDomainID())

	policy, err := resolveContactPolicy(ctx, m.appConfig, identity)
	if err != nil {
		return nil, err
	}

	if policy.Restricted() {
		// im-contact treats empty Ids as "no filter", so an empty intersection must not reach it.
		in.Ids = policy.AllowedIDs(in.GetIds())
		if len(in.GetIds()) == 0 {
			return &contactv1.ContactList{Page: in.GetPage()}, nil
		}
	}

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
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	policy, err := resolveContactPolicy(ctx, m.appConfig, identity)
	if err != nil {
		return nil, err
	}

	if policy.Restricted() {
		if err := checkContactAllowed(policy, in.GetId()); err != nil {
			return nil, err
		}
	}

	contact, err := m.contactClient.LocateContact(ctx, in)
	if err != nil {
		return nil, err
	}

	return contact, nil
}
