package grpc

import (
	"context"
	"log/slog"

	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

type (
	MessageHistoryService struct {
		pb.UnimplementedMessageHistoryServer

		logger                 *slog.Logger
		messageHistorySearcher service.MessageHistorySearcher
	}
)

// NewMessageHistoryService creates a new instance of MessageHistoryService.
//
// Args:
//   - logger: logger for the service
//   - messageHistorySearcher: service for searching message history
//
// Returns:
//   - *MessageHistoryService: a new instance of MessageHistoryService
func NewMessageHistoryService(logger *slog.Logger, messageHistorySearcher service.MessageHistorySearcher) *MessageHistoryService {
	return &MessageHistoryService{
		logger:                 logger,
		messageHistorySearcher: messageHistorySearcher,
	}
}

// SearchThreadMessagesHistory performs a search for messages in a given thread.
//
// Args:
//   - ctx: context of the request
//   - req: search request
//
// Returns:
//   - response: search result
//   - error: error if occurred
func (s *MessageHistoryService) SearchThreadMessagesHistory(ctx context.Context, req *pb.SearchMessageHistoryRequest) (*pb.SearchMessageHistoryResponse, error) {
	searchQuery := mapper.MapSearchMessageHistoryRequestToDTO(req)

	resp, err := s.messageHistorySearcher.Search(ctx, searchQuery)
	if err != nil {
		return nil, err
	}
	mappedResp := mapper.MapToSearchHistoryProto(resp)

	return mappedResp, nil
}
