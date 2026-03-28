package http

import (
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/internal/service"
)

// Handler registers and serves HTTP endpoints for the IM media API.
type Handler struct {
	logger     *slog.Logger
	downloader service.MediaDownloader
}

func NewHandler(
	logger *slog.Logger,
	downloader service.MediaDownloader,
	authMW func(http.Handler) http.Handler,
	mux *http.ServeMux,
) *Handler {
	h := &Handler{
		logger:     logger,
		downloader: downloader,
	}
	h.registerRoutes(mux, authMW)
	return h
}

func (h *Handler) registerRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /im/media/{id}", authMW(http.HandlerFunc(h.downloadFile)))
	mux.Handle("GET /im/media/{id}/stream", authMW(http.HandlerFunc(h.streamFile)))
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	if err == auth.IdentityNotFoundErr {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

	h.logger.Error("media handler error", "err", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
