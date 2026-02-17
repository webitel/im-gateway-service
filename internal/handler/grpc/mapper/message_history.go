package mapper

import (
	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// MapSearchMessageHistoryRequestToDTO maps a SearchMessageHistoryRequest to a SearchMessageHistoryRequestDTO.
//
// Args:
//  - req: The SearchMessageHistoryRequest to be mapped.
//
// Returns:
//  - A SearchMessageHistoryRequestDTO with the given fields, IDs, thread IDs, sender IDs, receiver IDs, types, cursor, and size.
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
		ThreadIDs:   req.GetThreadIds(),
		SenderIDs:   req.GetSenderIds(),
		Types:       req.GetTypes(),
		Cursor:      cursor,
		Size:        req.GetSize(),
	}
}

// MapToSearchHistoryProto maps a SearchMessageHistoryResponseDTO to a SearchMessageHistoryResponse.
//
// Args:
//  - res: The SearchMessageHistoryResponseDTO to be mapped.
//
// Returns:
//  - A SearchMessageHistoryResponse with the given messages, next cursor, next, from, and paging.
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
//
// Args:
//  - messages: The slice of HistoryMessageDTOs to be mapped.
//
// Returns:
//  - A slice of HistoryMessages with the given IDs, thread IDs, sender IDs, receiver IDs, types, bodies, metadata, created at times, updated at times, documents, and images.
func toProtoMessages(messages []dto.HistoryMessage) []*pb.HistoryMessage {
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
//
// Args:
//  - docs: The slice of HistoryDocumentDTOs to be mapped.
//
// Returns:
//  - A slice of Documents with the given IDs, message IDs, file IDs, names, mime types, sizes, created at times, and URLs.
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
//
// Args:
//  - imgs: The slice of HistoryImageDTOs to be mapped.
//
// Returns:
//  - A slice of Images with the given IDs, message IDs, file IDs, mime types, widths, heights, created at times, and URLs.
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
//
// Args:
//  - c: The HistoryMessageCursor to be mapped.
//
// Returns:
//  - A HistoryMessageCursor with the given ID, created at time, and direction.
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
//
// Args:
//  - meta: The map of string keys to interface values to be mapped.
//
// Returns:
//  - A map of string keys to Any values. The Any values are the JSON-marshalled versions of the interface values.
//
// If an error occurs while marshaling a value, it is skipped.
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
//
// Args:
//  - ms: The MessageSender to be mapped.
//
// Returns:
//  - A MessageSender with the given subject, issuer, and type.
func toProtoMessageSender(ms *dto.MessageSender) *pb.MessageParticipant {
	return &pb.MessageParticipant{
		Subject: ms.Subject,
		Issuer:  ms.Issuer,
		Type:    ms.Type,
	}
}

// toProtoMessageSenderList maps a slice of MessageSendDTOs to a slice of MessageSenders.
//
// Args:
//  - msList: The slice of MessageSendDTOs to be mapped.
//
// Returns:
//  - A slice of MessageSenders with the given subjects, issuers, and types.
func toProtoMessageSenderList(msList []*dto.MessageSender) []*pb.MessageParticipant {
	var (
		pbMessageSenderList = make([]*pb.MessageParticipant, 0, len(msList))
	)
	
	for _, ms := range msList {
		pbMessageSenderList = append(pbMessageSenderList, toProtoMessageSender(ms))
	}

	return pbMessageSenderList
}