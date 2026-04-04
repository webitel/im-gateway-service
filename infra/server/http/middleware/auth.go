package middleware

import (
	"log/slog"
	"net/http"

	"google.golang.org/grpc/metadata"

	"github.com/webitel/im-gateway-service/infra/auth"
)

// NewAuthMiddleware returns an HTTP middleware that bridges HTTP request headers into
// gRPC incoming metadata, then delegates to the existing Authorizer.SetIdentity.
// This reuses the same auth logic as the gRPC interceptor without reimplementation.
func NewAuthMiddleware(authorizer auth.Authorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			md := metadata.MD{}
			if v := r.Header.Get("Authorization"); v != "" {
				md["authorization"] = []string{v}
			}
			if v := r.Header.Get("X-Webitel-Access"); v != "" {
				md["x-webitel-access"] = []string{v}
			}
			if v := r.Header.Get("X-Webitel-Device"); v != "" {
				md["x-webitel-device"] = []string{v}
			}
			if v := r.Header.Get("X-Webitel-Client"); v != "" {
				md["x-webitel-client"] = []string{v}
			}

			// Inject as gRPC incoming metadata so SetIdentity can read it.
			ctx := metadata.NewIncomingContext(r.Context(), md)

			newCtx, err := authorizer.SetIdentity(ctx)
			if err != nil {
				slog.Error("auth failed", "error", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(newCtx))
		})
	}
}
