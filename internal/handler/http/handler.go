package http

import (
	"log/slog"
	"net/http"

	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/internal/service"
	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handler registers and serves HTTP endpoints for the IM media API.
type Handler struct {
	logger *slog.Logger
	media  service.Media
}

func NewHandler(
	logger *slog.Logger,
	media service.Media,
	authMW func(http.Handler) http.Handler,
	mux *http.ServeMux,
) *Handler {
	h := &Handler{
		logger: logger,
		media:  media,
	}
	h.registerRoutes(mux, authMW)
	return h
}

func (h *Handler) registerRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /media/{id}", authMW(http.HandlerFunc(h.downloadFile)))
	mux.Handle("GET /media/{id}/stream", authMW(http.HandlerFunc(h.streamFile)))
	mux.Handle("GET /media", authMW(http.HandlerFunc(h.getUploadFileInfo)))
	mux.Handle("PUT /media", authMW(http.HandlerFunc(h.uploadFile)))
	mux.Handle("POST /media", authMW(http.HandlerFunc(h.createUploadSession)))
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.IdentityNotFoundErr) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if errors.Is(err, service.ErrSessionNotFound) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if errors.Is(err, service.ErrSessionConflict) || errors.Is(err, service.ErrSessionDone) {
		http.Error(w, "Conflict", http.StatusConflict)
		return
	}

	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		case codes.Unauthenticated, codes.PermissionDenied:
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		case codes.InvalidArgument:
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}

	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
