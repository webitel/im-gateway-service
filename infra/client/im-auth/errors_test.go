package imauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	intrcp "github.com/webitel/webitel-go-kit/pkg/interceptors"
)

// renderThroughGoKit runs err through the go-kit error interceptor the gateway
// server uses and returns what the frontend receives.
func renderThroughGoKit(t *testing.T, err error) (codes.Code, map[string]any) {
	t.Helper()

	_, out := intrcp.UnaryServerErrorInterceptor()(
		context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/webitel.im.api.gateway.v1.Account/Token"},
		func(context.Context, any) (any, error) { return nil, err },
	)

	st, ok := status.FromError(out)
	if !ok {
		t.Fatalf("interceptor returned a non-status error: %v", out)
	}

	var body map[string]any
	if jerr := json.Unmarshal([]byte(st.Message()), &body); jerr != nil {
		t.Fatalf("grpc-message is not JSON: %q", st.Message())
	}

	return st.Code(), body
}

func bufferLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer

	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func TestProcessTokenErrorClientStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status     string
		wantDetail string
	}{
		{"CAPTCHA_REQUIRED", "api.auth.captcha_required"},
		{"CAPTCHA_FAILED", "api.auth.captcha_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()

			logger, logs := bufferLogger()
			upstream := status.Error(codes.Unauthenticated, tt.status)

			mapped := processTokenError(context.Background(), logger, upstream)
			if !errors.Is(mapped, upstream) {
				t.Errorf("mapped error lost its upstream cause")
			}

			code, body := renderThroughGoKit(t, mapped)
			if code != codes.Unauthenticated {
				t.Errorf("grpc code = %v, want Unauthenticated", code)
			}

			if body["detail"] != tt.wantDetail {
				t.Errorf("detail = %v, want %q", body["detail"], tt.wantDetail)
			}

			if body["code"] != float64(401) {
				t.Errorf("code = %v, want 401", body["code"])
			}

			if logs.Len() != 0 {
				t.Errorf("client captcha error must not be logged here, got %q", logs.String())
			}
		})
	}
}

func TestProcessTokenErrorServerStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   string
		upstream codes.Code
	}{
		{"CAPTCHA_UNAVAILABLE", codes.Unavailable},
		{"CAPTCHA_MISCONFIGURED", codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()

			logger, logs := bufferLogger()
			upstream := status.Error(tt.upstream, tt.status)

			mapped := processTokenError(context.Background(), logger, upstream)
			if !errors.Is(mapped, upstream) {
				t.Errorf("mapped error lost its upstream cause")
			}

			code, body := renderThroughGoKit(t, mapped)
			if code != codes.Internal {
				t.Errorf("grpc code = %v, want Internal", code)
			}

			if body["detail"] != errTokenServerMessage {
				t.Errorf("detail = %v, want %q", body["detail"], errTokenServerMessage)
			}

			if strings.Contains(body["detail"].(string), "CAPTCHA") || strings.Contains(body["id"].(string), "captcha") {
				t.Errorf("response exposes the captcha status: %v", body)
			}

			if !strings.Contains(logs.String(), tt.status) {
				t.Errorf("log must contain %s, got %q", tt.status, logs.String())
			}
		})
	}
}

func TestProcessTokenErrorLeavesOtherErrors(t *testing.T) {
	t.Parallel()

	logger, _ := bufferLogger()

	for _, err := range []error{
		status.Error(codes.Unauthenticated, "invalid issuer"),
		status.Error(codes.Internal, "CAPTCHA_REQUIRED but not exactly"),
		errors.New("CAPTCHA_REQUIRED"), // not a gRPC status
	} {
		if got := processTokenError(context.Background(), logger, err); got != err { //nolint:errorlint // identity is the point
			t.Errorf("processTokenError(%v) = %v, want the same error", err, got)
		}
	}
}
