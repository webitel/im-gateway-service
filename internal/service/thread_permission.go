package service

import (
	"context"
	"log/slog"

	gtwperm "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	imthread "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	permcli "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type ThreadPermissioner interface {
	Get(ctx context.Context, req *gtwperm.GetThreadPermissionsRequest) (*gtwperm.GetThreadPermissionsResponse, error)
	Update(ctx context.Context, req *gtwperm.UpdateThreadPermissionsRequest) (*gtwperm.UpdateThreadPermissionsResponse, error)
}

type ThreadPermissionService struct {
	logger        *slog.Logger
	threadClient  *permcli.ThreadPermissionClient
	contactClient *imcontact.Client
}

func NewThreadPermissionService(logger *slog.Logger, thread *permcli.ThreadPermissionClient, contactClient *imcontact.Client) *ThreadPermissionService {
	return &ThreadPermissionService{
		logger:        logger,
		threadClient:  thread,
		contactClient: contactClient,
	}
}

func (s *ThreadPermissionService) Get(ctx context.Context, req *gtwperm.GetThreadPermissionsRequest) (*gtwperm.GetThreadPermissionsResponse, error) {

	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	initiatorID := identity.GetContactID()
	internalReq := &imthread.GetThreadPermissionsRequest{
		InitiatorContactId: &initiatorID,
		MemberId:           &req.MemberId,
		Size:               1,
	}

	resp, err := s.threadClient.GetThreadPermissions(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	if len(resp.Permissions) == 0 {
		return nil, errors.NotFound("no permissions found for member")
	}
	perm := resp.Permissions[0]

	convertedPerm := s.convertToThreadPermission(perm)

	return &gtwperm.GetThreadPermissionsResponse{
		Permissions: convertedPerm,
	}, nil

}

func (s *ThreadPermissionService) Update(ctx context.Context, req *gtwperm.UpdateThreadPermissionsRequest) (*gtwperm.UpdateThreadPermissionsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	initiatorID := identity.GetContactID()
	internalReq := &imthread.UpdateThreadPermissionsRequest{
		InitiatorContactId:          &initiatorID,
		MemberId:                    req.MemberId,
		CanSendMessages:             req.CanSendMessages,
		CanAddMembers:               req.CanAddMembers,
		CanRemoveMembers:            req.CanRemoveMembers,
		CanChangeMembersPermissions: req.CanChangeMembersPermissions,
		CanChangeThreadInfo:         req.CanChangeThreadInfo,
	}

	resp, err := s.threadClient.UpdateThreadPermissions(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	convertedPerm := s.convertToThreadPermission(resp)

	return &gtwperm.UpdateThreadPermissionsResponse{
		Permissions: convertedPerm,
	}, nil
}

func (s *ThreadPermissionService) convertToThreadPermission(perm *imthread.ThreadPermissions) *gtwperm.ThreadPermissions {
	if perm == nil {
		return nil
	}
	return &gtwperm.ThreadPermissions{
		Id:                          perm.Id,
		MemberId:                    perm.MemberId,
		CreatedAt:                   perm.CreatedAt,
		UpdatedAt:                   perm.UpdatedAt,
		CanSendMessages:             perm.CanSendMessages,
		CanAddMembers:               perm.CanAddMembers,
		CanRemoveMembers:            perm.CanRemoveMembers,
		CanChangeMembersPermissions: perm.CanChangeMembersPermissions,
		CanChangeThreadInfo:         perm.CanChangeThreadInfo,
	}
}
