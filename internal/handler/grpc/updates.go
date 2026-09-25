package grpc

import (
	"context"

	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type UpdatesService struct {
	pb.UnimplementedUpdatesServer

	updates service.UpdatesFetcher
}

func NewUpdatesService(updates service.UpdatesFetcher) *UpdatesService {
	return &UpdatesService{updates: updates}
}

// GetUpdates returns every thread changed for the caller since the cursor.
func (s *UpdatesService) GetUpdates(ctx context.Context, req *pb.GetUpdatesRequest) (*pb.GetUpdatesResponse, error) {
	resp, err := s.updates.GetUpdates(ctx, &dto.GetUpdatesRequest{Cursor: req.GetCursor()})
	if err != nil {
		return nil, err
	}

	return mapper.MapToGetUpdatesProto(resp), nil
}
