package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

type (
	ThreadService struct {
		impb.UnimplementedThreadManagementServer

		logger         *slog.Logger
		converter      mapper.ThreadConverter
		threadSearcher service.ThreadManager
	}
)

func NewThreadService(logger *slog.Logger, converter mapper.ThreadConverter, threadSearcher service.ThreadManager) *ThreadService {
	return &ThreadService{
		logger:         logger,
		converter:      converter,
		threadSearcher: threadSearcher,
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
		Items: pbThreads,
		Next:  next,
	}, nil
}


func (s *ThreadService) AddMember(ctx context.Context, req *impb.AddMemberRequest) (*impb.AddMemberResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.AddMember"))

	converted, err := mapper.Convert(req, new(threadv1.AddMemberRequest))
	if err != nil {
		return nil, err
	}

	err = s.threadSearcher.AddMember(ctx, converted)
	if err != nil {
		log.Error("failed to add member to thread", slog.Any("err", err))
		return nil, err
	}

	return &impb.AddMemberResponse{}, nil


}


func (s *ThreadService) RemoveMember(ctx context.Context, req *impb.RemoveMemberRequest) (*impb.RemoveMemberResponse, error) {
	log := s.logger.With(slog.String("op", "ThreadService.AddMember"))

	converted, err := mapper.Convert(req, new(threadv1.RemoveMemberRequest))
	if err != nil {
		return nil, err
	}

	err = s.threadSearcher.RemoveMember(ctx, converted)
	if err != nil {
		log.Error("failed to add member to thread", slog.Any("err", err))
		return nil, err
	}

	return &impb.RemoveMemberResponse{}, nil


}