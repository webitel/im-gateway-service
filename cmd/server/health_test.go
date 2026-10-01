package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/fx"

	healthfx "github.com/webitel/webitel-go-kit/infra/health/fx"

	"github.com/webitel/im-gateway-service/config"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	httpsrv "github.com/webitel/im-gateway-service/infra/server/http"
)

func TestHealth_Lifecycle(t *testing.T) {
	cfg := &config.Config{}
	cfg.Service.HTTP.Addr = freeAddr(t)

	grpcServer, err := grpcsrv.New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new grpc server: %v", err)
	}

	app := fx.New(
		fx.NopLogger,
		fx.Supply(cfg, grpcServer, slog.New(slog.NewTextHandler(io.Discard, nil))),
		fx.Provide(
			http.NewServeMux,
			func(mux *http.ServeMux) http.Handler { return mux },
		),
		healthfx.Module(healthfx.Config{}),
		httpsrv.Module,
		fx.Invoke(func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						if err := grpcServer.Listen(); err != nil && !errors.Is(err, net.ErrClosed) {
							t.Errorf("grpc listen: %v", err)
						}
					}()

					return nil
				},
				OnStop: func(context.Context) error { return grpcServer.Shutdown() },
			})
		}),
		fx.Invoke(registerHealth),
		healthfx.Shutdown(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		t.Fatalf("start app: %v", err)
	}

	waitReadyz(t, "http://"+cfg.Service.HTTP.Addr+"/readyz")

	if err := app.Stop(context.Background()); err != nil {
		t.Fatalf("stop app: %v", err)
	}
}

func waitReadyz(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		resp, err := http.Get(url)
		if err == nil {
			if cerr := resp.Body.Close(); cerr != nil {
				t.Errorf("close body: %v", cerr)
			}

			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("GET %s never returned 200: last err %v", url, err)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}

	addr := l.Addr().String()

	if err := l.Close(); err != nil {
		t.Fatalf("release free port: %v", err)
	}

	return addr
}
