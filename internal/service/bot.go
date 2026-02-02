package service

import (
	"context"
	"log/slog"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

var _ Botter = (*BotService)(nil)

const (
	BotContactType = "bot"
	BotIssuer      = "bot"
)

type Botter interface {
	CreateBot(ctx context.Context, in *dto.CreateBotRequest) (*dto.Bot, error)
	DeleteBot(ctx context.Context, in *dto.DeleteBotRequest) (*dto.Bot, error)
}

type BotService struct {
	logger        *slog.Logger
	contactClient *imcontact.Client
}

func NewBotService(logger *slog.Logger, contactClient *imcontact.Client) *BotService {
	return &BotService{
		logger:        logger,
		contactClient: contactClient,
	}
}

func (m *BotService) CreateBot(ctx context.Context, in *dto.CreateBotRequest) (*dto.Bot, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	resp, err := m.contactClient.CreateContact(ctx, &contactv1.CreateContactRequest{
		IssId:    BotIssuer,
		Type:     BotContactType,
		Name:     in.Name,
		Username: in.Username,
		Metadata: in.Metadata,
		Subject:  in.SchemaID,
		DomainId: int32(identity.GetDomainID()),
	})
	if err != nil {
		return nil, err
	}

	return m.toBot(resp), nil
}

func (m *BotService) DeleteBot(ctx context.Context, in *dto.DeleteBotRequest) (*dto.Bot, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	contacts, err := m.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Size:     2,
		IssId:    []string{BotIssuer},
		Type:     []string{BotContactType},
		Ids:      []string{in.ID},
		DomainId: int32(identity.GetDomainID()),
	})
	if err != nil {
		return nil, err
	}
	if len(contacts.GetContacts()) == 0 {
		return nil, errors.NotFound("no bots found")
	}

	if len(contacts.GetContacts()) > 1 {
		return nil, errors.Internal("too many bots found")
	}

	resp, err := m.contactClient.DeleteContact(ctx, &contactv1.DeleteContactRequest{})
	if err != nil {
		return nil, err
	}

	return m.toBot(resp), nil
}

// --- Internal Mappers ---

func (m *BotService) toBot(p *contactv1.Contact) *dto.Bot {
	out := &dto.Bot{
		ID:       p.GetId(),
		DomainID: int64(p.GetDomainId()),
		Username: p.GetUsername(),
		Name:     p.GetName(),
		SchemaID: p.GetSubject(),
		Metadata: p.GetMetadata(),
	}

	return out
}

func (m *BotService) toContact(p *contactv1.Contact) *dto.Contact {
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
