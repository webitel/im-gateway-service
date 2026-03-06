package mapper

import (
	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// MapSearchMessageHistoryRequestToDTO maps a SearchMessageHistoryRequest to a SearchMessageHistoryRequestDTO.
func MapSearchMessageHistoryRequestToDTO(req *pb.SearchMessageHistoryRequest) *dto.SearchMessageHistoryRequest {
	var cursor *dto.HistoryMessageCursor
	if req.Cursor != nil {
		cursor = &dto.HistoryMessageCursor{
			CreatedAt: req.Cursor.CreatedAt,
			ID:        req.Cursor.Id,
			Direction: req.Cursor.Direction,
		}
	}
	
	return &dto.SearchMessageHistoryRequest{
		Fields:      req.GetFields(),
		IDs:         req.GetIds(),
		ThreadIDs:   []string{req.GetThreadId()},
		SenderIDs:   req.GetSenderIds(),
		Types:       req.GetTypes(),
		Cursor:      cursor,
		Size:        req.GetSize(),
	}
}

// MapToSearchHistoryProto maps a SearchMessageHistoryResponseDTO to a SearchMessageHistoryResponse.
func MapToSearchHistoryProto(res *dto.SearchMessageHistoryResponse) *pb.SearchMessageHistoryResponse {
	if res == nil {
		return nil
	}

	return &pb.SearchMessageHistoryResponse{
		Messages:    toProtoMessages(res.Messages),
		NextCursor:  toProtoCursor(res.NextCursor),
		Next:        res.Next,
		From:        toProtoMessageSenderList(res.MessageSenders),
		Paging: &pb.Paging{
			Cursors: &pb.Cursors{
				After:  toProtoCursor(res.Paging.Cursors.After),
				Before: toProtoCursor(res.Paging.Cursors.Before),
			},
		},
	}
}

// toProtoMessages maps a slice of HistoryMessageDTOs to a slice of HistoryMessages.
func toProtoMessages(messages []*dto.HistoryMessage) []*pb.HistoryMessage {
	if len(messages) == 0 {
		return nil
	}

	protoMsgs := make([]*pb.HistoryMessage, len(messages))
	for i, m := range messages {
		md, err := toAnyMap(m.Metadata)
		if err != nil {
			return nil
		}
		
		protoMsgs[i] = &pb.HistoryMessage{
			Id:         m.ID,
			ThreadId:  m.ThreadID,
			Sender:  toProtoMessageSender(m.Sender),
			Type:       m.Type,
			Body:       m.Body,
			Metadata:   md,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			Documents:  toProtoDocuments(m.Documents),
			Images:     toProtoImages(m.Images),
		}
	}
	return protoMsgs
}

// toProtoDocuments maps a slice of HistoryDocumentDTOs to a slice of Documents.
func toProtoDocuments(docs []dto.HistoryDocument) []*pb.Document {
	res := make([]*pb.Document, len(docs))
	for i, d := range docs {
		res[i] = &pb.Document{
			Id:         d.ID,
			MessageId: d.MessageID,
			FileId:    d.FileID,
			Name:       d.Name,
			Mime:       d.Mime,
			Size:       d.Size,
			CreatedAt: d.CreatedAt,
			Url:        d.URL,
		}
	}
	return res
}

// toProtoImages maps a slice of HistoryImageDTOs to a slice of Images.
func toProtoImages(imgs []dto.HistoryImage) []*pb.Image {
	res := make([]*pb.Image, len(imgs))
	for i, img := range imgs {
		res[i] = &pb.Image{
			Id:         img.ID,
			MessageId: img.MessageID,
			FileId:    img.FileID,
			Mime:       img.Mime,
			Width:      img.Width,
			Height:     img.Height,
			CreatedAt: img.CreatedAt,
			Url:        img.URL,
		}
	}
	return res
}

// toProtoCursor maps a HistoryMessageCursor to a HistoryMessageCursor.
func toProtoCursor(c *dto.HistoryMessageCursor) *pb.HistoryMessageCursor {
	if c == nil {
		return nil
	}
	return &pb.HistoryMessageCursor{
		Id:        c.ID,
		CreatedAt: c.CreatedAt,
		Direction: c.Direction,
	}
}

// toProtoMetadata maps a map of string keys to interface values to a map of string keys to Any values.
func toAnyMap(src map[string]any) (map[string]*anypb.Any, error) {
	dst := make(map[string]*anypb.Any, len(src))

	for k, v := range src {
		spb, _ := structpb.NewValue(v)
		a, err := anypb.New(spb)
		if err != nil {
			return nil, err
		}
		dst[k] = a
	}

	return dst, nil
}

// toProtoMessageSender maps a MessageSender to a MessageSender.
func toProtoMessageSender(ms *dto.MessageSender) *pb.MessageParticipant {
	if ms == nil {
		return nil
	}
	
	return &pb.MessageParticipant{
		Subject: ms.Subject,
		Issuer:  ms.Issuer,
		Type:    ms.Type,
		Username: ms.UserName,
		IsBot: ms.IsBot,
	}
}

// toProtoMessageSenderList maps a slice of MessageSendDTOs to a slice of MessageSenders.
func toProtoMessageSenderList(msList []*dto.MessageSender) []*pb.MessageParticipant {
	var (
		pbMessageSenderList = make([]*pb.MessageParticipant, 0, len(msList))
	)
	
	for _, ms := range msList {
		pbMessageSenderList = append(pbMessageSenderList, toProtoMessageSender(ms))
	}

	return pbMessageSenderList
}