package service

import (
	"context"
	"log/slog"

	gtwtag "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	tagcli "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type ThreadTagger interface {
	AddTag(ctx context.Context, req *gtwtag.AddThreadTagRequest) (*gtwtag.ChatTag, error)
	RemoveTag(ctx context.Context, req *gtwtag.RemoveThreadTagRequest) (*gtwtag.RemoveThreadTagResponse, error)
	Search(ctx context.Context, req *gtwtag.SearchThreadTagsRequest) (*gtwtag.SearchThreadTagsResponse, error)
}

type ThreadTagService struct {
	logger    *slog.Logger
	tagClient *imthread.ThreadTagClient
}

func NewThreadTagService(logger *slog.Logger, tagClient *imthread.ThreadTagClient) *ThreadTagService {
	return &ThreadTagService{
		logger:    logger,
		tagClient: tagClient,
	}
}

func (s *ThreadTagService) AddTag(ctx context.Context, req *gtwtag.AddThreadTagRequest) (*gtwtag.ChatTag, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	internalReq := &tagcli.AddThreadTagRequest{
		ContactId: identity.GetContactID(),
		ThreadId:  req.GetThreadId(),
		Tag:       req.GetTag(),
	}

	resp, err := s.tagClient.AddTag(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return s.convertToChatTag(resp), nil
}

func (s *ThreadTagService) RemoveTag(ctx context.Context, req *gtwtag.RemoveThreadTagRequest) (*gtwtag.RemoveThreadTagResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	internalReq := &tagcli.RemoveThreadTagRequest{
		Id:        req.GetId(),
		ContactId: identity.GetContactID(),
	}

	_, err := s.tagClient.RemoveTag(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return &gtwtag.RemoveThreadTagResponse{}, nil
}

func (s *ThreadTagService) Search(ctx context.Context, req *gtwtag.SearchThreadTagsRequest) (*gtwtag.SearchThreadTagsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	internalReq := &tagcli.SearchThreadTagsRequest{
		ContactId: identity.GetContactID(),
		Page:      req.GetPage(),
		Size:      req.GetSize(),
	}

	resp, err := s.tagClient.Search(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return &gtwtag.SearchThreadTagsResponse{
		Tags: resp.GetTags(),
		Next: resp.GetNext(),
	}, nil
}

func (s *ThreadTagService) convertToChatTag(tag *tagcli.ChatTag) *gtwtag.ChatTag {
	if tag == nil {
		return nil
	}
	return &gtwtag.ChatTag{
		Id:        tag.GetId(),
		ThreadId:  tag.GetThreadId(),
		Tag:       tag.GetTag(),
		CreatedAt: tag.GetCreatedAt(),
	}
}
