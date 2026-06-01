package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{w, http.StatusOK}
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrappedWriter := newResponseWriterWrapper(w)

			next.ServeHTTP(wrappedWriter, r)

			if wrappedWriter.statusCode >= 400 {
				duration := time.Since(start)

				level := slog.LevelWarn
				if wrappedWriter.statusCode >= 500 {
					level = slog.LevelError
				}

				logger.Log(r.Context(), level, "http request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"status", wrappedWriter.statusCode,
					"duration_ms", duration.Milliseconds(),
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
				)
			}
		})
	}
}
