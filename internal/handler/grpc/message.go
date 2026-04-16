package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

var (
	_ impb.MessageServer = (*MessageService)(nil)
)

type MessageService struct {
	impb.UnimplementedMessageServer

	logger    *slog.Logger
	messenger service.Messenger
}

func NewMessageService(logger *slog.Logger, messager service.Messenger) *MessageService {
	return &MessageService{
		logger:    logger,
		messenger: messager,
	}
}

func (m *MessageService) SendText(ctx context.Context, in *impb.SendTextRequest) (*impb.SendTextResponse, error) {
	out, err := m.messenger.SendText(ctx, mapper.MapToSendTextRequest(in))
	if err != nil {
		return nil, err
	}

	return mapper.MapToSendTextResponse(out), nil
}

func (m *MessageService) SendImage(ctx context.Context, in *impb.SendImageRequest) (*impb.SendImageResponse, error) {
	out, err := m.messenger.SendImage(ctx, mapper.MapToSendImageRequest(in))
	if err != nil {
		return nil, err
	}

	return mapper.MapToSendImageResponse(out), nil
}

func (m *MessageService) SendDocument(ctx context.Context, in *impb.SendDocumentRequest) (*impb.SendDocumentResponse, error) {
	out, err := m.messenger.SendDocument(ctx, mapper.MapToSendDocumentRequest(in))
	if err != nil {
		return nil, err
	}

	return mapper.MapToSendDocumentResponse(out), nil
}

func (m *MessageService) Read(ctx context.Context, in *impb.ReadMessageRequest) (*impb.ReadMessageResponse, error) {
	err := m.messenger.Read(ctx, mapper.MapToReadMessageRequest(in))
	if err != nil {
		return nil, err
	}
	return &impb.ReadMessageResponse{}, nil
}

func (m *MessageService) SendContact(ctx context.Context, in *impb.SendContactRequest) (*impb.SendMessageResponse, error) {
	out, err := m.messenger.SendContact(ctx, in)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *MessageService) SendInteractive(ctx context.Context, in *impb.SendInteractiveMessageRequest) (*impb.SendMessageResponse, error) {
	out, err := m.messenger.SendInteractive(ctx, in)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *MessageService) SendInteractiveCallback(ctx context.Context, in *impb.InteractiveCallbackRequest) (*impb.InteractiveCallbackResponse, error) {
	out, err := m.messenger.SendInteractiveCallback(ctx, in)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *MessageService) SendLocation(ctx context.Context, in *impb.SendLocationRequest) (*impb.SendMessageResponse, error) {
	out, err := m.messenger.SendLocation(ctx, in)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *MessageService) SendSystemMessage(ctx context.Context, in *impb.SendSystemMessageRequest) (*impb.SendMessageResponse, error) {
	out, err := m.messenger.SendSystemMessage(ctx, mapper.MapPbToSystemMessageRequest(in))
	if err != nil {
		return nil, err
	}

	return mapper.MapToSendSystemMessageResponse(out), nil
}
