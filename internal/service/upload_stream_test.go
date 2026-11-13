package service

import (
	"bufio"
	"context"
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

// rejectingStorage mimics im-storage ending SafeUploadFile with a file policy
// error once the first chunk arrives.
type rejectingStorage struct {
	storagev1.UnimplementedFileServiceServer
}

func (rejectingStorage) SafeUploadFile(stream grpc.BidiStreamingServer[storagev1.SafeUploadFileRequest, storagev1.SafeUploadFileResponse]) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		if req.GetChunk() != nil {
			return status.Error(codes.FailedPrecondition, "policy.file.allow")
		}
	}
}

func dialRejectingStorage(t *testing.T) storagev1.FileService_SafeUploadFileClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	storagev1.RegisterFileServiceServer(srv, rejectingStorage{})

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
		Data: &storagev1.SafeUploadFileRequest_Metadata_{Metadata: &storagev1.SafeUploadFileRequest_Metadata{Name: "a.bin"}},
	}); err != nil {
		t.Fatalf("send metadata: %v", err)
	}

	return stream
}

// endlessReader yields bytes forever so the chunk loop keeps sending until
// storage's rejection surfaces.
type endlessReader struct{}

func (endlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}

	return len(p), nil
}

// A mid-stream rejection must surface storage's status, not the io.EOF Send returns.
func TestStreamChunks_StorageRejectionKeepsStatus(t *testing.T) {
	t.Parallel()

	stream := dialRejectingStorage(t)

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
	if err == nil {
		t.Fatal("streamChunks: want storage rejection, got nil")
	}

	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("code = %v (err %v), want FailedPrecondition", got, err)
	}

	if got := status.Convert(err).Message(); got != "policy.file.allow" {
		t.Errorf("message = %q, want policy.file.allow", got)
	}
}

func TestFinalize_StorageRejectionKeepsStatus(t *testing.T) {
	t.Parallel()

	stream := dialRejectingStorage(t)

	if err := stream.Send(&storagev1.SafeUploadFileRequest{
		Data: &storagev1.SafeUploadFileRequest_Chunk{Chunk: []byte("x")},
	}); err != nil {
		t.Fatalf("send chunk: %v", err)
	}

	sess := newUploadSession("a.bin")
	if !sess.attachStream(stream, func() {}, func() {}, "upload-1") {
		t.Fatal("attach stream")
	}

	t.Cleanup(sess.terminate)

	_, err := sess.finalize()
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("code = %v (err %v), want FailedPrecondition", got, err)
	}
}

func TestUploadStreamErr_KeepsNonEOF(t *testing.T) {
	t.Parallel()

	want := status.Error(codes.Unavailable, "down")
	if got := uploadStreamErr(nil, want); got != want { //nolint:errorlint // identity check: must be returned as is
		t.Errorf("err = %v, want %v", got, want)
	}
}
