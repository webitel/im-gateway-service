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

// Search implements [api.ThreadTagManagementServer].
func (t *ThreadTagServer) Search(ctx context.Context, req *impb.SearchThreadTagsRequest) (*impb.SearchThreadTagsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	return t.tagger.Search(ctx, req)
}
