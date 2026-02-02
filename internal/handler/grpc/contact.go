package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

var _ impb.ContactsServer = (*ContactService)(nil)

type ContactService struct {
	impb.UnimplementedContactsServer

	logger    *slog.Logger
	contacter service.Contacter
}

func NewContactService(logger *slog.Logger, contacter service.Contacter) *ContactService {
	return &ContactService{
		logger:    logger,
		contacter: contacter,
	}
}

func (c *ContactService) Search(ctx context.Context, request *impb.SearchContactRequest) (*impb.ContactList, error) {
	out, err := c.contacter.SearchContact(ctx, mapper.MapToSearchContactRequest(request))
	if err != nil {
		return nil, err
	}
	return mapper.MapToContactList(out), nil
}
