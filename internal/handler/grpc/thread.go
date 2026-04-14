package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

type ThreadService struct {
	impb.UnimplementedThreadManagementServer

	logger        *slog.Logger
	converter     mapper.ThreadConverter
	threadManager service.ThreadManager
}

func NewThreadService(logger *slog.Logger, converter mapper.ThreadConverter, threadSearcher service.ThreadManager) *ThreadService {
	return &ThreadService{
		logger:        logger,
		converter:     converter,
		threadManager: threadSearcher,
	}
}

func (s *ThreadService) Search(ctx context.Context, req *impb.ThreadSearchRequest) (*impb.SearchThreadResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.Search"))

	searchRequestDTO := s.converter.ProtoThreadSearchRequestToDTO(req)
	resultThreads, next, err := s.threadManager.Search(ctx, searchRequestDTO)
	if err != nil {
		log.Error("failed to fetch threads from provider", slog.Any("err", err))
		return nil, err
	}

	pbThreads := s.converter.DTOToProto(resultThreads)

	return &impb.SearchThreadResponse{
		Items: pbThreads,
		Next:  next,
	}, nil
}

func (s *ThreadService) AddMember(ctx context.Context, req *impb.AddMemberRequest) (*impb.AddMemberResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.AddMember"))

	err := s.threadManager.AddMember(ctx, req)
	if err != nil {
		log.Error("failed to add member to thread", slog.Any("err", err))
		return nil, err
	}

	return &impb.AddMemberResponse{}, nil

}

func (s *ThreadService) RemoveMember(ctx context.Context, req *impb.RemoveMemberRequest) (*impb.RemoveMemberResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.AddMember"))

	err := s.threadManager.RemoveMember(ctx, req)
	if err != nil {
		log.Error("failed to add member to thread", slog.Any("err", err))
		return nil, err
	}

	return &impb.RemoveMemberResponse{}, nil

}

func (s *ThreadService) SetVariables(ctx context.Context, req *impb.SetVariablesRequest) (*impb.ThreadVariables, error) {
	return s.threadManager.SetVariables(ctx, req)
}

func (s *ThreadService) SearchVariables(ctx context.Context, req *impb.SearchVariablesRequest) (*impb.SearchVariablesResponse, error) {
	return s.threadManager.SearchVariables(ctx, req)
}

func (s *ThreadService) LocateVariables(ctx context.Context, req *impb.LocateVariablesRequest) (*impb.ThreadVariables, error) {
	return s.threadManager.LocateVariables(ctx, req)
}

func (s *ThreadService) FlushVariables(ctx context.Context, req *impb.FlushVariablesRequest) (*impb.ThreadVariables, error) {
	return s.threadManager.FlushVariables(ctx, req)
}
