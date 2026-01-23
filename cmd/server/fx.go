package server

import (
	"github.com/webitel/im-gateway-service/infra/pubsub"
	"github.com/webitel/im-gateway-service/infra/tls"
	"go.uber.org/fx"

	"github.com/webitel/webitel-go-kit/infra/discovery"

	"github.com/webitel/im-gateway-service/config"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	grpchandler "github.com/webitel/im-gateway-service/internal/handler/grpc"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/im-gateway-service/internal/store/postgres"
)

func NewApp(cfg *config.Config) *fx.App {
	return fx.New(
		fx.Provide(
			func() *config.Config { return cfg },
			ProvideLogger,
			ProvideSD,
		),
		fx.Invoke(func(discovery discovery.DiscoveryProvider) error { return nil }),

		pubsub.Module,
		tls.Module,
		postgres.Module,
		service.Module,
		grpcsrv.Module,
		grpchandler.Module,
	)
}
