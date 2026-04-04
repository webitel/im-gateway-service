package http

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
	"github.com/webitel/im-gateway-service/infra/auth"
	httpmw "github.com/webitel/im-gateway-service/infra/server/http/middleware"
)

var Module = fx.Module("http_handler",
	fx.Provide(
		func() *http.ServeMux { return http.NewServeMux() },
		func(authorizer auth.Authorizer) func(http.Handler) http.Handler {
			return httpmw.NewAuthMiddleware(authorizer)
		},
		fx.Annotate(
			func(cfg *config.Config) func(http.Handler) http.Handler {
				return httpmw.WithBodyLimit(cfg.Service.MaxUploadSize)
			},
			fx.ResultTags(`name:"bodyLimitMW"`),
		),
		func(cfg *config.Config, mux *http.ServeMux) http.Handler {
			var h http.Handler = mux
			return httpmw.WithCORS(cfg.Service.HTTP.CORS.AllowedOrigins, h)
		},
		fx.Annotate(
			NewHandler,
			fx.ParamTags(``, ``, ``, `name:"bodyLimitMW"`, ``),
		),
	),
	// Force Handler instantiation so routes are registered on the mux.
	fx.Invoke(func(*Handler) {}),
)
