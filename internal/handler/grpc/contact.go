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

// Create implements [api.ContactsServer].
func (c *ContactService) Create(ctx context.Context, req *impb.CreateContactRequest) (*impb.Contact, error) {
	mapped, err := mapper.Convert(req, new(contactservice.CreateContactRequest))
	if err != nil {
		return nil, err
	}

	out, err := c.contacter.CreateContact(ctx, mapped)
	if err != nil {
		return nil, err
	}
	return mapper.Convert(out, new(impb.Contact))
}

func (c *ContactService) Search(ctx context.Context, request *impb.SearchContactRequest) (*impb.ContactList, error) {
	mapped, err := mapper.Convert(request, new(contactservice.SearchContactRequest))
	if err != nil {
		return nil, err
	}

	out, err := c.contacter.SearchContact(ctx, mapped)
	if err != nil {
		return nil, err
	}
	return mapContactsToGatewayResponseProto(out), nil
}

// WARNING: rewritten from proto proxy
// mapper to manual due to different props for list [Contacts vs Items]
func mapContactsToGatewayResponseProto(internal *contactservice.ContactList) *impb.ContactList {
	cl := new(impb.ContactList)
	{
		cl.Next = internal.GetNext()
		cl.Page = internal.GetPage()
		cl.Size = internal.GetSize()
		cl.Items = make([]*impb.Contact, 0, len(internal.GetContacts()))
	}

	for _, c := range internal.GetContacts() {
		cl.Items = append(cl.Items, &impb.Contact{
			Iss:       c.GetIssId(),
			AppId:     c.GetAppId(),
			Type:      c.GetType(),
			Name:      c.GetName(),
			Username:  c.GetUsername(),
			Metadata:  c.GetMetadata(),
			CreatedAt: c.GetCreatedAt(),
			UpdatedAt: c.GetUpdatedAt(),
			Sub:       c.GetSubject(),
			IsBot:     c.GetIsBot(),
		})
	}

	return cl
}
