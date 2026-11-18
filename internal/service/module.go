package service

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
	crmcontacts "github.com/webitel/im-gateway-service/infra/client/crm-contacts"
	imauth "github.com/webitel/im-gateway-service/infra/client/im-auth"
	improviders "github.com/webitel/im-gateway-service/infra/client/im-providers"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
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
			NewUpdates,
			fx.As(new(UpdatesFetcher)),
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
			NewThreadTagService,
			fx.As(new(ThreadTagger)),
		),
		fx.Annotate(
			NewThreadPreviewService,
			fx.As(new(ThreadPreviewer)),
		),
		fx.Annotate(
			NewContactSettingsService,
			fx.As(new(ContactSettingsManager)),
		),
		fx.Annotate(newVia, fx.As(new(Via))),

		fx.Annotate(
			NewAppConfigService,
			fx.As(new(AppConfigProvider)),
		),
		fx.Annotate(
			func(c *imauth.AdminClient) AdminAppSearcher { return c },
			fx.As(new(AdminAppSearcher)),
		),
		fx.Annotate(
			NewThreadChannelService,
			fx.As(new(ThreadChannelResolver)),
		),
		NewContactCandidates,
		func(c *contactCandidates) ContactCandidates { return c },
		func(c *contactCandidates) CRMContactAccessChecker { return c },
		func(c *imthread.ThreadClient) ThreadReader { return c },
		func(c *imthread.ContactIdentificationClient) ContactCandidateLister { return c },
		func(c *crmcontacts.Client) CRMContactVisibility { return c },
		fx.Annotate(
			func(c *improviders.GateClient) GateGetter { return c },
			fx.As(new(GateGetter)),
		),
	),
)
