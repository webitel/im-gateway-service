package http

import (
	"context"
	"crypto/tls"
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
	logger *slog.Logger,
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

	srv := &http.Server{
		Addr:      cfg.Service.HTTP.Address,
		Handler:   handler,
		TLSConfig: tlsCfg,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info(fmt.Sprintf("listen http %s", cfg.Service.HTTP.Address))
				var err error
				if tlsCfg != nil {
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