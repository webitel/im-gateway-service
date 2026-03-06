package grpc

import (
	"context"
	"log/slog"

	contactservice "github.com/webitel/im-gateway-service/gen/go/contact/v1"
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
	mapped, err := mapper.Convert(request, &contactservice.SearchContactRequest{} )
	if err != nil {
		return nil, err
	}
	
	out, err := c.contacter.SearchContact(ctx, mapped) 
	if err != nil {
		return nil, err
	}
	return mapper.Convert(out, &impb.ContactList{})
}
