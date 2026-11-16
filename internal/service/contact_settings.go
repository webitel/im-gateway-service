package service

import (
	"context"
	"log/slog"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	contactpb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwpb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	gtwperm "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
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
		ContactId:          req.GetContactId(),
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
		ContactId:        settings.GetContactId(),
		UpdatedAt:        settings.GetUpdatedAt(),
		AllowInvitesFrom: gtwpb.UserFilter(settings.GetAllowInvitesFrom()),
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
		ContactId:         req.GetContactId(),
	}

	if req.AllowInvitesFrom != nil {
		allowInvitesFrom := contactpb.UserFilter(req.GetAllowInvitesFrom())
		internalReq.AllowInvitesFrom = &allowInvitesFrom
	}

	resp, err := s.contactClient.UpdateSettings(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return s.convertToContactSettings(resp)
}
