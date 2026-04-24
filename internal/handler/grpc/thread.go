package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
)

var _ impb.ThreadManagementServer = (*ThreadService)(nil)

type (
	ThreadService struct {
		impb.UnimplementedThreadManagementServer

		logger        *slog.Logger
		threadManager service.ThreadManager
	}
)

func NewThreadService(logger *slog.Logger, threadManager service.ThreadManager) *ThreadService {
	return &ThreadService{
		logger:        logger,
		threadManager: threadManager,
	}
}

func (s *ThreadService) Search(ctx context.Context, req *impb.ThreadSearchRequest) (*impb.SearchThreadResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.Search"))

	resultThreads, next, err := s.threadManager.Search(ctx, req)
	if err != nil {
		log.Error("failed to fetch threads from provider", slog.Any("err", err))
		return nil, err
	}

	return &impb.SearchThreadResponse{
		Items: resultThreads,
		Next:  next,
	}, nil
}

func (s *ThreadService) SearchLeft(ctx context.Context, req *impb.SearchLeftRequest) (*impb.SearchLeftResponse, error) {
	log := s.logger.With(slog.String("op", "ListLeftChats"))

	result, next, err := s.threadManager.SearchLeft(ctx, req)
	if err != nil {
		log.Error("list left chats", slog.Any("err", err))
		return nil, err
	}

	return &impb.SearchLeftResponse{
		Items: result,
		Next:  next,
	}, nil
}

func (s *ThreadService) AddMember(ctx context.Context, req *impb.AddMemberRequest) (*impb.AddMemberResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.AddMember"))

	member, err := s.threadManager.AddMember(ctx, req)
	if err != nil {
		log.Error("failed to add member to thread", slog.Any("err", err))
		return nil, err
	}

	response := &impb.AddMemberResponse{Member: member.Member}

	return response, nil
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
