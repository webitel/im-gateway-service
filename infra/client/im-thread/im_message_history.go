package imthread

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
)

type (
	MessageHistoryClient struct {
		logger *slog.Logger

		rpc *rpc.Client[threadv1.MessageHistoryClient]
	}
)

// NewMessageHistoryClient initializes a resilient gRPC client for the Message History service.
//
// Args:
//  - logger: logger for the client
//  - discovery: discovery provider for the Message History service
//  - tls: TLS configuration for the gRPC connection
//
// Returns:
//  - *MessageHistoryClient: a new instance of the MessageHistoryClient
//  - error: any error encountered during initialization
func NewMessageHistoryClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*MessageHistoryClient, error) {
	log := logger.With(slog.String("component", "im-message-history-client"))
	
	factory := func(conn *grpc.ClientConn) threadv1.MessageHistoryClient {
		return threadv1.NewMessageHistoryClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		log.Error("initialization failed", slog.Any("error", err))
		return nil, fmt.Errorf("[im-message-history-client] initialization failed: %w", err)
	}

	return &MessageHistoryClient{logger: log, rpc: c}, err
}

// Search performs a search for messages in the message history given a search query.
//
// Args:
//  - ctx: context of the request
//  - searchQuery: search query for the message history
//
// Returns:
//  - *dto.SearchMessageHistoryResponse: search result
//  - error: any error encountered during the search operation
func (c *MessageHistoryClient) Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error) {
	log := c.logger.With(
		slog.Int("domain_id", int(searchQuery.DomainID)),
		slog.Uint64("size", uint64(searchQuery.Size)),
		slog.Any("thread_ids", searchQuery.ThreadIDs),
		slog.Any("cursor", searchQuery.Cursor),
	)
	
	var cursor *threadv1.HistoryMessageCursor
	if searchQuery.Cursor != nil {
		cursor = &threadv1.HistoryMessageCursor{
			Id: searchQuery.Cursor.ID,
			CreatedAt: searchQuery.Cursor.CreatedAt,
			Direction: searchQuery.Cursor.Direction,
		}
	}
	
	req := &threadv1.SearchMessageHistoryRequest{
		Fields:      searchQuery.Fields,
		Ids:         searchQuery.IDs,
		ThreadIds:   searchQuery.ThreadIDs,
		SenderIds:   searchQuery.SenderIDs,
		ReceiverIds: searchQuery.ReceiverIDs,
		Types:       searchQuery.Types,
		DomainId:    searchQuery.DomainID,
		Cursor:      cursor,
		Size:        searchQuery.Size,
	}

	var (
		response *threadv1.SearchMessageHistoryResponse
		err error		
	)
	err = c.rpc.Execute(ctx, func(mhc threadv1.MessageHistoryClient) error {
		response, err = mhc.SearchThreadMessagesHistory(ctx, req)
		return err
	})

	if err != nil {
		log.Error("failed to search message history",
			slog.Any("error", err),
			slog.Any("request", searchQuery),
		)
		return nil, err
	}

	respDto := ToSearchHistoryResponseDTO(response) 

	return respDto, nil
}

// Close gracefully shuts down the underlying gRPC connection pool.
// If the client is nil, the method does nothing and returns nil.
//
// Returns:
//  - error: any error encountered during shutdown
func (c *MessageHistoryClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}

// ToSearchHistoryResponseDTO maps a SearchMessageHistoryResponse to a SearchMessageHistoryResponseDTO.
//
// Args:
//  - resp: The SearchMessageHistoryResponse to be mapped.
//
// Returns:
//  - A SearchMessageHistoryResponseDTO with the given messages, next cursor, next, from, and paging.
func ToSearchHistoryResponseDTO(resp *threadv1.SearchMessageHistoryResponse) *dto.SearchMessageHistoryResponse {
	if resp == nil {
		return &dto.SearchMessageHistoryResponse{}
	}

	return &dto.SearchMessageHistoryResponse{
		Messages:   mapMessages(resp.Messages),
		NextCursor: mapCursor(resp.NextCursor),
		Next:       resp.Next,
		From:       resp.From,
		Paging: dto.Paging{
			Cursors: dto.Cursors{
				After:  mapCursor(resp.Paging.GetCursors().GetAfter()),
				Before: mapCursor(resp.Paging.GetCursors().GetBefore()),
			},
		},
	}
}

// mapMessages maps a slice of HistoryMessageDTOs to a slice of HistoryMessages.
//
// Args:
//  - pbMsgs: The slice of HistoryMessageDTOs to be mapped.
//
// Returns:
//  - A slice of HistoryMessages with the given IDs, thread IDs, sender IDs, receiver IDs, types, bodies, metadata, created at times, updated at times, documents, and images.
func mapMessages(pbMsgs []*threadv1.HistoryMessage) []dto.HistoryMessage {
	if len(pbMsgs) == 0 {
		return []dto.HistoryMessage{}
	}

	res := make([]dto.HistoryMessage, len(pbMsgs))
	for i, m := range pbMsgs {
		res[i] = dto.HistoryMessage{
			ID:         m.Id,
			ThreadID:   m.ThreadId,
			SenderID:   m.SenderId,
			ReceiverID: m.ReceiverId,
			Type:       m.Type,
			Body:       m.Body,
			Metadata:   unmarshalMetadata(m.Metadata),
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
			Documents:  mapDocuments(m.Documents),
			Images:     mapImages(m.Images),
		}
	}
	return res
}

// mapDocuments maps a slice of DocumentDTOs to a slice of HistoryDocuments.
//
// Args:
//  - pbDocs: The slice of DocumentDTOs to be mapped.
//
// Returns:
//  - A slice of HistoryDocuments with the given IDs, message IDs, file IDs, names, mime types, sizes, created at times, and URLs.
func mapDocuments(pbDocs []*threadv1.Document) []dto.HistoryDocument {
	res := make([]dto.HistoryDocument, len(pbDocs))
	for i, d := range pbDocs {
		res[i] = dto.HistoryDocument{
			ID:        d.Id,
			MessageID: d.MessageId,
			FileID:    d.FileId,
			Name:      d.Name,
			Mime:      d.Mime,
			Size:      d.Size,
			CreatedAt: d.CreatedAt,
			URL:       d.Url,
		}
	}
	return res
}

// mapImages maps a slice of ImageDTOs to a slice of HistoryImages.
//
// Args:
//  - pbImgs: The slice of ImageDTOs to be mapped.
//
// Returns:
//  - A slice of HistoryImages with the given IDs, message IDs, file IDs, mime types, widths, heights, created at times, and URLs.
func mapImages(pbImgs []*threadv1.Image) []dto.HistoryImage {
	res := make([]dto.HistoryImage, len(pbImgs))
	for i, img := range pbImgs {
		res[i] = dto.HistoryImage{
			ID:        img.Id,
			MessageID: img.MessageId,
			FileID:    img.FileId,
			Mime:      img.Mime,
			Width:     img.Width,
			Height:    img.Height,
			CreatedAt: img.CreatedAt,
			URL:       img.Url,
		}
	}
	return res
}

// mapCursor maps a HistoryMessageCursor from threadv1 to a HistoryMessageCursor from dto.
//
// Args:
//  - c: The HistoryMessageCursor to be mapped.
//
// Returns:
//  - A HistoryMessageCursor with the given created at time, ID, and direction. If c is nil, returns nil.
func mapCursor(c *threadv1.HistoryMessageCursor) *dto.HistoryMessageCursor {
	if c == nil {
		return nil
	}

	return &dto.HistoryMessageCursor{
		CreatedAt: c.CreatedAt,
		ID:        c.Id,
		Direction: c.Direction,
	}
}

// unmarshalMetadata unmarshals a map of string keys to Any values to a map of string keys to interface values.
//
// Args:
//  - pbMeta: The map of string keys to Any values to be unmarshaled.
//
// Returns:
//  - A map of string keys to interface values. The interface values are the JSON-unmarshalled versions of the Any values.
//  If an error occurs while unmarshaling a value, it is skipped.
func unmarshalMetadata(pbMeta map[string]*anypb.Any) map[string]interface{} {
	if len(pbMeta) == 0 {
		return nil
	}

	res := make(map[string]any, len(pbMeta))
	for k, v := range pbMeta {
		var temp any
		if err := json.Unmarshal(v.Value, &temp); err == nil {
			res[k] = temp
		} else {
			res[k] = v.Value
		}
	}
	return res
}