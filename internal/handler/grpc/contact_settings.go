package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type ContactSettingsServer struct {
	impb.UnimplementedContactSettingsManagementServer

	log                    *slog.Logger
	contactSettingsService service.ContactSettingsManager
}

func NewContactSettingsServer(log *slog.Logger, settings service.ContactSettingsManager) *ContactSettingsServer {
	return &ContactSettingsServer{
		log:                    log,
		contactSettingsService: settings,
	}
}

func (t *ContactSettingsServer) Get(ctx context.Context, req *impb.GetContactSettingsRequest) (*impb.GetContactSettingsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	res, err := t.contactSettingsService.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	return &impb.GetContactSettingsResponse{
		Settings: res,
	}, nil
}

func (t *ContactSettingsServer) Update(ctx context.Context, req *impb.UpdateContactSettingsRequest) (*impb.UpdateContactSettingsResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	res, err := t.contactSettingsService.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	return &impb.UpdateContactSettingsResponse{
		Settings: res,
	}, nil
}
