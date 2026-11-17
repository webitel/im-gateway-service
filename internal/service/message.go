package service

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1" // ADDED FOR SEARCH
	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/domain/model"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"github.com/webitel/im-gateway-service/internal/service/markdown"
)

const (
	NoNameRecipient = "Unknown Recipient"
)

var _ Messenger = (*MessageService)(nil)

type Messenger interface {
	SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error)
	SendDocument(ctx context.Context, in *dto.SendDocumentRequest) (*dto.SendDocumentResponse, error)
	Read(ctx context.Context, in *dto.ReadMessageRequest) error
	SendContact(ctx context.Context, in *api.SendContactRequest) (*api.SendMessageResponse, error)
	SendInteractive(ctx context.Context, in *api.SendInteractiveMessageRequest) (*api.SendMessageResponse, error)
	SendInteractiveCallback(ctx context.Context, in *api.InteractiveCallbackRequest) (*api.InteractiveCallbackResponse, error)
	SendLocation(ctx context.Context, in *api.SendLocationRequest) (*api.SendMessageResponse, error)
	SendSystemMessage(ctx context.Context, in *dto.SendSystemMessageRequest) (*dto.SendSystemMessageResponse, error)
	EditMessage(ctx context.Context, in *api.EditMessageRequest) (*api.EditMessageResponse, error)
	UpdateMessageDelivery(ctx context.Context, in *api.UpdateMessageDeliveryRequest) error
	DeleteMessages(ctx context.Context, in *api.DeleteMessagesRequest) (*api.DeleteMessagesResponse, error)
	ForwardMessages(ctx context.Context, in *api.ForwardMessagesRequest) (*api.ForwardMessagesResponse, error)
	SetReaction(ctx context.Context, in *api.SetReactionRequest) (*api.SetReactionResponse, error)
	SendInternalNote(ctx context.Context, in *api.SendInternalNoteRequest) (*api.SendMessageResponse, error)
	SendTyping(ctx context.Context, in *api.SendTypingRequest) (*api.SendTypingResponse, error)
}

type MessageService struct {
	logger    *slog.Logger
	threader  threadv1.MessageClient
	contacter recipientContacts
	appConfig AppConfigProvider
}

// SendContact implements Messenger.
func (m *MessageService) SendContact(ctx context.Context, in *api.SendContactRequest) (*api.SendMessageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, mapper.MapPeerFromProto(in.GetTo()), in.GetSendAs(), identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendContact(ctx, &threadv1.SendContactRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name:     identity.GetName(),
				ChatName: identity.GetChatName(),
			},
		},
		To:               to,
		Name:             in.Name,
		Email:            in.Email,
		PhoneNumber:      in.PhoneNumber,
		Metadata:         in.GetMetadata(),
		SendId:           in.GetSendId(),
		DomainId:         domainID,
		SendAs:           sendAs.GetContactIDPtr(),
		ReplyToMessageId: in.ReplyToMessageId,
		ForwardOrigin:    toThreadForwardOrigin(in.GetForwardOrigin()),

		ExternalId:        in.GetExternalId(),
		ReplyToExternalId: in.GetReplyToExternalId(),
		RetryOf:           in.RetryOf,
	})
	if err != nil {
		return nil, err
	}

	return &api.SendMessageResponse{
		To: in.GetTo(),
		Id: resp.GetId(),
	}, nil
}

// SendInteractive implements Messenger.
func (m *MessageService) SendInteractive(ctx context.Context, in *api.SendInteractiveMessageRequest) (*api.SendMessageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, mapper.MapPeerFromProto(in.GetTo()), in.GetSendAs(), identity)
	if err != nil {
		return nil, err
	}

	converted, err := mapper.Convert(in, &threadv1.SendInteractiveMessageRequest{})
	if err != nil {
		return nil, err
	}

	converted.To = to
	converted.From = &threadv1.Peer{
		Kind:     &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
		Identity: senderIdentity(identity),
	}
	converted.DomainId = domainID
	converted.SendAs = sendAs.GetContactIDPtr()

	resp, err := m.threader.SendInteractive(ctx, converted)
	if err != nil {
		return nil, err
	}

	return &api.SendMessageResponse{
		To: in.GetTo(),
		Id: resp.GetId(),
	}, nil
}

// SendInteractiveCallback implements Messenger.
func (m *MessageService) SendInteractiveCallback(ctx context.Context, in *api.InteractiveCallbackRequest) (*api.InteractiveCallbackResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	resp, err := m.threader.SendInteractiveCallback(ctx, &threadv1.InteractiveCallbackRequest{
		ReactedBy: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name:     identity.GetName(),
				ChatName: identity.GetChatName(),
			},
		},
		InReplyTo:    in.GetInReplyTo(),
		ButtonCode:   in.GetButtonCode(),
		CallbackData: in.GetCallbackData(),
	})
	if err != nil {
		return nil, err
	}

	return &api.InteractiveCallbackResponse{
		InReplyTo:    resp.GetInReplyTo(),
		ButtonCode:   resp.GetButtonCode(),
		CallbackData: resp.GetCallbackData(),
		ReactedAt:    resp.GetReactedAt(),
	}, nil
}

func (m *MessageService) SendLocation(ctx context.Context, in *api.SendLocationRequest) (*api.SendMessageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, mapper.MapPeerFromProto(in.GetTo()), in.GetSendAs(), identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendLocation(ctx, &threadv1.SendLocationRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name:     identity.GetName(),
				ChatName: identity.GetChatName(),
			},
		},
		To:               to,
		Latitude:         in.GetLatitude(),
		Longitude:        in.GetLongitude(),
		Name:             in.Name,
		Address:          in.Address,
		Metadata:         in.GetMetadata(),
		SendId:           in.GetSendId(),
		DomainId:         domainID,
		SendAs:           sendAs.GetContactIDPtr(),
		ReplyToMessageId: in.ReplyToMessageId,
		ForwardOrigin:    toThreadForwardOrigin(in.GetForwardOrigin()),

		ExternalId:        in.GetExternalId(),
		ReplyToExternalId: in.GetReplyToExternalId(),
		RetryOf:           in.RetryOf,
	})
	if err != nil {
		return nil, err
	}

	return &api.SendMessageResponse{
		To: in.GetTo(),
		Id: resp.GetId(),
	}, nil
}

func NewMessageService(logger *slog.Logger, threadClient *imthread.Client, contacter *imcontact.Client, appConfig AppConfigProvider) *MessageService {
	return &MessageService{
		logger:    logger,
		threader:  threadClient,
		contacter: contacter,
		appConfig: appConfig,
	}
}

// SendText handles plain text message delivery
func (m *MessageService) SendText(ctx context.Context, in *dto.SendTextRequest) (*dto.SendTextResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	plainText, entities, err := markdown.Parse(in.Body)
	if err != nil {
		if errors.Is(err, markdown.ErrEntityBoundsCorrupted) {
			m.logger.Error("SendText: markdown parser produced invalid entity bounds", "err", err, "from_name", identity.GetName(), "from_contact_id", identity.GetContactID())

			return nil, errors.Internal("internal markdown parser error", errors.WithCause(err), errors.WithID("service.message.send_text.parse_body"))
		}

		m.logger.Warn("SendText: message body rejected by markdown parser", "err", err, "from_name", identity.GetName(), "from_contact_id", identity.GetContactID())

		return nil, errors.InvalidArgument("invalid message formatting", errors.WithCause(err), errors.WithID("service.message.send_text.parse_body"))
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, in.To, in.SendAs, identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendText(ctx, &threadv1.SendTextRequest{
		From: &threadv1.Peer{
			Kind:     &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: senderIdentity(identity),
		},
		To:                to,
		Body:              plainText,
		DomainId:          identity.GetDomainID(),
		SendId:            in.SendID,
		SendAs:            sendAs.GetContactIDPtr(),
		ReplyToMessageId:  in.ReplyToMessageID,
		ExternalId:        in.ExternalID,
		ReplyToExternalId: in.ReplyToExternalID,
		ForwardOrigin:     toThreadForwardOrigin(in.ForwardOrigin),
		Entities:          toThreadEntities(entities),
		Variables:         in.Variables,
		RetryOf:           in.RetryOf,
	})
	if err != nil {
		m.logger.Error("SendText", "err", err, "to", to, "from_name", identity.GetName(), "from_contact_id", identity.GetContactID())

		return nil, err
	}

	return &dto.SendTextResponse{To: in.To, ID: m.parseUUID(resp.GetId())}, nil
}

// SendDocument handles file/attachment delivery
func (m *MessageService) SendDocument(ctx context.Context, in *dto.SendDocumentRequest) (*dto.SendDocumentResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	// Parse the body before resolving send metadata (a gRPC round-trip to
	// im-contact-service): a malformed body is rejected for free, without
	// paying for backend calls that would be discarded anyway.
	plainText, entities, err := markdown.Parse(in.Document.Body)
	if err != nil {
		if errors.Is(err, markdown.ErrEntityBoundsCorrupted) {
			m.logger.Error("SendDocument: markdown parser produced invalid entity bounds", "err", err, "from_name", identity.GetName(), "from_contact_id", identity.GetContactID())

			return nil, errors.Internal("internal markdown parser error", errors.WithCause(err), errors.WithID("service.message.send_document.parse_body"))
		}

		m.logger.Warn("SendDocument: message body rejected by markdown parser", "err", err, "from_name", identity.GetName(), "from_contact_id", identity.GetContactID())

		return nil, errors.InvalidArgument("invalid message formatting", errors.WithCause(err), errors.WithID("service.message.send_document.parse_body"))
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, in.To, in.SendAs, identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendDocument(ctx, &threadv1.SendDocumentRequest{
		From: &threadv1.Peer{
			Kind:     &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: senderIdentity(identity),
		},
		To:                to,
		DomainId:          identity.GetDomainID(),
		SendId:            in.SendID,
		Body:              plainText,
		Documents:         m.mapDocuments(in.Document.Documents),
		SendAs:            sendAs.GetContactIDPtr(),
		ReplyToMessageId:  in.ReplyToMessageID,
		ExternalId:        in.ExternalID,
		ReplyToExternalId: in.ReplyToExternalID,
		ForwardOrigin:     toThreadForwardOrigin(in.ForwardOrigin),
		Entities:          toThreadEntities(entities),
		Variables:         in.Variables,
		RetryOf:           in.RetryOf,
	})
	if err != nil {
		return nil, err
	}

	return &dto.SendDocumentResponse{To: in.To, ID: m.parseUUID(resp.GetId())}, nil
}

func (m *MessageService) SendSystemMessage(ctx context.Context, in *dto.SendSystemMessageRequest) (*dto.SendSystemMessageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	to, sendAs, err := m.resolveSendMetadata(ctx, in.To, in.SendAs, identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.SendSystemMessage(ctx, &threadv1.SendSystemMessageRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name:     identity.GetName(),
				ChatName: identity.GetChatName(),
			},
		},
		To:       to,
		Type:     in.Type,
		Body:     in.Body,
		Metadata: in.Metadata,
		SendId:   in.SendID,
		DomainId: domainID,
		SendAs:   sendAs.GetContactIDPtr(),
	})
	if err != nil {
		return nil, err
	}

	return &dto.SendSystemMessageResponse{
		To: in.To,
		ID: m.parseUUID(resp.GetId()),
	}, nil
}

func (m *MessageService) UpdateMessageDelivery(ctx context.Context, in *api.UpdateMessageDeliveryRequest) error {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return err
	}

	_, err = m.threader.UpdateMessageDelivery(ctx, &threadv1.UpdateMessageDeliveryRequest{
		GateId:            in.GetGateId(),
		ExternalMessageId: in.GetExternalMessageId(),
		Status:            mapDeliveryStatus(in.GetStatus()),
		Reason:            in.GetReason(),
		At:                in.GetAt(),
		DomainId:          domainID,
	})

	return err
}

func mapDeliveryStatus(in api.MessageDeliveryStatus) threadv1.MessageDeliveryStatus {
	switch in {
	case api.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_DELIVERED:
		return threadv1.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_DELIVERED
	case api.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_READ:
		return threadv1.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_READ
	case api.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_FAILED:
		return threadv1.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_FAILED
	default:
		return threadv1.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_UNSPECIFIED
	}
}

// Read implements [Messenger].
func (m *MessageService) Read(ctx context.Context, in *dto.ReadMessageRequest) error {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return err
	}

	_, err = m.threader.Read(ctx, &threadv1.ReadMessageRequest{
		Id:       in.MessageID,
		ThreadId: in.ThreadID,
		UserId:   identity.GetContactID(),
		DomainId: domainID,
		UpToSeq:  in.UpToSeq,
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *MessageService) EditMessage(ctx context.Context, in *api.EditMessageRequest) (*api.EditMessageResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	resp, err := m.threader.EditMessage(ctx, &threadv1.EditMessageRequest{
		Id:   in.GetId(),
		Body: in.GetBody(),
		EditedBy: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name:     identity.GetName(),
				ChatName: identity.GetChatName(),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return &api.EditMessageResponse{
		Id:       resp.GetId(),
		EditedAt: resp.GetEditedAt(),
		Version:  resp.GetVersion(),
	}, nil
}

// DeleteMessages removes the caller's own messages. Ownership and chat state
// are enforced by im-thread-service, which sees the caller through deleted_by.
func (m *MessageService) DeleteMessages(ctx context.Context, in *api.DeleteMessagesRequest) (*api.DeleteMessagesResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	resp, err := m.threader.DeleteMessages(ctx, &threadv1.DeleteMessagesRequest{
		Ids:      in.GetIds(),
		DomainId: domainID,
		DeletedBy: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name: identity.GetName(),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return &api.DeleteMessagesResponse{
		DeletedIds: resp.GetDeletedIds(),
		Skipped:    toAPISkipped(resp.GetSkipped()),
		DeletedAt:  resp.GetDeletedAt(),
	}, nil
}

func toAPISkipped(skipped []*threadv1.SkippedMessage) []*api.SkippedMessage {
	out := make([]*api.SkippedMessage, 0, len(skipped))
	for _, skip := range skipped {
		out = append(out, &api.SkippedMessage{
			Id:     skip.GetId(),
			Reason: api.SkippedMessage_Reason(skip.GetReason()),
		})
	}

	return out
}

func (m *MessageService) SetReaction(ctx context.Context, in *api.SetReactionRequest) (*api.SetReactionResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	resp, err := m.threader.SetReaction(ctx, &threadv1.SetReactionRequest{
		Reactor: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name: identity.GetName(),
			},
		},
		MessageId: in.GetMessageId(),
		Reaction: &threadv1.ReactionContent{
			Kind: &threadv1.ReactionContent_Emoji{Emoji: in.GetEmoji()},
		},
		DomainId: identity.GetDomainID(),
		SendId:   in.GetSendId(),
	})
	if err != nil {
		return nil, err
	}

	return &api.SetReactionResponse{
		Action:    resp.GetAction().String(),
		Emoji:     resp.GetReaction().GetEmoji(),
		ReactedAt: resp.GetReactedAt(),
	}, nil
}

func (m *MessageService) SendTyping(ctx context.Context, in *api.SendTypingRequest) (*api.SendTypingResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	_, err := m.threader.SendTyping(ctx, &threadv1.SendTypingRequest{
		From: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{ContactId: identity.GetContactID()},
			Identity: &threadv1.Identity{
				Name: identity.GetName(),
			},
		},
		To: &threadv1.Peer{
			Kind: &threadv1.Peer_ThreadId{ThreadId: in.GetThreadId()},
		},
		DomainId:    identity.GetDomainID(),
		TimeoutMs:   in.TimeoutMs,
		PreviewText: in.PreviewText,
	})
	if err != nil {
		return nil, err
	}

	return &api.SendTypingResponse{}, nil
}

// --- Internal Helpers & Mappers ---

// senderIdentity builds the thread/v1 identity of the calling sender. The gate is sent both
// as via_ref{id,name,type} and as the deprecated via (id only) so im-thread versions that
// predate via_ref keep working; im-thread prefers via_ref.id when both are set.
func senderIdentity(identity auth.Identifier) *threadv1.Identity {
	return &threadv1.Identity{
		Name:     identity.GetName(),
		ChatName: identity.GetChatName(),
		Via:      identity.GetViaPtr(), //nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref
		ViaRef:   callerViaRef(identity),
	}
}

// callerViaRef returns the gate the caller came through. name/type are empty for
// providers that do not send the new headers yet.
func callerViaRef(identity auth.Identifier) *threadv1.Via {
	return viaRef(identity.GetVia(), identity.GetViaName(), identity.GetViaType())
}

// viaRef returns the gate reference, or nil when there is no gate id.
func viaRef(id, name, typ string) *threadv1.Via {
	if id == "" {
		return nil
	}

	return &threadv1.Via{Id: id, Name: name, Type: typ}
}

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
		return m.resolveContact(ctx, p.ID, p.Issuer, p.Via, domainID)
	case shared.PeerThread:
		return &threadv1.Peer{
			Kind: &threadv1.Peer_ThreadId{ThreadId: p.ID},
		}, nil
	default:
		return nil, errors.InvalidArgument("unknown receiver peer type", errors.WithID("service.message.resolve_recipient.unknown_peer_type"))
	}
}

// resolveSendRecipient: a restricted application may still send to a thread_id — a thread can only
// have been created with an allowed contact (Create/AddMember/Transfer enforce the same policy).
func (m *MessageService) resolveSendRecipient(ctx context.Context, toPeer shared.Peer, session auth.Identifier) (*threadv1.Peer, error) {
	domainID, err := domainIDInt32(session)
	if err != nil {
		return nil, err
	}

	policy, err := resolveContactPolicy(ctx, m.appConfig, session)
	if err != nil {
		return nil, err
	}

	// mapper.MapPeerFromProto returns PeerUnspecified for an absent `to` and for a `to` with no kind set.
	if toPeer.Type == shared.PeerUnspecified {
		if id, ok := policy.DefaultContactID(); ok {
			return m.resolveDefaultContact(ctx, id, domainID)
		}

		return nil, errors.InvalidArgument(
			"recipient (to) is required: the application has no default contact",
			errors.WithID("service.message.resolve_send_recipient.missing_to"),
		)
	}

	if !policy.Restricted() {
		return m.resolveRecipient(ctx, toPeer, domainID)
	}

	switch toPeer.Type {
	case shared.PeerThread:
		return m.resolveRecipient(ctx, toPeer, domainID)

	case shared.PeerContact:
		to, err := m.resolveContact(ctx, toPeer.ID, toPeer.Issuer, toPeer.Via, domainID)
		if err != nil && errors.Code(err) != codes.NotFound {
			return nil, err
		}

		// On NotFound to is nil and its contact id is empty.
		if err := checkContactAllowed(policy, to.GetContactId()); err != nil {
			m.logger.DebugContext(ctx, "recipient denied by application contacts policy",
				slog.String("app_id", session.GetApplicationID()),
				slog.String("contact_id", to.GetContactId()),
			)

			return nil, err
		}

		return to, nil

	default:
		return nil, errors.Forbidden(
			"recipient type is not allowed for this application",
			errors.WithID("service.message.resolve_send_recipient.peer_type_not_allowed"),
		)
	}
}

func (m *MessageService) resolveDefaultContact(ctx context.Context, contactID string, domainID int32) (*threadv1.Peer, error) {
	res, err := m.contacter.SearchContact(ctx, &impb.SearchContactRequest{
		Ids:      []string{contactID},
		DomainId: domainID,
		Size:     1,
		Page:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve default recipient: %w", err)
	}

	contacts := res.GetContacts()

	idx := slices.IndexFunc(contacts, func(c *impb.Contact) bool { return c.GetId() == contactID })
	if idx < 0 {
		return nil, errors.NotFound("default recipient contact not found", errors.WithID("service.message.resolve_default_contact.not_found"))
	}

	contact := contacts[idx]

	return &threadv1.Peer{
		Kind: &threadv1.Peer_ContactId{ContactId: contact.GetId()},
		Identity: &threadv1.Identity{
			Name: cmp.Or(contact.GetName(), contact.GetUsername(), NoNameRecipient),
		},
	}, nil
}

func (m *MessageService) resolveContact(ctx context.Context, sub, iss string, via *string, domainID int32) (*threadv1.Peer, error) {
	if normalize, ok := recipientSubjectNormalizers[iss]; ok {
		sub = normalize(sub)
	}

	contacts, err := m.searchRecipient(ctx, sub, iss, domainID)
	if err != nil {
		return nil, err
	}

	if len(contacts) == 0 {
		return nil, errors.NotFound("recipient contact not found")
	}

	if len(contacts) > 1 {
		return nil, errors.NotFound("too many recipients found")
	}

	contact := contacts[0]
	m.logger.DebugContext(ctx, "resolveContact result",
		slog.String("sub", sub),
		slog.String("iss", iss),
		slog.String("contact_id", contact.GetId()),
		slog.Bool("is_bot", contact.GetIsBot()),
		slog.String("name", contact.GetName()),
		slog.String("username", contact.GetUsername()),
	)
	// Recipient side: only the gate id is known.
	var viaID string
	if via != nil {
		viaID = *via
	}

	return &threadv1.Peer{
		Kind: &threadv1.Peer_ContactId{ContactId: contact.GetId()},
		Identity: &threadv1.Identity{
			Name:   cmp.Or(contact.GetName(), contact.GetUsername(), NoNameRecipient),
			Via:    via, //nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref
			ViaRef: viaRef(viaID, "", ""),
		},
	}, nil
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

func (m *MessageService) guardSendAsAction(ctx context.Context, sendAs *api.PeerIdentity, session auth.Identifier) (*model.AuthContact, error) {
	if sendAs == nil {
		return nil, nil
	}

	if session.GetIssuer() != "webitel" {
		return nil, errors.Forbidden(
			"use send as functionality can only webitel users",
			errors.WithID("service.message.guard_send_as_action.not_webitel_user"),
		)
	}

	searchOnlyBot := true

	contacts, err := m.contacter.SearchContact(ctx, &impb.SearchContactRequest{
		Size:     1,
		IssId:    []string{sendAs.GetIss()},
		Subjects: []string{sendAs.GetSub()},
		OnlyBots: &searchOnlyBot,
	})
	if err != nil {
		return nil, err
	}

	if len(contacts.GetContacts()) < 1 {
		return nil, errors.Forbidden("found zero bots for send as parameter", errors.WithID("service.message.guard_send_as_action.zero_contacts"))
	}

	contact := contacts.GetContacts()[0]

	return &model.AuthContact{
		DC:        int64(contact.GetDomainId()),
		ContactID: contact.GetId(),
		Sub:       contact.GetSubject(),
		Iss:       contact.GetIssId(),
		Name:      contact.GetName(),
	}, nil
}

func (m *MessageService) resolveSendMetadata(ctx context.Context, toPeer shared.Peer, sendAsPeer *api.PeerIdentity, session auth.Identifier) (*threadv1.Peer, *model.AuthContact, error) {
	var (
		to     *threadv1.Peer
		sendAs *model.AuthContact
	)

	egroup, ectx := errgroup.WithContext(ctx)

	egroup.Go(func() error {
		var err error

		to, err = m.resolveSendRecipient(ectx, toPeer, session)

		return err
	})

	egroup.Go(func() error {
		var err error

		sendAs, err = m.guardSendAsAction(ectx, sendAsPeer, session)

		return err
	})

	if err := egroup.Wait(); err != nil {
		return nil, nil, err
	}

	return to, sendAs, nil
}
