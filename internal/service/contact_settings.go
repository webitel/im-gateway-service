package service

import (
	"context"
	"log/slog"

	contactpb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwpb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	gtwperm "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type ContactSettingsManager interface {
	Get(ctx context.Context, req *gtwpb.GetContactSettingsRequest) (*gtwpb.ContactSettings, error)
	Update(ctx context.Context, req *gtwpb.UpdateContactSettingsRequest) (*gtwpb.ContactSettings, error)
}
type ContactSettingsService struct {
	logger        *slog.Logger
	contactClient *imcontact.ContactSettingsClient
}

func NewContactSettingsService(logger *slog.Logger, contact *imcontact.ContactSettingsClient) *ContactSettingsService {
	return &ContactSettingsService{
		logger:        logger,
		contactClient: contact,
	}
}

func (s *ContactSettingsService) Get(ctx context.Context, req *gtwpb.GetContactSettingsRequest) (*gtwpb.ContactSettings, error) {

	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	initiatorID := identity.GetContactID()
	internalReq := &contactpb.GetContactSettingsRequest{
		InitiatorContactId: &initiatorID,
		ContactId:          req.ContactId,
	}

	resp, err := s.contactClient.GetSettings(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	return s.convertToContactSettings(resp)

}

func (s *ContactSettingsService) convertToContactSettings(settings *contactpb.Settings) (*gtwpb.ContactSettings, error) {
	if settings == nil {
		return nil, nil
	}
	return &gtwpb.ContactSettings{
		ContactId:        settings.ContactId,
		UpdatedAt:        settings.UpdatedAt,
		AllowInvitesFrom: gtwpb.UserFilter(settings.AllowInvitesFrom),
	}, nil
}

func (s *ContactSettingsService) Update(ctx context.Context, req *gtwperm.UpdateContactSettingsRequest) (*gtwperm.ContactSettings, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	initiatorID := identity.GetContactID()

	internalReq := &contactpb.UpdateContactSettingsRequest{
		IntiatorContactId: &initiatorID,
		ContactId:         req.ContactId,
	}

	if req.AllowInvitesFrom != nil {
		allowInvitesFrom := contactpb.UserFilter(*req.AllowInvitesFrom)
		internalReq.AllowInvitesFrom = &allowInvitesFrom
	}

	resp, err := s.contactClient.UpdateSettings(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	return s.convertToContactSettings(resp)
}
