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

// MapSearchMessagesRequestToDTO maps a SearchMessagesRequest to a SearchMessagesRequest DTO.
func MapSearchMessagesRequestToDTO(req *pb.SearchMessagesRequest) *dto.SearchMessagesRequest {
	var cursor *dto.HistoryMessageCursor
	if req.Cursor != nil {
		cursor = &dto.HistoryMessageCursor{
			ID:     req.Cursor.Id,
			Before: req.Cursor.Before,
		}
	}

	return &dto.SearchMessagesRequest{
		Fields:    req.GetFields(),
		Term:      req.GetQ(),
		ThreadID:  req.GetThreadId(),
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
		Items:         toProtoMessages(res.Messages),
		NextCursor:    toProtoCursor(res.NextCursor),
		PrevCursor:    toProtoCursor(res.PrevCursor),
		LastUpdateSeq: res.LastUpdateSeq,
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
			Id:              m.ID,
			ThreadId:        m.ThreadID,
			Sender:          toProtoMessageSender(m.Sender),
			Type:            m.Type,
			Body:            m.Body,
			Metadata:        md,
			CreatedAt:       m.CreatedAt,
			EditedAt:        m.UpdatedAt,
			Seq:             m.Seq,
			Documents:       toProtoDocuments(m.Documents),
			Images:          toProtoImages(m.Images),
			Contact:         m.Contact,
			Location:        m.Location,
			Interactive:     m.Interactive,
			System:          m.System,
			ReactedMetadata: toProtoReactedMetadta(m.ReactedMetadata),
			ReplyTo:         toProtoReplyTo(m.ReplyTo),
			ForwardOrigin:   m.ForwardOrigin,
			Reactions:       m.Reactions,
			Deleted:         m.Deleted,
			DeletedAt:       m.DeletedAt,
			DeletedBy:       toProtoMessageSender(m.DeletedBy),
			RevisionCount:   m.RevisionCount,
		}
	}

	return protoMsgs
}

func MapGetMessageRevisionsRequestToDTO(req *pb.GetMessageRevisionsRequest) *dto.GetMessageRevisionsRequest {
	return &dto.GetMessageRevisionsRequest{
		MessageID: req.GetMessageId(),
	}
}

func MapToGetMessageRevisionsProto(revisions []*dto.MessageRevision) *pb.GetMessageRevisionsResponse {
	items := make([]*pb.MessageRevision, 0, len(revisions))
	for _, r := range revisions {
		items = append(items, &pb.MessageRevision{
			Version:   r.Version,
			Action:    r.Action,
			Body:      r.Body,
			ChangedBy: toProtoMessageSender(r.ChangedBy),
			ChangedAt: r.ChangedAt,
		})
	}

	return &pb.GetMessageRevisionsResponse{Items: items}
}

func MapToGetUpdatesProto(res *dto.GetUpdatesResponse) *pb.GetUpdatesResponse {
	if res == nil {
		return nil
	}

	threads := make([]*pb.ThreadUpdates, 0, len(res.Threads))
	for _, t := range res.Threads {
		threads = append(threads, toProtoThreadUpdates(t))
	}

	return &pb.GetUpdatesResponse{Cursor: res.Cursor, Resync: res.Resync, Threads: threads}
}

func toProtoThreadUpdates(t *dto.ThreadUpdates) *pb.ThreadUpdates {
	changes := make([]*pb.ThreadMemberChange, 0, len(t.MemberChanges))
	for _, mc := range t.MemberChanges {
		changes = append(changes, &pb.ThreadMemberChange{
			Member: toProtoMessageSender(mc.Member),
			Action: pb.ThreadMemberChangeAction(mc.Action),
			By:     toProtoMessageSender(mc.By),
		})
	}

	readStates := make([]*pb.MemberReadState, 0, len(t.ReadStates))
	for _, rs := range t.ReadStates {
		readStates = append(readStates, &pb.MemberReadState{
			MemberId:         rs.MemberID,
			DeliveredUpToSeq: rs.DeliveredUpToSeq,
			ReadUpToSeq:      rs.ReadUpToSeq,
			Member:           toProtoMessageSender(rs.Member),
		})
	}

	out := &pb.ThreadUpdates{
		ThreadId:          t.ThreadID,
		Cursor:            t.Cursor,
		Left:              t.Left,
		UnreadCount:       t.UnreadCount,
		Dialog:            t.Dialog,
		Messages:          toProtoUpdatedMessages(t.Messages),
		DeletedMessageIds: t.DeletedMessageIDs,
		MemberChanges:     changes,
		ReadStates:        readStates,
	}

	if t.TopMessage != nil {
		out.TopMessage = toProtoUpdatedMessages([]*dto.HistoryMessage{t.TopMessage})[0]
	}

	return out
}

// toProtoUpdatedMessages reuses history's message mapping and keeps only what a UI renders.
func toProtoUpdatedMessages(messages []*dto.HistoryMessage) []*pb.UpdatedMessage {
	out := make([]*pb.UpdatedMessage, 0, len(messages))
	for _, h := range toProtoMessages(messages) {
		out = append(out, &pb.UpdatedMessage{
			Id:            h.GetId(),
			Seq:           h.GetSeq(),
			Sender:        h.GetSender(),
			Type:          h.GetType(),
			Body:          h.GetBody(),
			Metadata:      h.GetMetadata(),
			CreatedAt:     h.GetCreatedAt(),
			EditedAt:      h.GetEditedAt(),
			ReplyTo:       h.GetReplyTo(),
			ForwardOrigin: h.GetForwardOrigin(),
			Documents:     h.GetDocuments(),
			Images:        h.GetImages(),
			Location:      h.GetLocation(),
			Contact:       h.GetContact(),
			Interactive:   h.GetInteractive(),
			System:        h.GetSystem(),
			Reactions:     h.GetReactions(),
		})
	}

	return out
}

func toProtoReplyTo(replyTo *dto.HistoryReplyTo) *pb.ReplyToMessage {
	if replyTo == nil {
		return nil
	}

	return &pb.ReplyToMessage{
		MessageId:      replyTo.MessageID,
		Sender:         toProtoMessageSender(replyTo.Sender),
		SenderId:       replyTo.SenderID,
		Type:           replyTo.Type,
		Body:           replyTo.Body,
		CreatedAt:      replyTo.CreatedAt,
		AttachmentKind: replyTo.AttachmentKind,
		AttachmentName: replyTo.AttachmentName,
		AttachmentMime: replyTo.AttachmentMime,

		AttachmentAddress: replyTo.AttachmentAddress,
		IsDeleted:         replyTo.IsDeleted,
	}
}

func toProtoReactedMetadta(reactedMetadata *dto.ApiInteractiveCallbackWrapper) *pb.InteractiveCallback {
	if reactedMetadata == nil {
		return nil
	}

	return &pb.InteractiveCallback{
		ReactedBy:    reactedMetadata.GetReactedBy(),
		InReplyTo:    reactedMetadata.GetInReplyTo(),
		ButtonCode:   reactedMetadata.GetButtonCode(),
		CallbackData: reactedMetadata.GetCallbackData(),
		ReactedAt:    reactedMetadata.GetReactedAt(),
	}
}

// toProtoDocuments maps a slice of HistoryDocumentDTOs to a slice of Documents.
func toProtoDocuments(docs []dto.HistoryDocument) []*pb.Document {
	res := make([]*pb.Document, len(docs))
	for i, d := range docs {
		res[i] = &pb.Document{
			Id:        d.FileID,
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

// MapSearchLeftThreadsMessageHistoryRequestToDTO maps a SearchLeftThreadsMessageHistoryRequest to its DTO form.
func MapSearchLeftThreadsMessageHistoryRequestToDTO(req *pb.SearchLeftThreadsMessageHistoryRequest) *dto.SearchLeftThreadsMessageHistoryRequest {
	var cursor *dto.HistoryMessageCursor
	if req.Cursor != nil {
		cursor = &dto.HistoryMessageCursor{
			ID:     req.Cursor.Id,
			Before: req.Cursor.Before,
		}
	}

	return &dto.SearchLeftThreadsMessageHistoryRequest{
		Fields:     req.GetFields(),
		ThreadID:   req.GetThreadId(),
		SenderIDs:  req.GetSenderIds(),
		Types:      req.GetTypes(),
		PeriodFrom: req.GetPeriodFrom(),
		PeriodTo:   req.GetPeriodTo(),
		Cursor:     cursor,
		Size:       req.GetSize(),
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
