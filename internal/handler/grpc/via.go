package grpc

import (
	"context"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
)

type ViaServer struct {
	impb.UnimplementedViasServiceServer

	viaService service.Via
}

func newViaServer(viaService service.Via) *ViaServer {
	return &ViaServer{viaService: viaService}
}

func (server *ViaServer) Create(ctx context.Context, req *impb.ViasServiceCreateRequest) (*impb.ViasServiceCreateResponse, error) {
	response, err := server.viaService.Create(ctx, &contact.CreateViaRequest{
		ContactId:     req.GetContactId(),
		Via:           req.GetVia(),
		Disable:       req.GetDisable(),
		DisableReason: req.DisableReason,
		Metadata:      req.GetMetadata(),
	})

	if err != nil {
		return nil, err
	}

	return &impb.ViasServiceCreateResponse{
		ContactId:     response.GetContactId(),
		Via:           response.GetVia(),
		Disable:       response.GetDisable(),
		DisableReason: response.DisableReason,
		CreatedAt:     response.GetCreatedAt(),
		UpdatedAt:     response.GetUpdatedAt(),
		Metadata:      response.GetMetadata(),
	}, nil
}

func (server *ViaServer) PartialUpdate(ctx context.Context, req *impb.ViasServicePartialUpdateRequest) (*impb.ViasServicePartialUpdateResponse, error) {
	var disableReason *string
	if upd := req.GetUpdate(); upd != nil {
		disableReason = req.GetUpdate().DisableReason
	}

	response, err := server.viaService.PartialUpdate(ctx, &contact.PartialUpdateViaRequest{
		Update: &contact.UpdateViaRequest{
			ContactId:     req.GetUpdate().GetContactId(),
			Via:           req.GetUpdate().GetVia(),
			Disable:       req.GetUpdate().GetDisable(),
			DisableReason: disableReason,
			Metadata:      req.GetUpdate().GetMetadata(),
		},
		FieldMask: req.GetFieldMask(),
	})

	if err != nil {
		return nil, err
	}

	return &impb.ViasServicePartialUpdateResponse{
		ContactId:     response.GetContactId(),
		Via:           response.GetVia(),
		Disable:       response.GetDisable(),
		DisableReason: response.DisableReason,
		CreatedAt:     response.GetCreatedAt(),
		UpdatedAt:     response.GetUpdatedAt(),
		Metadata:      response.GetMetadata(),
	}, nil
}

func (server *ViaServer) Search(ctx context.Context, req *impb.ViasServiceSearchRequest) (*impb.ViasServiceSearchResponse, error) {
	response, err := server.viaService.Search(ctx, &contact.SearchViaRequest{
		Sort:       req.GetSort(),
		Size:       req.GetSize(),
		Page:       req.GetPage(),
		Fields:     req.GetFields(),
		ContactIds: req.GetContactIds(),
		Disabled:   req.Disabled,
		Vias:       req.GetVias(),
	})

	if err != nil {
		return nil, err
	}

	items := make([]*impb.Via, len(response.GetItems()))
	for i, item := range response.GetItems() {
		items[i] = &impb.Via{
			ContactId:     item.GetContactId(),
			Via:           item.GetVia(),
			Disable:       item.GetDisable(),
			DisableReason: item.DisableReason,
			CreatedAt:     item.GetCreatedAt(),
			UpdatedAt:     item.GetUpdatedAt(),
			Metadata:      item.GetMetadata(),
		}
	}

	return &impb.ViasServiceSearchResponse{
		Items: items,
		Next:  response.GetNext(),
		Page:  response.GetPage(),
	}, nil
}

func (server *ViaServer) Update(ctx context.Context, req *impb.ViasServiceUpdateRequest) (*impb.ViasServiceUpdateResponse, error) {
	response, err := server.viaService.Update(ctx, &contact.UpdateViaRequest{
		ContactId:     req.GetContactId(),
		Via:           req.GetVia(),
		Disable:       req.GetDisable(),
		DisableReason: req.DisableReason,
		Metadata:      req.GetMetadata(),
	})

	if err != nil {
		return nil, err
	}

	return &impb.ViasServiceUpdateResponse{
		ContactId:     response.GetContactId(),
		Via:           response.GetVia(),
		Disable:       response.GetDisable(),
		DisableReason: response.DisableReason,
		CreatedAt:     response.GetCreatedAt(),
		UpdatedAt:     response.GetUpdatedAt(),
		Metadata:      response.GetMetadata(),
	}, nil
}
