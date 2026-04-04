package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/internal/service"
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
	bodyLimitMW func(http.Handler) http.Handler,
	mux *http.ServeMux,
) *Handler {
	h := &Handler{
		logger: logger,
		media:  media,
	}
	h.registerRoutes(mux, authMW, bodyLimitMW)
	return h
}

func (h *Handler) registerRoutes(mux *http.ServeMux, authMW, bodyLimitMW func(http.Handler) http.Handler) {
	mux.Handle("GET /media/{id}/download", authMW(http.HandlerFunc(h.downloadFile)))
	mux.Handle("GET /media/{id}/stream", authMW(http.HandlerFunc(h.streamFile)))
	mux.Handle("GET /media", authMW(http.HandlerFunc(h.getUploadFileInfo)))
	mux.Handle("PUT /media", authMW(bodyLimitMW(http.HandlerFunc(h.uploadFile))))
	mux.Handle("POST /media", authMW(http.HandlerFunc(h.createUploadSession)))
}

type apiError struct {
	ID     string `json:"id"`
	Code   int32  `json:"code"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

func renderError(w http.ResponseWriter, httpCode int, id, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)

	_ = json.NewEncoder(w).Encode(&apiError{
		ID:     id,
		Code:   int32(httpCode),
		Detail: detail,
		Status: http.StatusText(httpCode),
	})
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var (
		httpCode int
		id       string
	)

	var maxBytesErr *http.MaxBytesError

	switch {
	case errors.As(err, &maxBytesErr):
		httpCode = http.StatusRequestEntityTooLarge
		id = "api.request_too_large"
	case errors.Is(err, auth.IdentityNotFoundErr):
		httpCode = http.StatusUnauthorized
		id = "api.unauthenticated"
	case errors.Is(err, service.ErrSessionNotFound):
		httpCode = http.StatusNotFound
		id = "api.not_found"
	case errors.Is(err, service.ErrSessionConflict), errors.Is(err, service.ErrSessionDone):
		httpCode = http.StatusConflict
		id = "api.conflict"
	default:
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				httpCode = http.StatusNotFound
				id = "api.not_found"
			case codes.Unauthenticated:
				httpCode = http.StatusUnauthorized
				id = "api.unauthenticated"
			case codes.PermissionDenied:
				httpCode = http.StatusForbidden
				id = "api.forbidden"
			case codes.InvalidArgument, codes.Aborted:
				httpCode = http.StatusBadRequest
				id = "api.bad_args"
			case codes.AlreadyExists:
				httpCode = http.StatusConflict
				id = "api.conflict"
			default:
				httpCode = http.StatusInternalServerError
				id = "api.internal"
			}
		} else {
			httpCode = http.StatusInternalServerError
			id = "api.internal"
		}
	}

	renderError(w, httpCode, id, err.Error())
}
