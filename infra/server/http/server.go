package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
	"github.com/webitel/im-gateway-service/infra/server/http/middleware"
	apptls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/model"
)

var Module = fx.Module("http_server",
	fx.Provide(ProvideServer),
	fx.Invoke(func(*Server) {}),
)

type Server struct {
	*http.Server

	listener net.Listener
}

func ProvideServer(
	cfg *config.Config,
	logger *slog.Logger,
	handler http.Handler,
	lc fx.Lifecycle,
) (*Server, error) {
	var tlsCfg *tls.Config
	if cfg.Service.HTTP.VerifyCerts {
		var err error
		tlsCfg, err = apptls.Load(cfg.Service.HTTP.TLS, tls.NoClientCert)
		if err != nil {
			return nil, err
		}
	}

	wrapped := handler
	if cfg.Log.Otel {
		wrapped = otelhttp.NewHandler(
			handler,
			model.ServiceName,
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			}),
		)
	}

	logged := middleware.LoggingMiddleware(logger)(wrapped)
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isProbe(r.URL.Path) {
			handler.ServeHTTP(w, r)

			return
		}

		logged.ServeHTTP(w, r)
	})

	s := &Server{Server: &http.Server{
		Addr:      cfg.Service.HTTP.Addr,
		Handler:   finalHandler,
		TLSConfig: tlsCfg,
	}}
	srv := s.Server

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var lcfg net.ListenConfig

			l, err := lcfg.Listen(ctx, "tcp", srv.Addr)
			if err != nil {
				return fmt.Errorf("listen http %s: %w", srv.Addr, err)
			}

			s.listener = l

			go func() {
				logger.Info(fmt.Sprintf("listen http %s", cfg.Service.HTTP.Addr))
				var err error
				if tlsCfg != nil {
					err = srv.ServeTLS(l, "", "")
				} else {
					err = srv.Serve(l)
				}
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("http server error", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Debug("receiving shutdown signal for http server")
			return srv.Shutdown(ctx)
		},
	})

	return s, nil
}

func (s *Server) Listener() net.Listener {
	return s.listener
}

func isProbe(path string) bool {
	switch path {
	case "/livez", "/readyz", "/healthz":
		return true
	default:
		return false
	}
}
