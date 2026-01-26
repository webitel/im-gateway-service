package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	authv1 "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// Interface guard
var _ Messager = (*MessageService)(nil)

type Messager interface {
	SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error)
	SendImage(ctx context.Context, in *dto.SendImageRequest) (*dto.SendImageResponse, error)
	SendDocument(ctx context.Context, in *dto.SendDocumentRequest) (*dto.SendDocumentResponse, error)
}

type MessageService struct {
	logger   *slog.Logger
	threader *imthread.Client
	auther   Auther
}

func NewMessageService(logger *slog.Logger, threadClient *imthread.Client, authClient Auther) *MessageService {
	return &MessageService{
		logger:   logger,
		threader: threadClient,
		auther:   authClient,
	}
}

// SendText handles plain text message delivery
func (m *MessageService) SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error) {
	auth, err := m.auther.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	req := &threadv1.SendTextRequest{
		From:     m.mapAuthContactToPeer(auth.Contact),
		To:       m.mapPeerToProto(in.To),
		Body:     in.Body,
		DomainId: auth.Dc,
	}

	resp, err := m.threader.SendText(ctx, req)
	if err != nil {
		return nil, err
	}

	return &dto.SendTextResponse{
		To: in.To,
		ID: m.parseUUID(resp.GetId()),
	}, nil
}

// SendImage handles image gallery delivery
func (m *MessageService) SendImage(ctx context.Context, in *dto.SendImageRequest) (*dto.SendImageResponse, error) {
	auth, err := m.auther.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	images := make([]*threadv1.ImageInput, len(in.Image.Images))
	for i, img := range in.Image.Images {
		images[i] = &threadv1.ImageInput{
			Id:       img.URL,
			Link:     img.URL,
			MimeType: img.MimeType,
		}
	}

	req := &threadv1.SendImageRequest{
		From: m.mapAuthContactToPeer(auth.Contact),
		To:   m.mapPeerToProto(in.To),
		Image: &threadv1.ImageRequest{
			Images: images,
			Body:   in.Image.Body,
		},
		DomainId: auth.Dc,
	}

	resp, err := m.threader.SendImage(ctx, req)
	if err != nil {
		return nil, err
	}

	return &dto.SendImageResponse{
		To: in.To,
		ID: m.parseUUID(resp.GetId()),
	}, nil
}

// SendDocument handles file/attachment delivery
func (m *MessageService) SendDocument(ctx context.Context, in *dto.SendDocumentRequest) (*dto.SendDocumentResponse, error) {
	auth, err := m.auther.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	docs := make([]*threadv1.DocumentInput, len(in.Document.Documents))
	for i, doc := range in.Document.Documents {
		size := doc.Size
		docs[i] = &threadv1.DocumentInput{
			Id:        doc.URL,
			Url:       doc.URL,
			FileName:  doc.Name,
			MimeType:  doc.MimeType,
			SizeBytes: &size,
		}
	}

	req := &threadv1.SendDocumentRequest{
		From: m.mapAuthContactToPeer(auth.Contact),
		To:   m.mapPeerToProto(in.To),
		Document: &threadv1.DocumentRequest{
			Documents: docs,
			Body:      in.Document.Body,
		},
		DomainId: auth.Dc,
	}

	resp, err := m.threader.SendDocument(ctx, req)
	if err != nil {
		return nil, err
	}

	return &dto.SendDocumentResponse{
		To: in.To,
		ID: m.parseUUID(resp.GetId()),
	}, nil
}

// --- Mappers ---

func (m *MessageService) mapAuthContactToPeer(c *authv1.AuthContact) *threadv1.Peer {
	if c == nil {
		return nil
	}
	return &threadv1.Peer{
		Kind: &threadv1.Peer_ContactId{ContactId: c.Id},
	}
}

func (m *MessageService) mapPeerToProto(p shared.Peer) *threadv1.Peer {
	peer := &threadv1.Peer{}
	idStr := p.ID.String()

	switch p.Type {
	case shared.PeerContact:
		peer.Kind = &threadv1.Peer_ContactId{ContactId: idStr}
	case shared.PeerGroup:
		peer.Kind = &threadv1.Peer_GroupId{GroupId: idStr}
	case shared.PeerChannel:
		peer.Kind = &threadv1.Peer_ChannelId{ChannelId: idStr}
	default:
		m.logger.Warn("mapping unknown peer type", slog.String("type", p.Type.String()))
	}
	return peer
}

func (m *MessageService) parseUUID(id string) uuid.UUID {
	res, err := uuid.Parse(id)
	if err != nil {
		m.logger.Warn("invalid uuid in response", slog.String("raw_id", id))
		return uuid.Nil
	}
	return res
}
