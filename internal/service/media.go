package service

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	storageclient "github.com/webitel/im-gateway-service/infra/client/storage"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type Media interface {
	Download(ctx context.Context, req *dto.MediaDownloadRequest) (*dto.FileDownloadResult, error)
	CreateUploadSession(ctx context.Context, name, mimeType string) (string, error)
	AppendContent(ctx context.Context, uploadID string, body io.Reader) (*dto.FileMetadata, error)
	GetUploadFileInfo(ctx context.Context, uploadID string) (int64, error)
}

var (
	ErrSessionNotFound = errors.New("upload session not found")
	ErrSessionConflict = errors.New("upload already in progress for this session")
	ErrSessionDone     = errors.New("upload session already complete or cancelled")
)

const (
	uploadIdleTimeout = 3 * time.Minute
	uploadMaxTTL      = 10 * time.Minute
)

type MediaService struct {
	logger        *slog.Logger
	storageClient *storageclient.Client

	mu       sync.Mutex
	sessions map[string]*uploadSession
}

func NewMediaService(logger *slog.Logger, storageClient *storageclient.Client) Media {
	return &MediaService{
		logger:        logger,
		storageClient: storageClient,
		sessions:      make(map[string]*uploadSession),
	}
}

// Download opens a server-streaming gRPC call to the storage service and returns
// the file metadata along with an io.ReadCloser over the remaining chunk stream.
// The caller must close the returned Body to release the underlying gRPC stream.
func (s *MediaService) Download(ctx context.Context, req *dto.MediaDownloadRequest) (*dto.FileDownloadResult, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	streamCtx, cancel := context.WithCancel(ctx)

	stream, err := s.storageClient.DownloadFile(streamCtx, &storagev1.DownloadFileRequest{
		Id:         req.FileID,
		DomainId:   identity.GetDomainID(),
		Metadata:   true,
		Offset:     req.Offset,
		BufferSize: 32768,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	// The first message from the storage service must be metadata.
	firstMsg, err := stream.Recv()
	if err != nil {
		cancel()
		return nil, err
	}

	meta := firstMsg.GetMetadata()
	if meta == nil {
		cancel()
		return nil, errors.New("storage: expected metadata as first stream message")
	}

	return &dto.FileDownloadResult{
		Metadata: &dto.FileMetadata{
			ID:       strconv.FormatInt(meta.GetId(), 10),
			Name:     meta.GetName(),
			MimeType: meta.GetMimeType(),
			Size:     meta.GetSize(),
		},
		Body: &streamReader{
			stream:   stream,
			cancelFn: cancel,
		},
	}, nil
}

// CreateUploadSession opens a SafeUploadFile gRPC stream, registers the upload
// with the storage service, and returns the storage-assigned upload ID.
// The stream is kept alive by a background heartbeat until AppendContent is
// called or the session times out.
func (s *MediaService) CreateUploadSession(ctx context.Context, name, mimeType string) (string, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return "", auth.IdentityNotFoundErr
	}

	streamCtx, cancelFn := context.WithCancel(context.Background())

	stream, releaseFn, err := s.storageClient.SafeUploadFile(streamCtx)
	if err != nil {
		cancelFn()
		return "", err
	}

	if err := stream.Send(&storagev1.SafeUploadFileRequest{
		Data: &storagev1.SafeUploadFileRequest_Metadata_{
			Metadata: &storagev1.SafeUploadFileRequest_Metadata{
				DomainId: identity.GetDomainID(),
				Name:     name,
				MimeType: mimeType,
			},
		},
	}); err != nil {
		cancelFn()
		releaseFn()
		return "", err
	}

	msg, err := stream.Recv()
	if err != nil {
		cancelFn()
		releaseFn()
		return "", err
	}

	part := msg.GetPart()
	if part == nil {
		cancelFn()
		releaseFn()
		return "", errors.New("storage: expected Part as first stream response")
	}

	uploadID := part.GetUploadId()
	if uploadID == "" {
		cancelFn()
		releaseFn()
		return "", errors.New("storage: received empty upload id")
	}

	sess := newUploadSession(stream, cancelFn, releaseFn)

	s.mu.Lock()
	s.sessions[uploadID] = sess
	s.mu.Unlock()

	s.logger.Debug("upload session created", slog.String("upload_id", uploadID))

	// Cleanup goroutine: removes the session after it terminates or after the max TTL.
	go func() {
		select {
		case <-sess.terminateChan:
		case <-time.After(uploadMaxTTL):
		}
		s.mu.Lock()
		delete(s.sessions, uploadID)
		s.mu.Unlock()
		_ = sess.terminate()
		s.logger.Debug("upload session removed", slog.String("upload_id", uploadID))
	}()

	return uploadID, nil
}

// AppendContent streams body chunk-by-chunk over the already-open gRPC stream
// for the given uploadID, then finalizes the upload and returns the stored file's metadata.
func (s *MediaService) AppendContent(ctx context.Context, uploadID string, body io.Reader) (*dto.FileMetadata, error) {
	s.mu.Lock()
	sess, found := s.sessions[uploadID]
	s.mu.Unlock()

	if !found {
		return nil, ErrSessionNotFound
	}

	sess.writeLock.Lock()
	defer sess.writeLock.Unlock()

	if !sess.isActive() {
		return nil, ErrSessionDone
	}

	buf := make([]byte, 4*1024)
	for {
		select {
		case <-ctx.Done():
			_ = sess.terminate()
			return nil, ctx.Err()
		default:
		}

		n, readErr := body.Read(buf)
		if n > 0 {
			if sendErr := sess.stream.Send(&storagev1.SafeUploadFileRequest{
				Data: &storagev1.SafeUploadFileRequest_Chunk{Chunk: buf[:n]},
			}); sendErr != nil {
				_ = sess.terminate()
				return nil, sendErr
			}

			// Signal the heartbeat goroutine that the session is still active.
			select {
			case sess.aliveChan <- struct{}{}:
			default:
			}

			sess.mu.Lock()
			sess.lastOffset += int64(n)
			sess.mu.Unlock()
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}

	meta, err := sess.finalize()
	if err != nil {
		_ = sess.terminate()
		return nil, err
	}

	_ = sess.terminate()

	s.mu.Lock()
	delete(s.sessions, uploadID)
	s.mu.Unlock()

	return meta, nil
}

// GetUploadFileInfo returns the number of bytes uploaded so far for the given session.
func (s *MediaService) GetUploadFileInfo(ctx context.Context, uploadID string) (int64, error) {
	s.mu.Lock()
	sess, found := s.sessions[uploadID]
	s.mu.Unlock()

	if !found {
		return 0, ErrSessionNotFound
	}
	if !sess.isActive() {
		return 0, ErrSessionDone
	}

	sess.mu.Lock()
	offset := sess.lastOffset
	sess.mu.Unlock()
	return offset, nil
}

// streamReader adapts a gRPC server-streaming FileService_DownloadFileClient
// into an io.ReadCloser by buffering one chunk message at a time.
type streamReader struct {
	stream   storagev1.FileService_DownloadFileClient
	buf      []byte
	pos      int
	cancelFn context.CancelFunc
}

func (r *streamReader) Read(p []byte) (int, error) {
	// Drain the current buffer before fetching the next message.
	if r.pos < len(r.buf) {
		n := copy(p, r.buf[r.pos:])
		r.pos += n
		return n, nil
	}

	msg, err := r.stream.Recv()
	if err != nil {
		return 0, err // includes io.EOF on stream end
	}

	chunk := msg.GetChunk()
	if chunk == nil {
		// Unexpected metadata frame in the middle of the stream; skip it.
		return r.Read(p)
	}

	r.buf = chunk
	r.pos = 0
	n := copy(p, r.buf)
	r.pos += n
	return n, nil
}

func (r *streamReader) Close() error {
	if r.cancelFn != nil {
		r.cancelFn()
	}
	return nil
}

// uploadSession holds an open SafeUploadFile gRPC stream across HTTP requests.
// The stream is opened and registered with the storage service in CreateUploadSession,
// then reused in AppendContent. A heartbeat goroutine keeps the stream alive during idle periods.
type uploadSession struct {
	stream    storagev1.FileService_SafeUploadFileClient
	cancelFn  context.CancelFunc
	releaseFn func() // returns the gRPC connection back to the pool

	mu            sync.Mutex
	writeLock     sync.Mutex
	inactive      bool
	lastOffset    int64
	terminateOnce sync.Once

	// aliveChan receives a signal on each uploaded chunk to reset the idle timer.
	// It is never closed — abandoned when the session terminates.
	aliveChan chan struct{}

	// terminateChan is closed when the session is terminated, signaling all
	// waiting goroutines (heartbeat, cleanup) to exit.
	terminateChan chan struct{}
}

func newUploadSession(stream storagev1.FileService_SafeUploadFileClient, cancelFn context.CancelFunc, releaseFn func()) *uploadSession {
	sess := &uploadSession{
		stream:        stream,
		cancelFn:      cancelFn,
		releaseFn:     releaseFn,
		aliveChan:     make(chan struct{}, 1),
		terminateChan: make(chan struct{}),
	}
	go sess.heartbeat()
	return sess
}

func (s *uploadSession) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.inactive
}

// heartbeat terminates the session after uploadIdleTimeout of inactivity.
func (s *uploadSession) heartbeat() {
	ticker := time.NewTicker(uploadIdleTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-s.terminateChan:
			return
		case <-s.aliveChan:
			ticker.Reset(uploadIdleTimeout)
		case <-ticker.C:
			_ = s.terminate()
			return
		}
	}
}

// terminate cancels the underlying gRPC stream and marks the session inactive.
// Safe to call multiple times and from multiple goroutines.
func (s *uploadSession) terminate() error {
	s.terminateOnce.Do(func() {
		s.mu.Lock()
		s.cancelFn()
		s.inactive = true
		close(s.terminateChan)
		s.mu.Unlock()
		s.releaseFn()
	})
	return nil
}

// finalize signals EOF to the storage server by sending an empty chunk, then
// receives the final file metadata. It must be called after all data chunks
// have been sent.
func (s *uploadSession) finalize() (*dto.FileMetadata, error) {
	if err := s.stream.Send(&storagev1.SafeUploadFileRequest{
		Data: &storagev1.SafeUploadFileRequest_Chunk{},
	}); err != nil {
		return nil, err
	}

	msg, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}

	meta := msg.GetMetadata()
	if meta == nil {
		return nil, errors.New("storage: expected Metadata as final stream response")
	}

	return &dto.FileMetadata{
		ID:       strconv.FormatInt(meta.GetFileId(), 10),
		Name:     meta.GetName(),
		MimeType: meta.GetMimeType(),
		Size:     meta.GetSize(),
		Hash:     meta.GetSha256Sum(),
	}, nil
}
