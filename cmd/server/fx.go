package server

import (
	"go.uber.org/fx"

	"github.com/webitel/webitel-go-kit/infra/discovery"

	"github.com/webitel/im-gateway-service/config"
	defaultauth "github.com/webitel/im-gateway-service/infra/auth/standard"
	webiteldi "github.com/webitel/im-gateway-service/infra/client/di"
	"github.com/webitel/im-gateway-service/infra/pubsub"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	"github.com/webitel/im-gateway-service/infra/tls"
	grpchandler "github.com/webitel/im-gateway-service/internal/handler/grpc"
	"github.com/webitel/im-gateway-service/internal/service"
)

func NewApp(cfg *config.Config) *fx.App {
	return fx.New(
		fx.Provide(
			func() *config.Config { return cfg },
			ProvideLogger,
			ProvideSD,
		),
		fx.Invoke(func(discovery discovery.DiscoveryProvider) error { return nil }),
		webiteldi.Module,
		defaultauth.Module,
		pubsub.Module,
		tls.Module,
		service.Module,
		grpcsrv.Module,
		grpchandler.Module,
	)
}
