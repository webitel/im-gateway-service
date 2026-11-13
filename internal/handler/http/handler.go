package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
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
	mux.Handle("DELETE /media", authMW(http.HandlerFunc(h.terminateUploadSession)))
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
		detail   = err.Error()
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
	case errors.Is(err, service.ErrEmptyBody):
		httpCode = http.StatusBadRequest
		id = "api.bad_args"
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
			case codes.FailedPrecondition:
				if rejection, ok := filePolicyRejection(st); ok {
					httpCode, id, detail = rejection.httpCode, rejection.id, rejection.detail
				} else {
					httpCode = http.StatusInternalServerError
					id = "api.internal"
				}
			default:
				httpCode = http.StatusInternalServerError
				id = "api.internal"
			}
		} else {
			httpCode = http.StatusInternalServerError
			id = "api.internal"
		}
	}

	renderError(w, httpCode, id, detail)
}

// storageFilePolicyID is the id im-storage reports when an upload breaks the
// domain's file policy (size limit, file type, channel).
const storageFilePolicyID = "policy.file.allow"

// storageMaxSizeReason is the reason im-storage gives for a size-limit rejection.
const storageMaxSizeReason = "max size"

type policyRejection struct {
	httpCode int
	id       string
	detail   string
}

var (
	fileTooLarge = policyRejection{
		httpCode: http.StatusRequestEntityTooLarge,
		id:       "api.file_too_large",
		detail:   "File exceeds the maximum upload size allowed for this domain",
	}
	fileRejected = policyRejection{
		httpCode: http.StatusUnprocessableEntity,
		id:       "api.file_rejected",
		detail:   "File is not allowed by the domain's file policy",
	}
)

// filePolicyRejection recognizes a storage file policy FailedPrecondition.
// im-storage today sends only the bare id ("policy.file.allow") as the message,
// so the reason is lost and every rejection maps to 422. A size rejection is
// recognized when the reason ("max size") reaches the gateway, either appended
// to the message or carried in ErrorInfo / PreconditionFailure details.
func filePolicyRejection(st *status.Status) (policyRejection, bool) {
	texts := []string{st.Message()}

	for _, d := range st.Details() {
		switch d := d.(type) {
		case *errdetails.ErrorInfo:
			texts = append(texts, d.GetReason())
			for _, v := range d.GetMetadata() {
				texts = append(texts, v)
			}
		case *errdetails.PreconditionFailure:
			for _, v := range d.GetViolations() {
				texts = append(texts, v.GetType(), v.GetSubject(), v.GetDescription())
			}
		}
	}

	all := strings.Join(texts, "\n")
	if !strings.Contains(all, storageFilePolicyID) {
		return policyRejection{}, false
	}

	if strings.Contains(all, storageMaxSizeReason) {
		return fileTooLarge, true
	}

	return fileRejected, true
}
