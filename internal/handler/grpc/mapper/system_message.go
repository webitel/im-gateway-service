package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapPbToSystemMessageRequest(in *impb.SendSystemMessageRequest) *dto.SendSystemMessageRequest {
	if in == nil {
		return nil
	}
	return &dto.SendSystemMessageRequest{
		To:       MapPeerFromProto(in.GetTo()),
		Type:     in.GetType(),
		Body:     in.GetBody(),
		Metadata: in.GetMetadata(),
		SendID:   in.GetSendId(),
	}
}

func MapToSendSystemMessageResponse(out *dto.SendSystemMessageResponse) *impb.SendMessageResponse {
	if out == nil {
		return nil
	}
	return &impb.SendMessageResponse{
		To: MapPeerToProto(out.To),
		Id: out.ID.String(),
	}
}
