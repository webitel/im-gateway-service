package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
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
}

func NewMessageService(logger *slog.Logger, threadClient *imthread.Client) *MessageService {
	return &MessageService{
		logger:   logger,
		threader: threadClient,
	}
}

// SendText handles plain text message delivery
func (m *MessageService) SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error) {
	resp, err := m.threader.SendText(ctx, &threadv1.SendTextRequest{
		From:     m.mapFromPeer(in.From),
		To:       m.mapPeerToProto(in.To),
		Body:     in.Body,
		DomainId: in.DomainID,
	})
	if err != nil {
		return nil, err
	}

	return &dto.SendTextResponse{To: in.To, ID: m.parseUUID(resp.GetId())}, nil
}

// SendImage handles image gallery delivery
func (m *MessageService) SendImage(ctx context.Context, in *dto.SendImageRequest) (*dto.SendImageResponse, error) {
	resp, err := m.threader.SendImage(ctx, &threadv1.SendImageRequest{
		From:     m.mapFromPeer(in.From),
		To:       m.mapPeerToProto(in.To),
		DomainId: in.DomainID,
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
	resp, err := m.threader.SendDocument(ctx, &threadv1.SendDocumentRequest{
		From:     m.mapFromPeer(in.From),
		To:       m.mapPeerToProto(in.To),
		DomainId: in.DomainID,
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

// --- Internal Mappers ---

func (m *MessageService) mapFromPeer(p shared.Peer) *threadv1.Peer {
	return &threadv1.Peer{
		Kind: &threadv1.Peer_ContactId{ContactId: p.ID.String()},
	}
}

func (m *MessageService) mapPeerToProto(p shared.Peer) *threadv1.Peer {
	peer := &threadv1.Peer{}
	switch p.Type {
	case shared.PeerContact:
		peer.Kind = &threadv1.Peer_ContactId{ContactId: p.ID.String()}
	case shared.PeerGroup:
		peer.Kind = &threadv1.Peer_GroupId{GroupId: p.ID.String()}
	case shared.PeerChannel:
		peer.Kind = &threadv1.Peer_ChannelId{ChannelId: p.ID.String()}
	default:
		m.logger.Warn("mapping unknown peer type", slog.String("type", p.Type.String()))
	}
	return peer
}

func (m *MessageService) mapImages(src []*dto.Image) []*threadv1.ImageInput {
	res := make([]*threadv1.ImageInput, len(src))
	for i, img := range src {
		if img == nil {
			continue
		}
		res[i] = &threadv1.ImageInput{
			Id:       fmt.Sprintf("%d", img.ID),
			Name:     img.Name,
			Link:     img.URL,
			MimeType: img.MimeType,
		}
	}
	return res
}

func (m *MessageService) mapDocuments(src []*dto.Document) []*threadv1.DocumentInput {
	res := make([]*threadv1.DocumentInput, len(src))
	for i, doc := range src {
		if doc == nil {
			continue
		}
		size := doc.Size
		res[i] = &threadv1.DocumentInput{
			Id:        fmt.Sprintf("%d", doc.ID),
			Url:       doc.URL,
			FileName:  doc.Name,
			MimeType:  doc.MimeType,
			SizeBytes: &size,
		}
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
