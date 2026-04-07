package middleware

import "net/http"

// WithBodyLimit returns middleware that caps the request body to maxBytes.
// If maxBytes <= 0, no limit is applied.
// When the limit is exceeded, http.MaxBytesReader returns *http.MaxBytesError
// which the handler can map to a 413 response.
func WithBodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if maxBytes <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
