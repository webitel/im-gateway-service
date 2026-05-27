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

	return &impb.Contact{
		Iss:       out.GetIssId(),
		AppId:     out.GetAppId(),
		Type:      out.GetType(),
		Name:      out.GetName(),
		Username:  out.GetUsername(),
		Metadata:  out.GetMetadata(),
		CreatedAt: out.GetCreatedAt(),
		UpdatedAt: out.GetUpdatedAt(),
		Sub:       out.GetSubject(),
		IsBot:     out.GetIsBot(),
	}, nil
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
			Vias:      ConvertInternalViaToOut(c.GetVias()),
		})
	}

	return cl
}

func (c *ContactService) Locate(ctx context.Context, req *impb.LocateConatctRequest) (*impb.LocateContactResponse, error) {
	response, err := c.contacter.Locate(ctx, &contactservice.LocateContactRequest{
		Id:       req.GetId(),
		DomainId: req.GetDomainId(),
	})

	if err != nil {
		return nil, err
	}

	return &impb.LocateContactResponse{
		Item: &impb.Contact{
			Iss:       response.GetItem().GetIssId(),
			AppId:     response.GetItem().GetAppId(),
			Type:      response.GetItem().GetType(),
			Name:      response.GetItem().GetName(),
			Username:  response.GetItem().GetUsername(),
			Metadata:  response.GetItem().GetMetadata(),
			CreatedAt: response.GetItem().GetCreatedAt(),
			UpdatedAt: response.GetItem().GetUpdatedAt(),
			Sub:       response.GetItem().GetSubject(),
			IsBot:     response.GetItem().GetIsBot(),
			Vias:      ConvertInternalViaToOut(response.GetItem().GetVias()),
		},
	}, nil
}

func ConvertInternalViaToOut(items []*contactservice.Via) []*impb.Via {
	converted := make([]*impb.Via, len(items))

	for i, via := range items {
		converted[i] = &impb.Via{
			ContactId:     via.GetContactId(),
			Via:           via.GetVia(),
			Disable:       via.GetDisable(),
			DisableReason: via.DisableReason,
			CreatedAt:     via.GetCreatedAt(),
			UpdatedAt:     via.GetUpdatedAt(),
			Metadata:      via.GetMetadata(),
		}
	}

	return converted
}
