package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/infra/server/grpc/interceptors"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ impb.MessageServer = (*MessageService)(nil)

type MessageService struct {
	impb.UnimplementedMessageServer
	logger   *slog.Logger
	messager service.Messager
}

func NewMessageService(logger *slog.Logger, messager service.Messager) *MessageService {
	return &MessageService{
		logger:   logger,
		messager: messager,
	}
}

// SendText handles plain text message delivery
func (m *MessageService) SendText(ctx context.Context, in *impb.SendTextRequest) (*impb.SendTextResponse, error) {
	// [AUTH] Extract pre-resolved identity from context via interceptor
	identity, ok := interceptors.GetIdentity(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "IDENTITY_NOT_FOUND: authentication context missing")
	}

	out, err := m.messager.SendText(ctx, mapper.MapToSendTextRequest(in, identity))
	if err != nil {
		return nil, err
	}

	return mapper.MapToSendTextResponse(out), nil
}

// SendImage handles image gallery delivery
func (m *MessageService) SendImage(ctx context.Context, in *impb.SendImageRequest) (*impb.SendImageResponse, error) {
	identity, ok := interceptors.GetIdentity(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "IDENTITY_NOT_FOUND: authentication context missing")
	}

	// Map enriched with identity
	out, err := m.messager.SendImage(ctx, mapper.MapToSendImageRequest(in, identity))
	if err != nil {
		m.logger.Error("failed to send image", "error", err)
		return nil, err
	}

	return mapper.MapToSendImageResponse(out), nil
}

// SendDocument handles file/attachment delivery
func (m *MessageService) SendDocument(ctx context.Context, in *impb.SendDocumentRequest) (*impb.SendDocumentResponse, error) {
	identity, ok := interceptors.GetIdentity(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "IDENTITY_NOT_FOUND: authentication context missing")
	}

	// Map enriched with identity
	out, err := m.messager.SendDocument(ctx, mapper.MapToSendDocumentRequest(in, identity))
	if err != nil {
		m.logger.Error("failed to send document", "error", err)
		return nil, err
	}

	return mapper.MapToSendDocumentResponse(out), nil
}

// SendFile implements [gatewayv1.MessageServer].
func (m *MessageService) SendFile(context.Context, *impb.SendDocumentRequest) (*impb.SendDocumentResponse, error) {
	panic("unimplemented")
}
