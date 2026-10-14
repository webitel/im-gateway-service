package server

import (
	"log/slog"
	"net/http"

	"go.uber.org/fx"

	"github.com/webitel/webitel-go-kit/infra/health"
	healthhttp "github.com/webitel/webitel-go-kit/infra/health/http"

	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	httpsrv "github.com/webitel/im-gateway-service/infra/server/http"
)

func registerHealth(
	lc fx.Lifecycle,
	log *slog.Logger,
	h *health.Registry,
	grpcServer *grpcsrv.Server,
	httpServer *httpsrv.Server,
	mux *http.ServeMux,
) {
	mux.Handle("/livez", healthhttp.LivenessHandler(h, healthhttp.WithLogger(log)))
	mux.Handle("/readyz", healthhttp.ReadinessHandler(h, healthhttp.WithLogger(log)))
	mux.Handle("/healthz", healthhttp.HealthHandler(h, healthhttp.WithLogger(log)))

	lc.Append(fx.StartHook(func() {
		h.Critical("grpc", health.ListenerCheck(grpcServer.Listener()))
		h.Critical("http", health.ListenerCheck(httpServer.Listener()))
	}))
}
