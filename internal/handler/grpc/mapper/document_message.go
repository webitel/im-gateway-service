package mapper

import (
	"strconv"

	"github.com/google/uuid"
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/infra/server/grpc/interceptors"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapToSendDocumentRequest(in *impb.SendDocumentRequest, from *interceptors.Identity) *dto.SendDocumentRequest {
	if in == nil {
		return nil
	}

	var docReq dto.DocumentRequest
	if pbDoc := in.GetDocument(); pbDoc != nil {
		docReq.Body = pbDoc.GetBody()
		docReq.Documents = make([]*dto.Document, 0, len(pbDoc.GetDocuments()))

		for _, doc := range pbDoc.GetDocuments() {
			id, _ := strconv.ParseInt(doc.GetId(), 10, 64)
			docReq.Documents = append(docReq.Documents, &dto.Document{
				ID:       id,
				Name:     doc.GetFileName(),
				MimeType: doc.GetMimeType(),
				Size:     doc.GetSizeBytes(),
			})
		}
	}
	fromID, _ := uuid.Parse(from.ContactID)
	return &dto.SendDocumentRequest{
		To: MapPeerFromProto(in.GetTo()),
		From: shared.Peer{
			ID:   fromID,
			Type: shared.PeerContact,
		},
		Document: docReq,
		DomainID: from.DomainID,
	}
}

func MapToSendDocumentResponse(out *dto.SendDocumentResponse) *impb.SendDocumentResponse {
	if out == nil {
		return nil
	}
	return &impb.SendDocumentResponse{
		Id: out.ID.String(),
		To: MapPeerToProto(out.To),
	}
}
