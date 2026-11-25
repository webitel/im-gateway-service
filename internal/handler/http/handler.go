package http

import (
	"context"
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

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
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
	case errors.Is(err, service.ErrBodyRead):
		// The client dropped the connection mid-body (HTTP/1.1 unexpected
		// EOF, HTTP/2 stream reset); the session stays resumable from the
		// offset reported by GET /media.
		httpCode = http.StatusBadRequest
		id = "api.bad_args"
		detail = bodyCutOffDetail
	case errors.Is(err, service.ErrUploadNotStored):
		httpCode = http.StatusInternalServerError
		id = "api.upload_not_stored"
		detail = uploadNotStoredDetail
	default:
		// Own sentinel errors above name what the caller got wrong; anything
		// else may come from another service and carry its internals (storage
		// error JSON, backend hosts, bucket paths), so it gets a generic text.
		httpCode, id = http.StatusInternalServerError, "api.internal"
		detail = genericDetail(httpCode)

		if st, ok := status.FromError(err); ok {
			httpCode, id = statusToHTTP(st.Code())
			detail = upstreamDetail(st, httpCode)

			if rejection, ok := filePolicyRejection(st); ok {
				httpCode, id, detail = rejection.httpCode, rejection.id, rejection.detail
			}
		}
	}

	h.logError(r, httpCode, id, err)
	renderError(w, httpCode, id, detail)
}

// statusToHTTP maps a gRPC status from an upstream service to the HTTP status
// and error id of the media API.
func statusToHTTP(code codes.Code) (int, string) {
	switch code { //nolint:exhaustive // the remaining codes map to 500 via default
	case codes.NotFound:
		return http.StatusNotFound, "api.not_found"
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "api.unauthenticated"
	case codes.PermissionDenied:
		return http.StatusForbidden, "api.forbidden"
	case codes.InvalidArgument, codes.Aborted:
		return http.StatusBadRequest, "api.bad_args"
	case codes.AlreadyExists:
		return http.StatusConflict, "api.conflict"
	default:
		return http.StatusInternalServerError, "api.internal"
	}
}

// logError keeps the cause the client does not see. A call canceled because
// the client went away is not a server failure; a storage failure is logged
// even when the client stopped waiting for it.
func (h *Handler) logError(r *http.Request, httpCode int, id string, err error) {
	if httpCode < http.StatusInternalServerError || canceled(err) {
		return
	}

	attrs := []any{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("id", id),
		slog.String("error", err.Error()),
	}
	if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" {
		attrs = append(attrs, slog.String("upload_id", uploadID))
	}

	h.logger.ErrorContext(r.Context(), "media request failed", attrs...)
}

// logCopyError records a download that broke after the response had started:
// the status line is already sent, so the client only sees a cut body. A
// client that went away is not a server failure.
func (h *Handler) logCopyError(r *http.Request, err error) {
	if r.Context().Err() != nil || canceled(err) {
		return
	}

	h.logger.WarnContext(r.Context(), "media download interrupted",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("error", err.Error()))
}

// canceled reports an error caused by canceling the call.
func canceled(err error) bool {
	return errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled
}

func genericDetail(httpCode int) string {
	return http.StatusText(httpCode)
}

// storageMalwareID is the id im-storage reports (as PermissionDenied) for a
// file its antivirus check blocks, on upload and on download.
const storageMalwareID = `"file.malware"`

// upstreamDetail names the few upstream errors a user can act on; the rest
// get the generic status text.
func upstreamDetail(st *status.Status, httpCode int) string {
	if st.Code() == codes.PermissionDenied && strings.Contains(st.Message(), storageMalwareID) {
		return malwareDetail
	}

	return genericDetail(httpCode)
}

const (
	uploadNotStoredDetail = "File upload failed, try again later"
	bodyCutOffDetail      = "Request body ended unexpectedly"
	malwareDetail         = "File is blocked by the antivirus check"
)

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
	if st.Code() != codes.FailedPrecondition {
		return policyRejection{}, false
	}

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
