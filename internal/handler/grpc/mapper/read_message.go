package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapToReadMessageRequest(pb *impb.ReadMessageRequest) *dto.ReadMessageRequest {
	if pb == nil {
		return nil
	}
	return &dto.ReadMessageRequest{
		MessageID: pb.GetId(),
		ThreadID:  pb.GetThreadId(),
	}
}
