package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
	apptls "github.com/webitel/im-gateway-service/infra/tls"
)

var Module = fx.Module("http_server",
	fx.Invoke(ProvideServer),
)

func ProvideServer(
	cfg *config.Config,
	tlsCfg *apptls.Config,
	logger *slog.Logger,
	mux *http.ServeMux,
	lc fx.Lifecycle,
) error {
	srv := &http.Server{
		Addr:      cfg.Service.HTTPAddress,
		Handler:   mux,
		TLSConfig: tlsCfg.Server,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info(fmt.Sprintf("listen http %s", cfg.Service.HTTPAddress))
				var err error
				if tlsCfg.Server != nil {
					err = srv.ListenAndServeTLS("", "")
				} else {
					err = srv.ListenAndServe()
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

	return nil
}
