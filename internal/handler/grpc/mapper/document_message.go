package mapper

import (
	"strconv"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// goverter:converter
// goverter:output:file ./generated/document_message.go
// goverter:matchIgnoreCase
// goverter:extend MapPeerFromProto
// goverter:extend StringToInt64
type MessageMapper interface {
	// goverter:useZeroValueOnPointerInconsistency
	// goverter:map FileName Name
	// goverter:map SizeBytes Size
	MapToDocument(in *impb.DocumentInput) *dto.Document
	// goverter:useZeroValueOnPointerInconsistency
	// goverter:ignoreUnexported
	// goverter:ignore From
	// goverter:ignore DomainID
	MapToSendDocumentRequest(in *impb.SendDocumentRequest) *dto.SendDocumentRequest
	MapToSendDocumentResponse(out *dto.SendDocumentResponse) *impb.SendDocumentResponse
}

func StringToInt64(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}
