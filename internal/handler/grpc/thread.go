package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

type (
	ThreadService struct {
		impb.UnimplementedThreadManagementServer

		logger *slog.Logger
		converter mapper.ThreadConverter
		threadSearcher service.ThreadSearcher	
	}
)

func NewThreadService(logger *slog.Logger, converter mapper.ThreadConverter, threadSearcher service.ThreadSearcher) *ThreadService {
	return &ThreadService{
		logger:   logger,
		converter:   converter,
		threadSearcher:   threadSearcher,
	}
}

func (s *ThreadService) Search(ctx context.Context, req *impb.ThreadSearchRequest) (*impb.SearchThreadResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.Search"))
	
	searchRequestDTO := s.converter.ProtoThreadSearchRequestToDTO(req)
	resultThreads, next, err := s.threadSearcher.Search(ctx, searchRequestDTO)
	if err != nil {
		log.Error("failed to fetch threads from provider", slog.Any("err", err))
		return nil, err
	}
	
	pbThreads := s.converter.DTOToProto(resultThreads)

	return &impb.SearchThreadResponse{
		Threads: pbThreads,
		Next: next,
	}, nil
}