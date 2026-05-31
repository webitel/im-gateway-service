package imthread

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/gen/go/thread/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
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
//   - logger: logger for the client
//   - discovery: discovery provider for the Message History service
//   - tls: TLS configuration for the gRPC connection
//
// Returns:
//   - *MessageHistoryClient: a new instance of the MessageHistoryClient
//   - error: any error encountered during initialization
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
//   - ctx: context of the request
//   - searchQuery: search query for the message history
//
// Returns:
//   - *dto.SearchMessageHistoryResponse: search result
//   - error: any error encountered during the search operation
func (c *MessageHistoryClient) Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, []*threadv1.ThreadMember, error) {
	log := c.logger.With(
		slog.Int("domain_id", int(searchQuery.DomainID)),
		slog.Uint64("size", uint64(searchQuery.Size)),
		slog.Any("thread_ids", searchQuery.ThreadIDs),
		slog.Any("cursor", searchQuery.Cursor),
	)

	var cursor *threadv1.HistoryMessageCursorRequest
	if searchQuery.Cursor != nil {
		cursor = &threadv1.HistoryMessageCursorRequest{
			Id:     searchQuery.Cursor.ID,
			Before: searchQuery.Cursor.Before,
		}
	}

	req := &threadv1.SearchMessageHistoryRequest{
		Fields:    searchQuery.Fields,
		Ids:       searchQuery.IDs,
		ThreadId:  searchQuery.ThreadIDs[0],
		SenderIds: searchQuery.SenderIDs,
		Types:     searchQuery.Types,
		DomainId:  searchQuery.DomainID,
		Cursor:    cursor,
		Size:      searchQuery.Size,
	}

	var (
		response *threadv1.SearchMessageHistoryResponse
		err      error
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
		return nil, nil, err
	}

	respDto := ToSearchHistoryResponseDTO(response)

	return respDto, response.GetFrom(), nil
}

// SearchLeftThreads retrieves message history covering the user's closed
// membership periods within a thread.
//
// Args:
//   - ctx: context of the request
//   - query: search query for the left-threads message history
//
// Returns:
//   - *dto.SearchMessageHistoryResponse: flat search result
//   - []*threadv1.ThreadMember: internal participants used for sender enrichment
//   - error: any error encountered during the search operation
func (c *MessageHistoryClient) SearchLeftThreads(ctx context.Context, query *dto.SearchLeftThreadsMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, []*threadv1.ThreadMember, error) {
	log := c.logger.With(
		slog.Int("domain_id", int(query.DomainID)),
		slog.Uint64("size", uint64(query.Size)),
		slog.String("thread_id", query.ThreadID),
		slog.Any("cursor", query.Cursor),
	)

	var cursor *threadv1.HistoryMessageCursorRequest
	if query.Cursor != nil {
		cursor = &threadv1.HistoryMessageCursorRequest{
			Id:     query.Cursor.ID,
			Before: query.Cursor.Before,
		}
	}

	req := &threadv1.SearchLeftThreadsMessageHistoryRequest{
		Fields:     query.Fields,
		ThreadId:   query.ThreadID,
		DomainId:   query.DomainID,
		SenderIds:  query.SenderIDs,
		Types:      query.Types,
		PeriodFrom: query.PeriodFrom,
		PeriodTo:   query.PeriodTo,
		Cursor:     cursor,
		Size:       query.Size,
	}

	var (
		response *threadv1.SearchMessageHistoryResponse
		err      error
	)
	err = c.rpc.Execute(ctx, func(mhc threadv1.MessageHistoryClient) error {
		response, err = mhc.SearchLeftThreadsMessageHistory(ctx, req)
		return err
	})
	if err != nil {
		log.Error("failed to search left threads message history",
			slog.Any("error", err),
			slog.Any("request", query),
		)
		return nil, nil, err
	}

	return ToSearchHistoryResponseDTO(response), response.GetFrom(), nil
}

// Close gracefully shuts down the underlying gRPC connection pool.
// If the client is nil, the method does nothing and returns nil.
//
// Returns:
//   - error: any error encountered during shutdown
func (c *MessageHistoryClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}

// ToSearchHistoryResponseDTO maps a SearchMessageHistoryResponse to a SearchMessageHistoryResponseDTO.
//
// Args:
//   - resp: The SearchMessageHistoryResponse to be mapped.
//
// Returns:
//   - A SearchMessageHistoryResponseDTO with the given messages, next cursor, next, from, and paging.
func ToSearchHistoryResponseDTO(resp *threadv1.SearchMessageHistoryResponse) *dto.SearchMessageHistoryResponse {
	if resp == nil {
		return &dto.SearchMessageHistoryResponse{}
	}

	return &dto.SearchMessageHistoryResponse{
		Messages:   mapMessages(resp.Items),
		NextCursor: mapCursor(resp.NextCursor),
		PrevCursor: mapCursor(resp.PrevCursor),
	}
}

// mapMessages maps a slice of HistoryMessageDTOs to a slice of HistoryMessages.
//
// Args:
//   - pbMsgs: The slice of HistoryMessageDTOs to be mapped.
//
// Returns:
//   - A slice of HistoryMessages with the given IDs, thread IDs, sender IDs, receiver IDs, types, bodies, metadata, created at times, updated at times, documents, and images.
func mapMessages(pbMsgs []*threadv1.HistoryMessage) []*dto.HistoryMessage {
	if len(pbMsgs) == 0 {
		return []*dto.HistoryMessage{}
	}

	res := make([]*dto.HistoryMessage, len(pbMsgs))
	for i, m := range pbMsgs {
		res[i] = &dto.HistoryMessage{
			ID:          m.Id,
			ThreadID:    m.ThreadId,
			SenderID:    m.SenderId,
			Type:        m.Type,
			Body:        m.Body,
			Metadata:    m.Metadata.AsMap(),
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
			Documents:   mapDocuments(m.Documents),
			Images:      mapImages(m.Images),
			Location:    MapLocation(m.Location),
			Contact:     MapContact(m.Contact),
			Interactive: MapInteractive(m.Interactive),
		}
	}
	return res
}

func MapInteractive(interactive *threadv1.Interactive) *api.Interactive {
	if interactive == nil {
		return nil
	}

	converted, _ := mapper.Convert(interactive, new(api.Interactive))

	return converted
}

func MapLocation(location *threadv1.Location) *api.MessageLocation {
	if location == nil {
		return nil
	}

	return &api.MessageLocation{
		Longitude: location.GetLongitude(),
		Latitude:  location.GetLatitude(),
		Address:   location.Address,
		Name:      location.Name,
	}
}

func MapContact(contact *thread.Contact) *api.MessageContact {
	if contact == nil {
		return nil
	}

	return &api.MessageContact{
		Name:  contact.Name,
		Email: contact.Email,
		Phone: contact.PhoneNumber,
	}
}

// mapDocuments maps a slice of DocumentDTOs to a slice of HistoryDocuments.
//
// Args:
//   - pbDocs: The slice of DocumentDTOs to be mapped.
//
// Returns:
//   - A slice of HistoryDocuments with the given IDs, message IDs, file IDs, names, mime types, sizes, created at times, and URLs.
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
//   - pbImgs: The slice of ImageDTOs to be mapped.
//
// Returns:
//   - A slice of HistoryImages with the given IDs, message IDs, file IDs, mime types, widths, heights, created at times, and URLs.
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
//   - c: The HistoryMessageCursor to be mapped.
//
// Returns:
//   - A HistoryMessageCursor with the given created at time, ID, and direction. If c is nil, returns nil.
func mapCursor(c *threadv1.HistoryMessageCursorResponse) *dto.HistoryMessageCursor {
	if c == nil {
		return nil
	}

	return &dto.HistoryMessageCursor{
		ID: c.Id,
	}
}

// unmarshalMetadata unmarshals a map of string keys to Any values to a map of string keys to interface values.
//
// Args:
//   - pbMeta: The map of string keys to Any values to be unmarshaled.
//
// Returns:
//   - A map of string keys to interface values. The interface values are the JSON-unmarshalled versions of the Any values.
//     If an error occurs while unmarshaling a value, it is skipped.
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
