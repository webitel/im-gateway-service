package service

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwperm "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	imthread "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	permcli "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

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
		RequestInitiatorId: &initiatorID,
		MemberId:           &req.MemberId,
		Size:               1,
	}

	resp, err := s.threadClient.GetThreadPermissions(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	var (
		permissions      []*gtwperm.ThreadPermissions
		uniqueContactMap = make(map[string]*contact.Contact)
	)
	for _, perm := range resp.Permissions {
		uniqueContactMap[perm.MemberId] = nil
	}
	contacts, err := s.findContacts(ctx, slices.Collect(maps.Keys(uniqueContactMap)))
	if err != nil {
		return nil, err
	}
	for _, contact := range contacts {
		uniqueContactMap[contact.Id] = contact
	}

	for _, perm := range resp.Permissions {
		contact := uniqueContactMap[perm.MemberId]
		if contact == nil {
			s.logger.Warn("contact not found for permission member", slog.String("memberId", perm.MemberId))
			continue
		}
		permissions = append(permissions, &gtwperm.ThreadPermissions{
			Id:                          perm.Id,
			ThreadId:                    perm.ThreadId,
			MemberId:                    perm.MemberId,
			CreatedAt:                   perm.CreatedAt,
			UpdatedAt:                   perm.UpdatedAt,
			CanSendMessages:             perm.CanSendMessages,
			CanAddMembers:               perm.CanAddMembers,
			CanRemoveMembers:            perm.CanRemoveMembers,
			CanChangeMembersPermissions: perm.CanChangeMembersPermissions,
			CanChangeThreadInfo:         perm.CanChangeThreadInfo,
		})

	}

	return &gtwperm.GetThreadPermissionsResponse{
		Permissions: permissions,
	}, nil

}

func (s *ThreadPermissionService) findContacts(ctx context.Context, contactIDs []string) ([]*contact.Contact, error) {
	resp, err := s.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Ids: contactIDs,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetContacts(), nil
}

func (s *ThreadPermissionService) Update(context.Context, *gtwperm.UpdateThreadPermissionsRequest) (*gtwperm.ThreadPermissions, error) {
	panic("unimplemented")
}
