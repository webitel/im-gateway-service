package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var _ impb.ThreadTagManagementServer = (*ThreadTagServer)(nil)

type ThreadTagServer struct {
	impb.UnimplementedThreadTagManagementServer
	log    *slog.Logger
	tagger service.ThreadTagger
}

func NewThreadTagServer(log *slog.Logger, tagger service.ThreadTagger) *ThreadTagServer {
	return &ThreadTagServer{
		log:    log,
		tagger: tagger,
	}
}

// AddTag implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) AddTag(ctx context.Context, req *impb.AddThreadTagRequest) (*impb.ChatTag, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.AddTag(ctx, req)
}

// RemoveTag implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) RemoveTag(ctx context.Context, req *impb.RemoveThreadTagRequest) (*impb.RemoveThreadTagResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.RemoveTag(ctx, req)
}

// RemoveTagByName implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) RemoveTagByName(ctx context.Context, req *impb.RemoveThreadTagByNameRequest) (*impb.RemoveThreadTagResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.RemoveTagByName(ctx, req)
}

// Snooze implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) Snooze(ctx context.Context, req *impb.SnoozeThreadRequest) (*impb.ChatTag, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.Snooze(ctx, req)
}

// Unsnooze implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) Unsnooze(ctx context.Context, req *impb.UnsnoozeThreadRequest) (*impb.RemoveThreadTagResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.Unsnooze(ctx, req)
}

// Search implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) Search(ctx context.Context, req *impb.SearchThreadTagsRequest) (*impb.SearchThreadTagsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.Search(ctx, req)
}
