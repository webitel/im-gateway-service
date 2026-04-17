package webiteldi

import (
	"context"

	imauth "github.com/webitel/im-gateway-service/infra/client/im-auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	storage "github.com/webitel/im-gateway-service/infra/client/storage"
	"go.uber.org/fx"
)

var Module = fx.Module(
	"webitel_clients",

	// [CONSTRUCTOR] Provides the resilient contact client
	fx.Provide(imthread.New, imthread.NewMessageHistoryClient, imthread.NewThreadClient, imthread.NewThreadPermissionClient),
	fx.Provide(imauth.New),
	fx.Provide(imcontact.New),
	fx.Provide(storage.New),

	// [LIFECYCLE] Ensures the gRPC connection pool is closed gracefully on app shutdown
	fx.Invoke(
		func(lc fx.Lifecycle, client *imthread.Client) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return client.Close()
				},
			})
		},

		func(lc fx.Lifecycle, client *imthread.MessageHistoryClient) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return client.Close()
				},
			})
		},

		func(lc fx.Lifecycle, client *imthread.ThreadClient) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return client.Close()
				},
			})
		},
	),

	fx.Invoke(func(lc fx.Lifecycle, client *imauth.Client) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return client.Close()
			},
		})
	}),

	fx.Invoke(func(lc fx.Lifecycle, client *imcontact.Client) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return client.Close()
			},
		})
	}),

	fx.Invoke(func(lc fx.Lifecycle, client *storage.Client) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return client.Close()
			},
		})
	}),
	fx.Invoke(func(lc fx.Lifecycle, client *imthread.ThreadPermissionClient) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return client.Close()
			},
		})
	}),
)
