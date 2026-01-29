package mapper

import (
	"strconv"

	"github.com/google/uuid"
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/infra/server/grpc/interceptors"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapToSendImageRequest(in *impb.SendImageRequest, from *interceptors.Identity) *dto.SendImageRequest {
	if in == nil {
		return nil
	}

	var imgReq dto.ImageRequest
	if pbImg := in.GetImage(); pbImg != nil {
		imgReq.Body = pbImg.GetBody()
		imgReq.Images = make([]*dto.Image, 0, len(pbImg.GetImages()))

		for _, img := range pbImg.GetImages() {
			id, _ := strconv.ParseInt(img.GetId(), 10, 64)
			imgReq.Images = append(imgReq.Images, &dto.Image{
				ID:       id,
				Name:     img.GetName(),
				URL:      img.GetLink(),
				MimeType: img.GetMimeType(),
			})
		}
	}
	fromID, _ := uuid.Parse(from.ContactID)
	return &dto.SendImageRequest{
		To: MapPeerFromProto(in.GetTo()),
		From: shared.Peer{
			ID:   fromID,
			Type: shared.PeerContact,
		},
		Image:    imgReq,
		DomainID: from.DomainID,
	}
}

func MapToSendImageResponse(out *dto.SendImageResponse) *impb.SendImageResponse {
	if out == nil {
		return nil
	}
	return &impb.SendImageResponse{
		Id: out.ID.String(),
		To: MapPeerToProto(out.To),
	}
}
