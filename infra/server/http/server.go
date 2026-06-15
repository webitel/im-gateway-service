package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/webitel/webitel-go-kit/pkg/depenlog"
	"github.com/webitel/webitel-go-kit/pkg/errors"
	kitlog "github.com/webitel/webitel-go-kit/pkg/logger"
	"github.com/webitel/webitel-go-kit/pkg/semconv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
	apptls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/model"
)

var Module = fx.Module("http_server",
	fx.Invoke(ProvideServer),
)

func ProvideServer(
	cfg *config.Config,
	logger *slog.Logger,
	kit kitlog.Logger,
	handler http.Handler,
	lc fx.Lifecycle,
) error {
	var tlsCfg *tls.Config
	if cfg.Service.HTTP.VerifyCerts {
		var err error
		tlsCfg, err = apptls.Load(cfg.Service.HTTP.TLS, tls.NoClientCert)
		if err != nil {
			return err
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

	loggingMiddleware := depenlog.Middleware(kit)
	finalHandler := loggingMiddleware(wrapped)

	srv := &http.Server{
		Addr:      cfg.Service.HTTP.Addr,
		Handler:   finalHandler,
		TLSConfig: tlsCfg,
		ErrorLog:  depenlog.ErrorLog(kit),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info(fmt.Sprintf("listen http %s", cfg.Service.HTTP.Addr))
				var err error
				if tlsCfg != nil {
					err = srv.ListenAndServeTLS("", "")
				} else {
					err = srv.ListenAndServe()
				}
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("http server error", semconv.ErrorKey, err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Debug("receiving shutdown signal for http server")
			return srv.Shutdown(ctx)
		},
	})

	return nil
}
