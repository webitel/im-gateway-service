package http

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/infra/auth"
	httpmw "github.com/webitel/im-gateway-service/infra/server/http/middleware"
)

var Module = fx.Module("http_handler",
	fx.Provide(
		func() *http.ServeMux { return http.NewServeMux() },
		func(authorizer auth.Authorizer) func(http.Handler) http.Handler {
			return httpmw.NewAuthMiddleware(authorizer)
		},
		NewHandler,
	),
	// Force Handler instantiation so routes are registered on the mux.
	fx.Invoke(func(*Handler) {}),
)
