package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
)

var Module = fx.Module("http_server",
	fx.Invoke(ProvideServer),
)

func ProvideServer(
	cfg *config.Config,
	logger *slog.Logger,
	mux *http.ServeMux,
	lc fx.Lifecycle,
) error {
	srv := &http.Server{
		Addr:    cfg.Service.HTTPAddress,
		Handler: mux,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info(fmt.Sprintf("listen http %s", cfg.Service.HTTPAddress))
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
