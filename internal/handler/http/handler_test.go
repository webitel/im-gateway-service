package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/webitel/im-gateway-service/internal/service"
)

func withDetails(t *testing.T, st *status.Status, details ...*errdetails.ErrorInfo) error {
	t.Helper()

	for _, d := range details {
		var err error
		if st, err = st.WithDetails(d); err != nil {
			t.Fatalf("attaching details: %v", err)
		}
	}

	return st.Err()
}

func TestWriteError_StorageFilePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantID     string
		wantDetail string
	}{
		{
			// Exactly what im-storage sends today: status.Error(FailedPrecondition, err.GetId()).
			name:       "storage today: bare policy id",
			err:        status.Error(codes.FailedPrecondition, "policy.file.allow"),
			wantCode:   http.StatusUnprocessableEntity,
			wantID:     "api.file_rejected",
			wantDetail: fileRejected.detail,
		},
		{
			name:       "storage with reason in message: size limit",
			err:        status.Error(codes.FailedPrecondition, "policy.file.allow: max size"),
			wantCode:   http.StatusRequestEntityTooLarge,
			wantID:     "api.file_too_large",
			wantDetail: fileTooLarge.detail,
		},
		{
			name:       "storage with reason in message: extension",
			err:        status.Error(codes.FailedPrecondition, "policy.file.allow: file extension is not allowed"),
			wantCode:   http.StatusUnprocessableEntity,
			wantID:     "api.file_rejected",
			wantDetail: fileRejected.detail,
		},
		{
			name: "storage with reason in ErrorInfo details: size limit",
			err: withDetails(t, status.New(codes.FailedPrecondition, "policy.file.allow"),
				&errdetails.ErrorInfo{Reason: "policy.file.allow", Metadata: map[string]string{"detail": "max size"}}),
			wantCode:   http.StatusRequestEntityTooLarge,
			wantID:     "api.file_too_large",
			wantDetail: fileTooLarge.detail,
		},
		{
			name:       "wrapped policy rejection",
			err:        fmt.Errorf("uploading part: %w", status.Error(codes.FailedPrecondition, "policy.file.allow")),
			wantCode:   http.StatusUnprocessableEntity,
			wantID:     "api.file_rejected",
			wantDetail: fileRejected.detail,
		},
		{
			name:     "other failed precondition stays internal",
			err:      status.Error(codes.FailedPrecondition, "bucket not ready"),
			wantCode: http.StatusInternalServerError,
			wantID:   "api.internal",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &Handler{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			rec := httptest.NewRecorder()

			h.writeError(rec, httptest.NewRequest(http.MethodPut, "/media?uploadId=test-upload", nil), tc.err)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}

			var body apiError
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}

			if body.ID != tc.wantID {
				t.Errorf("id = %q, want %q", body.ID, tc.wantID)
			}

			if tc.wantDetail == "" {
				return
			}

			if body.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", body.Detail, tc.wantDetail)
			}

			for _, leak := range []string{"rpc error", "desc =", "policy.file.allow"} {
				if strings.Contains(body.Detail, leak) {
					t.Errorf("detail %q leaks internal error text %q", body.Detail, leak)
				}
			}
		})
	}
}

func TestWriteError_UpstreamErrorsHideInternals(t *testing.T) {
	t.Parallel()

	storageJSON := `{"id":"utils.file.s3.writing.origin","detailed_error":"Put \"https://s3.example.com/1/2026/a.txt\": dial tcp: no such host"}`

	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantID     string
		wantDetail string
		wantLog    bool
	}{
		{
			name:       "storage did not store the upload",
			err:        fmt.Errorf("finalize: %w", service.ErrUploadNotStored),
			wantCode:   http.StatusInternalServerError,
			wantID:     "api.upload_not_stored",
			wantDetail: uploadNotStoredDetail,
			wantLog:    true,
		},
		{
			// What the gateway's finalize makes of storage failing the write.
			name:       "storage write failure carried with the sentinel",
			err:        fmt.Errorf("%w: %w", service.ErrUploadNotStored, status.Error(codes.Internal, storageJSON)),
			wantCode:   http.StatusInternalServerError,
			wantID:     "api.upload_not_stored",
			wantDetail: uploadNotStoredDetail,
			wantLog:    true,
		},
		{
			name:       "storage internal status with backend details",
			err:        status.Error(codes.Internal, storageJSON),
			wantCode:   http.StatusInternalServerError,
			wantID:     "api.internal",
			wantDetail: "Internal Server Error",
			wantLog:    true,
		},
		{
			name:       "plain internal error",
			err:        errors.New("storage: expected Metadata as final stream response"),
			wantCode:   http.StatusInternalServerError,
			wantID:     "api.internal",
			wantDetail: "Internal Server Error",
			wantLog:    true,
		},
		{
			name:       "storage not found with its error json",
			err:        status.Error(codes.NotFound, `{"id":"app.file.get.not_found","detailed_error":"sql: no rows"}`),
			wantCode:   http.StatusNotFound,
			wantID:     "api.not_found",
			wantDetail: "Not Found",
		},
		{
			name:       "storage antivirus block stays forbidden with its reason",
			err:        status.Error(codes.PermissionDenied, `{"id":"file.malware","status":"Forbidden","detail":"Malicious software"}`),
			wantCode:   http.StatusForbidden,
			wantID:     "api.forbidden",
			wantDetail: malwareDetail,
		},
		{
			name:       "other permission denied",
			err:        status.Error(codes.PermissionDenied, `{"id":"app.file.access"}`),
			wantCode:   http.StatusForbidden,
			wantID:     "api.forbidden",
			wantDetail: "Forbidden",
		},
		{
			// A policy id under another code is not a policy rejection.
			name:       "policy id outside failed precondition",
			err:        status.Error(codes.Internal, "policy.file.allow"),
			wantCode:   http.StatusInternalServerError,
			wantID:     "api.internal",
			wantDetail: "Internal Server Error",
			wantLog:    true,
		},
		{
			name:       "client cut the upload body (HTTP/1.1)",
			err:        fmt.Errorf("%w: %w", service.ErrBodyRead, io.ErrUnexpectedEOF),
			wantCode:   http.StatusBadRequest,
			wantID:     "api.bad_args",
			wantDetail: bodyCutOffDetail,
		},
		{
			name:       "client reset the upload stream (HTTP/2)",
			err:        fmt.Errorf("%w: %w", service.ErrBodyRead, errors.New("http2: stream closed")),
			wantCode:   http.StatusBadRequest,
			wantID:     "api.bad_args",
			wantDetail: bodyCutOffDetail,
		},
		{
			name:     "body over the limit stays 413",
			err:      fmt.Errorf("%w: %w", service.ErrBodyRead, &http.MaxBytesError{Limit: 10}),
			wantCode: http.StatusRequestEntityTooLarge,
			wantID:   "api.request_too_large",
			// The limit text is the gateway's own.
			wantDetail: fmt.Errorf("%w: %w", service.ErrBodyRead, &http.MaxBytesError{Limit: 10}).Error(),
		},
		{
			// Client errors keep their detail: it names what the caller got wrong.
			name:       "session not found keeps its detail",
			err:        service.ErrSessionNotFound,
			wantCode:   http.StatusNotFound,
			wantID:     "api.not_found",
			wantDetail: service.ErrSessionNotFound.Error(),
		},
		{
			name:       "canceled upstream call is not logged",
			err:        status.Error(codes.Canceled, "context canceled"),
			wantCode:   http.StatusInternalServerError,
			wantID:     "api.internal",
			wantDetail: "Internal Server Error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var logs strings.Builder

			h := &Handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}
			rec := httptest.NewRecorder()

			h.writeError(rec, httptest.NewRequest(http.MethodPut, "/media?uploadId=test-upload", nil), tc.err)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}

			var body apiError
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}

			if body.ID != tc.wantID {
				t.Errorf("id = %q, want %q", body.ID, tc.wantID)
			}

			if body.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", body.Detail, tc.wantDetail)
			}

			for _, leak := range []string{"rpc error", "s3.example.com", "detailed_error", "Metadata"} {
				if strings.Contains(body.Detail, leak) {
					t.Errorf("detail %q leaks internal error text %q", body.Detail, leak)
				}
			}

			logged := strings.Contains(logs.String(), "media request failed")
			if logged != tc.wantLog {
				t.Errorf("logged = %v, want %v (log: %s)", logged, tc.wantLog, logs.String())
			}

			if tc.wantLog && !strings.Contains(logs.String(), "upload_id=test-upload") {
				t.Errorf("log %q lacks the upload id", logs.String())
			}
		})
	}
}

func TestWriteError_StorageFailureLoggedAfterClientLeft(t *testing.T) {
	t.Parallel()

	var logs strings.Builder

	h := &Handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodPut, "/media?uploadId=test-upload", nil).WithContext(ctx)

	// Storage answered after the client stopped waiting: still a server failure.
	h.writeError(httptest.NewRecorder(), req, service.ErrUploadNotStored)

	if !strings.Contains(logs.String(), "media request failed") {
		t.Errorf("storage failure not logged: %q", logs.String())
	}
}

func TestWriteError_CanceledCallIsNotLogged(t *testing.T) {
	t.Parallel()

	var logs strings.Builder

	h := &Handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}

	for _, err := range []error{context.Canceled, fmt.Errorf("streaming: %w", context.Canceled)} {
		h.writeError(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/media?uploadId=test-upload", nil), err)
	}

	if logs.Len() != 0 {
		t.Errorf("logged %q for a canceled call", logs.String())
	}
}

func TestLogCopyError_ClientGoneIsNotLogged(t *testing.T) {
	t.Parallel()

	var logs strings.Builder

	h := &Handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h.logCopyError(httptest.NewRequest(http.MethodGet, "/media/1/download", nil).WithContext(ctx), errors.New("write: broken pipe"))

	if logs.Len() != 0 {
		t.Errorf("logged %q for a client that went away", logs.String())
	}
}

func TestLogCopyError_LogsServerSideBreak(t *testing.T) {
	t.Parallel()

	var logs strings.Builder

	h := &Handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}

	h.logCopyError(httptest.NewRequest(http.MethodGet, "/media/1/stream", nil), status.Error(codes.Unavailable, "storage down"))

	if !strings.Contains(logs.String(), "media download interrupted") || !strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("log = %q, want a WARN about the interrupted download", logs.String())
	}
}
