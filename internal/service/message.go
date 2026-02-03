package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1" // ADDED FOR SEARCH
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

const (
	NoNameRecepient = "Unknown Recepient"
)

var _ Messenger = (*MessageService)(nil)

type Messenger interface {
	SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error)
	SendImage(ctx context.Context, in *dto.SendImageRequest) (*dto.SendImageResponse, error)
	SendDocument(ctx context.Context, in *dto.SendDocumentRequest) (*dto.SendDocumentResponse, error)
}

type MessageService struct {
	logger    *slog.Logger
	threader  *imthread.Client
	contacter *imcontact.Client
}

func NewMessageService(logger *slog.Logger, threadClient *imthread.Client, contacter *imcontact.Client) *MessageService {
	return &MessageService{
		logger:    logger,
		threader:  threadClient,
		contacter: contacter,
	}
}

// SendText handles plain text message delivery
func (m *MessageService) SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	to, err := m.resolveRecipient(ctx, in.To, int32(identity.GetDomainID()))
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendText(ctx, &threadv1.SendTextRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name: identity.GetName(),
			},
		},
		To:       to,
		Body:     in.Body,
		DomainId: identity.GetDomainID(),
	})
	if err != nil {
		return nil, err
	}

	return &dto.SendTextResponse{To: in.To, ID: m.parseUUID(resp.GetId())}, nil
}

// SendImage handles image gallery delivery
func (m *MessageService) SendImage(ctx context.Context, in *dto.SendImageRequest) (*dto.SendImageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	to, err := m.resolveRecipient(ctx, in.To, int32(identity.GetDomainID()))
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendImage(ctx, &threadv1.SendImageRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
		},
		To:       to,
		DomainId: identity.GetDomainID(),
		Image: &threadv1.ImageRequest{
			Body:   in.Image.Body,
			Images: m.mapImages(in.Image.Images),
		},
	})
	if err != nil {
		return nil, err
	}

	return &dto.SendImageResponse{To: in.To, ID: m.parseUUID(resp.GetId())}, nil
}

// SendDocument handles file/attachment delivery
func (m *MessageService) SendDocument(ctx context.Context, in *dto.SendDocumentRequest) (*dto.SendDocumentResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	to, err := m.resolveRecipient(ctx, in.To, int32(identity.GetDomainID()))
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendDocument(ctx, &threadv1.SendDocumentRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
		},
		To:       to,
		DomainId: identity.GetDomainID(),
		Document: &threadv1.DocumentRequest{
			Body:      in.Document.Body,
			Documents: m.mapDocuments(in.Document.Documents),
		},
	})
	if err != nil {
		return nil, err
	}

	return &dto.SendDocumentResponse{To: in.To, ID: m.parseUUID(resp.GetId())}, nil
}

// --- Internal Helpers & Mappers ---

// [INTERNAL] resolveRecipient maps shared.Peer to threadv1.Peer and performs contact lookup if necessary
func (m *MessageService) resolveRecipient(ctx context.Context, p shared.Peer, domainID int32) (*threadv1.Peer, error) {
	switch p.Type {
	case shared.PeerGroup:
		return &threadv1.Peer{
			Kind: &threadv1.Peer_GroupId{GroupId: p.ID},
		}, nil

	case shared.PeerChannel:
		return &threadv1.Peer{
			Kind: &threadv1.Peer_ChannelId{ChannelId: p.ID},
		}, nil
	case shared.PeerContact:
		return m.resolveContact(ctx, p.ID, p.Issuer, domainID)
	default:
		return nil, errors.New("unknown receiver peer type")
	}
}

func (m *MessageService) resolveContact(ctx context.Context, sub, iss string, domainID int32) (*threadv1.Peer, error) {
	res, err := m.contacter.SearchContact(ctx, &impb.SearchContactRequest{
		Subjects: []string{sub},
		IssId:    []string{iss},
		DomainId: domainID,
		Size:     2,
		Page:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve recipient: %w", err)
	}

	if len(res.GetContacts()) == 0 {
		return nil, errors.NotFound("recipient contact not found")
	}

	if len(res.GetContacts()) > 1 {
		return nil, errors.NotFound("too many recipients found")
	}
	contact := res.GetContacts()[0]
	return &threadv1.Peer{
		Kind: &threadv1.Peer_ContactId{ContactId: contact.GetId()},
		Identity: &threadv1.Identity{
			Name: coalesceString(contact.GetName(), contact.GetUsername(), NoNameRecepient),
		},
	}, nil
}

func coalesceString(args ...string) string {
	for _, s := range args {
		if s != "" {
			return s
		}
	}
	return ""
}

func (m *MessageService) mapImages(src []*dto.Image) []*threadv1.ImageInput {
	res := make([]*threadv1.ImageInput, 0, len(src))
	for _, img := range src {
		if img == nil {
			continue
		}
		res = append(res, &threadv1.ImageInput{
			Id:       fmt.Sprintf("%d", img.ID),
			Name:     img.Name,
			Link:     img.URL,
			MimeType: img.MimeType,
		})
	}
	return res
}

func (m *MessageService) mapDocuments(src []*dto.Document) []*threadv1.DocumentInput {
	res := make([]*threadv1.DocumentInput, 0, len(src))
	for _, doc := range src {
		if doc == nil {
			continue
		}
		size := doc.Size
		res = append(res, &threadv1.DocumentInput{
			Id:        fmt.Sprintf("%d", doc.ID),
			Url:       doc.URL,
			FileName:  doc.Name,
			MimeType:  doc.MimeType,
			SizeBytes: &size,
		})
	}
	return res
}

func (m *MessageService) parseUUID(id string) uuid.UUID {
	res, err := uuid.Parse(id)
	if err != nil {
		m.logger.Warn("invalid uuid in response", slog.String("raw_id", id))
		return uuid.Nil
	}
	return res
}
