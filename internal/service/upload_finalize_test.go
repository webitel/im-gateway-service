package service

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
)

// finishingStorage mimics im-storage answering the end-of-upload chunk with
// either a final Metadata frame or a stream error.
type finishingStorage struct {
	storagev1.UnimplementedFileServiceServer

	meta *storagev1.SafeUploadFileResponse_Metadata
	err  error
	// failOnChunk ends the stream with err on the first data chunk, as
	// im-storage does when its file backend write fails mid-upload.
	failOnChunk bool
}

func (f finishingStorage) SafeUploadFile(stream grpc.BidiStreamingServer[storagev1.SafeUploadFileRequest, storagev1.SafeUploadFileResponse]) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		chunk := req.GetChunk()
		if len(chunk) > 0 && f.failOnChunk {
			return f.err
		}

		if chunk == nil || len(chunk) > 0 {
			continue
		}

		if f.err != nil {
			return f.err
		}

		return stream.Send(&storagev1.SafeUploadFileResponse{
			Data: &storagev1.SafeUploadFileResponse_Metadata_{Metadata: f.meta},
		})
	}
}

func dialFinishingStorage(t *testing.T, srvImpl finishingStorage) storagev1.FileService_SafeUploadFileClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	storagev1.RegisterFileServiceServer(srv, srvImpl)

	go func() { _ = srv.Serve(lis) }()

	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	stream, err := storagev1.NewFileServiceClient(conn).SafeUploadFile(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	if err := stream.Send(&storagev1.SafeUploadFileRequest{
		Data: &storagev1.SafeUploadFileRequest_Chunk{Chunk: []byte("payload")},
	}); err != nil {
		t.Fatalf("send chunk: %v", err)
	}

	return stream
}

func TestFinalize_StorageResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		storage  finishingStorage
		wantErr  error
		wantCode codes.Code
		wantID   string
	}{
		{
			name: "stored file",
			storage: finishingStorage{meta: &storagev1.SafeUploadFileResponse_Metadata{
				FileId: 42, Name: "a.txt", MimeType: "text/plain", Size: 7, Sha256Sum: "abc", Code: storagev1.UploadStatusCode_Ok,
			}},
			wantID: "42",
		},
		{
			// A storage build that leaves the status unset still reports a real file.
			name: "stored file, status unset",
			storage: finishingStorage{meta: &storagev1.SafeUploadFileResponse_Metadata{
				FileId: 7, Name: "a.txt", Sha256Sum: "abc",
			}},
			wantID: "7",
		},
		{
			// What im-storage sends today when writing to the file backend fails.
			name: "failed write reported as ok without file id",
			storage: finishingStorage{meta: &storagev1.SafeUploadFileResponse_Metadata{
				Name: "a.txt", MimeType: "text/plain", Code: storagev1.UploadStatusCode_Ok,
			}},
			wantErr: ErrUploadNotStored,
		},
		{
			name: "negative file id",
			storage: finishingStorage{meta: &storagev1.SafeUploadFileResponse_Metadata{
				FileId: -1, Name: "a.txt", Code: storagev1.UploadStatusCode_Ok,
			}},
			wantErr: ErrUploadNotStored,
		},
		{
			name: "failed status with a file id",
			storage: finishingStorage{meta: &storagev1.SafeUploadFileResponse_Metadata{
				FileId: 5, Name: "a.txt", Sha256Sum: "abc", Code: storagev1.UploadStatusCode_Failed,
			}},
			wantErr: ErrUploadNotStored,
		},
		{
			// im-storage with the fix: a failed write ends the stream with Internal.
			name:    "storage ends the stream with a write failure",
			storage: finishingStorage{err: status.Error(codes.Internal, `{"id":"utils.file.s3.writing.origin"}`)},
			wantErr: ErrUploadNotStored,
		},
		{
			name:     "storage policy rejection keeps its status",
			storage:  finishingStorage{err: status.Error(codes.FailedPrecondition, "policy.file.allow")},
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "storage antivirus rejection keeps its status",
			storage:  finishingStorage{err: status.Error(codes.PermissionDenied, `{"id":"file.malware"}`)},
			wantCode: codes.PermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sess := newUploadSession("a.txt")
			if !sess.attachStream(dialFinishingStorage(t, tc.storage), func() {}, func() {}, "upload-1") {
				t.Fatal("attach stream")
			}

			t.Cleanup(sess.terminate)

			meta, err := sess.finalize()

			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}

				if meta != nil {
					t.Errorf("meta = %+v, want nil", meta)
				}
			case tc.wantCode != codes.OK:
				if got := status.Code(err); got != tc.wantCode {
					t.Fatalf("code = %v (err %v), want %v", got, err, tc.wantCode)
				}

				if errors.Is(err, ErrUploadNotStored) {
					t.Errorf("err = %v: a rejection is not a failed write", err)
				}
			default:
				if err != nil {
					t.Fatalf("finalize: %v", err)
				}

				if meta.ID != tc.wantID {
					t.Errorf("id = %q, want %q", meta.ID, tc.wantID)
				}
			}
		})
	}
}

func TestStoreErr(t *testing.T) {
	t.Parallel()

	internal := status.Error(codes.Internal, "write failed")

	tests := []struct {
		name          string
		err           error
		wantNotStored bool
	}{
		{name: "storage internal status", err: internal, wantNotStored: true},
		{name: "storage unavailable", err: status.Error(codes.Unavailable, "down")},
		{name: "policy rejection", err: status.Error(codes.FailedPrecondition, "policy.file.allow")},
		// Not a status from storage: stream closed without a final frame.
		{name: "plain eof", err: io.EOF},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := storeErr(tc.err)

			if errors.Is(got, ErrUploadNotStored) != tc.wantNotStored {
				t.Fatalf("storeErr(%v) = %v, not-stored = %v, want %v", tc.err, got, errors.Is(got, ErrUploadNotStored), tc.wantNotStored)
			}

			// The cause stays reachable for logs.
			if !errors.Is(got, tc.err) {
				t.Errorf("storeErr(%v) = %v lost the cause", tc.err, got)
			}
		})
	}
}

// A write failure storage reports mid-upload is a file that was not stored,
// like the same failure after the last chunk.
func TestStreamChunks_StorageWriteFailureIsNotStored(t *testing.T) {
	t.Parallel()

	stream := dialFinishingStorage(t, finishingStorage{
		err:         status.Error(codes.Internal, `{"id":"utils.file.s3.writing.origin"}`),
		failOnChunk: true,
	})

	sess := newUploadSession("a.bin")
	if !sess.attachStream(stream, func() {}, func() {}, "upload-1") {
		t.Fatal("attach stream")
	}

	t.Cleanup(sess.terminate)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &MediaService{chunkSize: 1024}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := s.streamChunks(ctx, sess, bufio.NewReaderSize(endlessReader{}, s.chunkSize), log)
	if !errors.Is(err, ErrUploadNotStored) {
		t.Fatalf("streamChunks: err = %v, want ErrUploadNotStored", err)
	}

	if got := status.Code(err); got != codes.Internal {
		t.Errorf("code = %v, want the storage status kept as the cause", got)
	}
}

// eofStream is a SafeUploadFile client whose stream storage has already ended
// with status: Send sees io.EOF, Recv the status.
type eofStream struct {
	storagev1.FileService_SafeUploadFileClient

	status error
}

func (e eofStream) Send(*storagev1.SafeUploadFileRequest) error { return io.EOF }

func (e eofStream) Recv() (*storagev1.SafeUploadFileResponse, error) { return nil, e.status }

// Storage may fail the write and end the stream before the end-of-upload
// chunk is sent; finalize then learns it from Send, not Recv.
func TestFinalize_WriteFailureBeforeLastChunk(t *testing.T) {
	t.Parallel()

	sess := newUploadSession("a.bin")
	if !sess.attachStream(eofStream{status: status.Error(codes.Internal, "write failed")}, func() {}, func() {}, "upload-1") {
		t.Fatal("attach stream")
	}

	t.Cleanup(sess.terminate)

	if _, err := sess.finalize(); !errors.Is(err, ErrUploadNotStored) {
		t.Fatalf("finalize: err = %v, want ErrUploadNotStored", err)
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

// A body read failure is the client's connection, not storage.
func TestStreamChunks_BodyReadErrorIsMarked(t *testing.T) {
	t.Parallel()

	sess := newUploadSession("a.bin")
	if !sess.attachStream(dialFinishingStorage(t, finishingStorage{}), func() {}, func() {}, "upload-1") {
		t.Fatal("attach stream")
	}

	t.Cleanup(sess.terminate)

	s := &MediaService{chunkSize: 1024}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := s.streamChunks(context.Background(), sess, bufio.NewReaderSize(failingReader{err: io.ErrUnexpectedEOF}, s.chunkSize), log)
	if !errors.Is(err, ErrBodyRead) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("streamChunks: err = %v, want ErrBodyRead wrapping the read error", err)
	}

	if errors.Is(err, ErrUploadNotStored) {
		t.Errorf("err = %v: a body read failure is not a storage failure", err)
	}
}
