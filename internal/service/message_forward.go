package service

import (
	"context"
	"log/slog"

	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
)

// ForwardMessages copies the caller's readable messages into a direct chat with
// the target. Which messages the caller may forward is decided by
// im-thread-service from their membership of the source chat.
func (m *MessageService) ForwardMessages(ctx context.Context, in *api.ForwardMessagesRequest) (*api.ForwardMessagesResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, mapper.MapPeerFromProto(in.GetTo()), in.GetSendAs(), identity)
	if err != nil {
		return nil, err
	}

	var internalNote *string
	if in.InternalNote != nil {
		internalNote = in.InternalNote
	}

	resp, err := m.threader.ForwardMessages(ctx, &threadv1.ForwardMessagesRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name: identity.GetName(),
				Via:  identity.GetViaPtr(),
			},
		},
		To:           to,
		MessageIds:   in.GetMessageIds(),
		DomainId:     int32(identity.GetDomainID()),
		SendId:       in.GetSendId(),
		SendAs:       sendAs.GetContactIDPtr(),
		InternalNote: internalNote,
	})
	if err != nil {
		m.logger.Error("ForwardMessages", "err", err,
			slog.Int("messages", len(in.GetMessageIds())),
			slog.String("from_contact_id", identity.GetContactID()),
		)

		return nil, err
	}

	return &api.ForwardMessagesResponse{
		ThreadId:   resp.GetThreadId(),
		Ids:        resp.GetIds(),
		SkippedIds: resp.GetSkippedIds(),
	}, nil
}

// SendInternalNote posts an operator-only note into the thread. The sender is
// taken from the caller's identity; the note is never delivered to the client
// and never forwarded to an external messenger.
func (m *MessageService) SendInternalNote(ctx context.Context, in *api.SendInternalNoteRequest) (*api.SendMessageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, mapper.MapPeerFromProto(in.GetTo()), in.GetSendAs(), identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendInternalNote(ctx, &threadv1.SendInternalNoteRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name: identity.GetName(),
			},
		},
		To:               to,
		Body:             in.GetBody(),
		DomainId:         int64(identity.GetDomainID()),
		SendAs:           sendAs.GetContactIDPtr(),
		ReplyToMessageId: in.ReplyToMessageId,
		SendId:           in.GetSendId(),
	})
	if err != nil {
		return nil, err
	}

	return &api.SendMessageResponse{
		To: in.GetTo(),
		Id: resp.GetId(),
	}, nil
}

func toThreadForwardOrigin(in *api.ForwardOriginInput) *threadv1.ForwardOriginInput {
	if in == nil {
		return nil
	}

	return &threadv1.ForwardOriginInput{
		Kind:           threadv1.ForwardOriginKind(in.GetKind()),
		SenderName:     in.GetSenderName(),
		SenderIss:      in.GetSenderIss(),
		SenderSub:      in.GetSenderSub(),
		OriginalSentAt: in.GetOriginalSentAt(),
	}
}
