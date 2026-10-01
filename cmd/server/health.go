package server

import (
	"log/slog"
	"net/http"

	"github.com/webitel/webitel-go-kit/infra/health"
	healthhttp "github.com/webitel/webitel-go-kit/infra/health/http"

	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
)

func registerHealth(log *slog.Logger, h *health.Registry, grpcServer *grpcsrv.Server, mux *http.ServeMux) {
	mux.Handle("/livez", healthhttp.LivenessHandler(h, healthhttp.WithLogger(log)))
	mux.Handle("/readyz", healthhttp.ReadinessHandler(h, healthhttp.WithLogger(log)))
	mux.Handle("/healthz", healthhttp.HealthHandler(h, healthhttp.WithLogger(log)))

	h.Critical("grpc", health.ListenerCheck(grpcServer.Listener()))
}
