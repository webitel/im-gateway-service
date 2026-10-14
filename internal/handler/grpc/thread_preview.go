package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var _ impb.ThreadPreviewServiceServer = (*ThreadPreviewServer)(nil)

type ThreadPreviewServer struct {
	impb.UnimplementedThreadPreviewServiceServer
	log       *slog.Logger
	previewer service.ThreadPreviewer
}

func NewThreadPreviewServer(log *slog.Logger, previewer service.ThreadPreviewer) *ThreadPreviewServer {
	return &ThreadPreviewServer{
		log:       log,
		previewer: previewer,
	}
}

// GivePreview implements [api.ThreadPreviewServiceServer].
func (s *ThreadPreviewServer) GivePreview(ctx context.Context, req *impb.GivePreviewRequest) (*impb.GivePreviewResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return s.previewer.GivePreview(ctx, req)
}

// RemoveFromPreview implements [api.ThreadPreviewServiceServer].
func (s *ThreadPreviewServer) RemoveFromPreview(ctx context.Context, req *impb.RemoveFromPreviewRequest) (*impb.RemoveFromPreviewResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return s.previewer.RemoveFromPreview(ctx, req)
}

// UpgradePreviewToFullMember implements [api.ThreadPreviewServiceServer].
func (s *ThreadPreviewServer) UpgradePreviewToFullMember(ctx context.Context, req *impb.UpgradePreviewToFullMemberRequest) (*impb.UpgradePreviewToFullMemberResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return s.previewer.UpgradePreviewToFullMember(ctx, req)
}
