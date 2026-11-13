package http

import (
	"encoding/json"
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

			h.writeError(rec, tc.err)

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
