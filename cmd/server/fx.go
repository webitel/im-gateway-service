package server

import (
	"github.com/webitel/webitel-go-kit/infra/profiler"
	"go.uber.org/fx"

	"github.com/webitel/webitel-go-kit/infra/discovery"

	"github.com/webitel/im-gateway-service/config"
	defaultauth "github.com/webitel/im-gateway-service/infra/auth/standard"
	webiteldi "github.com/webitel/im-gateway-service/infra/client/di"
	"github.com/webitel/im-gateway-service/infra/pubsub"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	httpsrv "github.com/webitel/im-gateway-service/infra/server/http"
	"github.com/webitel/im-gateway-service/infra/tls"
	grpchandler "github.com/webitel/im-gateway-service/internal/handler/grpc"
	httphandler "github.com/webitel/im-gateway-service/internal/handler/http"
	"github.com/webitel/im-gateway-service/internal/service"
)

func NewApp(cfg *config.Config) *fx.App {
	return fx.New(
		fx.Provide(
			func() *config.Config { return cfg },
			ProvideLogger,
			ProvideSD,
			ProvideProfile,
		),
		fx.Invoke(func(discovery discovery.DiscoveryProvider) error { return nil }),
		webiteldi.Module,
		defaultauth.Module,
		pubsub.Module,
		tls.Module,
		service.Module,
		grpcsrv.Module,
		grpchandler.Module,
		httphandler.Module,
		httpsrv.Module,
		profiler.Module,
	)
}
