package http

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/fx/fxtest"

	"github.com/webitel/im-gateway-service/config"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestProvideServer_ProbesSkipRequestLog(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantLog bool
	}{
		{name: "livez", path: "/livez"},
		{name: "readyz", path: "/readyz"},
		{name: "healthz", path: "/healthz"},
		{name: "regular route", path: "/media", wantLog: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out syncBuffer

			cfg := &config.Config{}
			cfg.Service.HTTP.Addr = freeAddr(t)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			})

			lc := fxtest.NewLifecycle(t)

			if err := ProvideServer(cfg, slog.New(slog.NewTextHandler(&out, nil)), handler, lc); err != nil {
				t.Fatalf("ProvideServer() error = %v", err)
			}

			lc.RequireStart()
			t.Cleanup(lc.RequireStop)

			waitListening(t, cfg.Service.HTTP.Addr)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
				"http://"+cfg.Service.HTTP.Addr+tt.path, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}

			if err := resp.Body.Close(); err != nil {
				t.Errorf("close body: %v", err)
			}

			if got := strings.Contains(out.String(), "http request failed"); got != tt.wantLog {
				t.Errorf("request logged = %v, want %v; log: %q", got, tt.wantLog, out.String())
			}
		})
	}
}

func waitListening(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			if err := conn.Close(); err != nil {
				t.Errorf("close probe conn: %v", err)
			}

			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("server never listened on %s: %v", addr, err)
		}

		time.Sleep(10 * time.Millisecond)
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
