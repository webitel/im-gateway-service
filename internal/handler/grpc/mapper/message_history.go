package mapper

import (
	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"google.golang.org/protobuf/types/known/structpb"
)

// MapSearchMessageHistoryRequestToDTO maps a SearchMessageHistoryRequest to a SearchMessageHistoryRequestDTO.
func MapSearchMessageHistoryRequestToDTO(req *pb.SearchMessageHistoryRequest) *dto.SearchMessageHistoryRequest {
	var cursor *dto.HistoryMessageCursor
	if req.Cursor != nil {
		cursor = &dto.HistoryMessageCursor{
			ID:     req.Cursor.Id,
			Before: req.Cursor.Before,
		}
	}

	return &dto.SearchMessageHistoryRequest{
		Fields:    req.GetFields(),
		IDs:       req.GetIds(),
		ThreadIDs: []string{req.GetThreadId()},
		SenderIDs: req.GetSenderIds(),
		Types:     req.GetTypes(),
		Cursor:    cursor,
		Size:      req.GetSize(),
	}
}

// MapToSearchHistoryProto maps a SearchMessageHistoryResponseDTO to a SearchMessageHistoryResponse.
func MapToSearchHistoryProto(res *dto.SearchMessageHistoryResponse) *pb.SearchMessageHistoryResponse {
	if res == nil {
		return nil
	}

	return &pb.SearchMessageHistoryResponse{
		Items:      toProtoMessages(res.Messages),
		NextCursor: toProtoCursor(res.NextCursor),
		PrevCursor: toProtoCursor(res.PrevCursor),
	}
}

// toProtoMessages maps a slice of HistoryMessageDTOs to a slice of HistoryMessages.
func toProtoMessages(messages []*dto.HistoryMessage) []*pb.HistoryMessage {
	if len(messages) == 0 {
		return nil
	}

	protoMsgs := make([]*pb.HistoryMessage, len(messages))
	for i, m := range messages {
		md, err := structpb.NewStruct(m.Metadata)
		if err != nil {
			return nil
		}

		protoMsgs[i] = &pb.HistoryMessage{
			Id:        m.ID,
			ThreadId:  m.ThreadID,
			Sender:    toProtoMessageSender(m.Sender),
			Type:      m.Type,
			Body:      m.Body,
			Metadata:  md,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			Documents: toProtoDocuments(m.Documents),
			Images:    toProtoImages(m.Images),
		}
	}
	return protoMsgs
}

// toProtoDocuments maps a slice of HistoryDocumentDTOs to a slice of Documents.
func toProtoDocuments(docs []dto.HistoryDocument) []*pb.Document {
	res := make([]*pb.Document, len(docs))
	for i, d := range docs {
		res[i] = &pb.Document{
			Id:        d.ID,
			MessageId: d.MessageID,
			FileId:    d.FileID,
			Name:      d.Name,
			Mime:      d.Mime,
			Size:      d.Size,
			CreatedAt: d.CreatedAt,
			Url:       d.URL,
		}
	}
	return res
}

// toProtoImages maps a slice of HistoryImageDTOs to a slice of Images.
func toProtoImages(imgs []dto.HistoryImage) []*pb.Image {
	res := make([]*pb.Image, len(imgs))
	for i, img := range imgs {
		res[i] = &pb.Image{
			Id:        img.ID,
			MessageId: img.MessageID,
			FileId:    img.FileID,
			Mime:      img.Mime,
			Width:     img.Width,
			Height:    img.Height,
			CreatedAt: img.CreatedAt,
			Url:       img.URL,
		}
	}
	return res
}

// toProtoCursor maps a HistoryMessageCursor to a HistoryMessageCursor.
func toProtoCursor(c *dto.HistoryMessageCursor) *pb.HistoryMessageCursorResponse {
	if c == nil {
		return nil
	}
	return &pb.HistoryMessageCursorResponse{
		Id: c.ID,
	}
}

// toProtoMessageSender maps a MessageSender to a MessageSender.
func toProtoMessageSender(ms *dto.MessageSender) *pb.ThreadMember {
	if ms == nil {
		return nil
	}

	return &pb.ThreadMember{
		Contact: &pb.Contact{
			Sub:      ms.Sub,
			Iss:      ms.Iss,
			Type:     ms.Type,
			Name:     ms.Name,
			IsBot:    ms.IsBot,
			Username: ms.Username,
		},
		Id:   ms.MemberID,
		Role: pb.ThreadRole(ms.Role),
	}
}
