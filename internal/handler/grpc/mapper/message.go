package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// MapPbToEditMessageRequest converts the gateway EditMessageRequest proto into the service DTO.
func MapPbToEditMessageRequest(in *impb.EditMessageRequest) *dto.EditMessageRequest {
	if in == nil {
		return nil
	}

	return &dto.EditMessageRequest{
		ID:   in.GetId(),
		Body: in.GetBody(),
	}
}

// MapToEditMessageResponse converts the service DTO into the gateway EditMessageResponse proto.
func MapToEditMessageResponse(out *dto.EditMessageResponse) *impb.EditMessageResponse {
	if out == nil {
		return nil
	}

	return &impb.EditMessageResponse{
		Id:       out.ID,
		EditedAt: out.EditedAt,
	}
}
