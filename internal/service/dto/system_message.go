package dto

import (
	"github.com/google/uuid"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"google.golang.org/protobuf/types/known/structpb"
)

type SendSystemMessageRequest struct {
	From     shared.Peer      `json:"from"`
	To       shared.Peer      `json:"to"`
	Type     string           `json:"type"`
	Body     string           `json:"body"`
	Metadata *structpb.Struct `json:"metadata"`
	DomainID int64            `json:"domain_id"`
	SendID   string           `json:"send_id"`
}

type SendSystemMessageResponse struct {
	To shared.Peer `json:"to"`
	ID uuid.UUID   `json:"id"`
}
