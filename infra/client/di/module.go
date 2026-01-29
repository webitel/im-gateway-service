package webiteldi

import (
	"context"

	imauth "github.com/webitel/im-gateway-service/infra/client/im-auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"go.uber.org/fx"
)

var Module = fx.Module(
	"webitel_clients",

	// [CONSTRUCTOR] Provides the resilient contact client
	fx.Provide(imthread.New),
	fx.Provide(imauth.New),
	fx.Provide(imcontact.New),

	// [LIFECYCLE] Ensures the gRPC connection pool is closed gracefully on app shutdown
	fx.Invoke(func(lc fx.Lifecycle, client *imthread.Client) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return client.Close()
			},
		})
	}),

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
)
