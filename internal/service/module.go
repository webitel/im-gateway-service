package service

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
	storageclient "github.com/webitel/im-gateway-service/infra/client/storage"
)

var Module = fx.Module(
	"service",

	fx.Provide(
		// Domain services
		fx.Annotate(
			NewMessageService,
			fx.As(new(Messenger)),
		),

		fx.Annotate(
			func(logger *slog.Logger, storageClient *storageclient.Client, cfg *config.Config) Media {
				return NewMediaService(logger, storageClient, cfg.Service.UploadChunkSize)
			},
			fx.As(new(Media)),
		),

		fx.Annotate(
			NewAccountService,
			fx.As(new(Accounter)),
		),

		fx.Annotate(
			NewContactService,
			fx.As(new(Contacter)),
		),

		fx.Annotate(
			NewMessageHistory,
			fx.As(new(MessageHistorySearcher)),
		),

		fx.Annotate(
			NewBotService,
			fx.As(new(Botter)),
		),

		fx.Annotate(
			NewThread,
			fx.As(new(ThreadManager)),
		),
		fx.Annotate(
			NewThreadPermissionService,
			fx.As(new(ThreadPermissioner)),
		),
		fx.Annotate(
			NewContactSettingsService,
			fx.As(new(ContactSettingsManager)),
		),
		fx.Annotate(newVia, fx.As(new(Via))),
	),
)
