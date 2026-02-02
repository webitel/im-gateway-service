package service

import (
	"context"
	"log/slog"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// Interface guard
var _ Contacter = (*ContactService)(nil)

type Contacter interface {
	SearchContact(ctx context.Context, in *dto.SearchContactRequest) (*dto.ContactList, error)
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

func (m *ContactService) SearchContact(ctx context.Context, in *dto.SearchContactRequest) (*dto.ContactList, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	resp, err := m.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Page:     int32(in.Page),
		Size:     int32(in.Size),
		Q:        in.Q,
		Sort:     in.Sort,
		Fields:   in.Fields,
		AppId:    in.AppID,
		IssId:    in.IssID,
		Type:     in.Type,
		Ids:      in.IDs,
		Subjects: in.Subjects,
		DomainId: int32(identity.GetDomainID()),
	})
	if err != nil {
		return nil, err
	}

	return m.toContactList(resp), nil
}

// --- Internal Mappers ---

func (m *ContactService) toContactList(p *contactv1.ContactList) *dto.ContactList {
	out := &dto.ContactList{
		Page:  int(p.GetPage()),
		Size:  int(p.GetSize()),
		Next:  p.GetNext(),
		Items: make([]*dto.Contact, 0),
	}

	for _, item := range p.GetContacts() {
		out.Items = append(out.Items, m.toContact(item))
	}

	return out
}

func (m *ContactService) toContact(p *contactv1.Contact) *dto.Contact {
	return &dto.Contact{
		ID:        p.GetId(),
		IssID:     p.GetIssId(),
		AppID:     p.GetAppId(),
		Type:      p.GetType(),
		Name:      p.GetName(),
		Username:  p.GetUsername(),
		Metadata:  p.GetMetadata(),
		CreatedAt: p.GetCreatedAt(),
		UpdatedAt: p.GetUpdatedAt(),
		Subject:   p.GetSubject(),
	}
}
