package service

import (
	"context"
	"log/slog"

	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type (
	MessageHistorySearcher interface {
		Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error) 
	}

	messageHistory struct {
		logger *slog.Logger
		historyClient *imthread.MessageHistoryClient
	}
)

// NewMessageHistory returns a new instance of MessageHistorySearcher.
//
// Args:
//  - logger: logger for the service
//  - historyClient: client for the Message History service
//
// Returns:
//  - A new instance of MessageHistorySearcher
func NewMessageHistory(logger *slog.Logger, historyClient *imthread.MessageHistoryClient) *messageHistory {
	return &messageHistory{
		logger:        logger,
		historyClient: historyClient,
	}
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
func (s *messageHistory) Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error)  {
	log := s.logger.With(
		slog.String("op", "messageHistory.Search"),
		slog.Int("domain_id", int(searchQuery.DomainID)),
	)
	
	response, err := s.historyClient.Search(ctx, searchQuery)
	if err != nil {
		log.Error("failed to fetch message history from provider",
			slog.Any("err", err),
		)
		return nil, err
	}

	return response, nil
}