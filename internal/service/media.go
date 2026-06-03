package service

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	storageclient "github.com/webitel/im-gateway-service/infra/client/storage"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type Media interface {
	Download(ctx context.Context, req *dto.MediaDownloadRequest) (*dto.FileDownloadResult, error)
	CreateUploadSession(ctx context.Context, name string) (string, error)
	AppendContent(ctx context.Context, uploadID string, body io.Reader) (*dto.FileMetadata, error)
	GetUploadFileInfo(ctx context.Context, uploadID string) (int64, error)
}

var (
	ErrSessionNotFound = errors.New("upload session not found")
	ErrSessionConflict = errors.New("upload already in progress for this session")
	ErrSessionDone     = errors.New("upload session already complete or canceled")
	ErrEmptyBody       = errors.New("upload: empty body")
)

const (
	uploadIdleTimeout = 3 * time.Minute
	uploadMaxTTL      = 10 * time.Minute
	sniffSize         = 512
)

type MediaService struct {
	logger        *slog.Logger
	storageClient *storageclient.Client
	chunkSize     int

	mu       sync.Mutex
	sessions map[string]*uploadSession
}

func NewMediaService(logger *slog.Logger, storageClient *storageclient.Client, chunkSize int) Media {
	return &MediaService{
		logger:        logger,
		storageClient: storageClient,
		chunkSize:     chunkSize,
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

// CreateUploadSession allocates a gateway-side upload session and returns its ID.
// No storage RPC is performed here — the SafeUploadFile gRPC stream is opened
// lazily on the first chunk of AppendContent so the mime type can be sniffed
// from real bytes before the Metadata frame is sent.
func (s *MediaService) CreateUploadSession(ctx context.Context, name string) (string, error) {
	if _, ok := auth.GetIdentityFromContext(ctx); !ok {
		return "", auth.IdentityNotFoundErr
	}

	id := uuid.NewString()
	sess := newUploadSession(name)

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	s.logger.Debug("upload session created", slog.String("upload_id", id))

	// Cleanup goroutine: removes the session after it terminates or after the max TTL.
	go func() {
		select {
		case <-sess.terminateChan:
		case <-time.After(uploadMaxTTL):
		}

		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()

		_ = sess.terminate()

		s.logger.Debug("upload session removed", slog.String("upload_id", id))
	}()

	return id, nil
}

// AppendContent streams body chunks to storage. On the first call for a session
// the body is peeked to detect the mime type, then the SafeUploadFile stream is
// opened and the Metadata frame is sent with the sniffed mime. Subsequent bytes
// (including the peeked ones) are forwarded chunk-by-chunk.
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

	reader := bufio.NewReaderSize(body, s.chunkSize)

	if err := s.startStorageStream(ctx, sess, reader); err != nil {
		_ = sess.terminate()

		return nil, err
	}

	buf := make([]byte, s.chunkSize)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		n, readErr := reader.Read(buf)
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
			_ = sess.terminate()

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

// startStorageStream peeks the first 512 bytes of body, detects the mime type,
// opens the SafeUploadFile gRPC stream, and sends the Metadata frame with the
// sniffed mime. The peeked bytes remain in reader for the chunk loop to forward.
func (s *MediaService) startStorageStream(ctx context.Context, sess *uploadSession, reader *bufio.Reader) error {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}

	sniff, peekErr := reader.Peek(sniffSize)
	if peekErr != nil && peekErr != io.EOF {
		return peekErr
	}

	if len(sniff) == 0 {
		return ErrEmptyBody
	}

	mime := http.DetectContentType(sniff)

	streamCtx, cancelFn := context.WithCancel(context.Background())

	stream, releaseFn, err := s.storageClient.SafeUploadFile(streamCtx)
	if err != nil {
		cancelFn()

		return err
	}

	if err := stream.Send(&storagev1.SafeUploadFileRequest{
		Data: &storagev1.SafeUploadFileRequest_Metadata_{
			Metadata: &storagev1.SafeUploadFileRequest_Metadata{
				DomainId: identity.GetDomainID(),
				Name:     sess.name,
				MimeType: mime,
			},
		},
	}); err != nil {
		cancelFn()
		releaseFn()

		return err
	}

	msg, err := stream.Recv()
	if err != nil {
		cancelFn()
		releaseFn()

		return err
	}

	part := msg.GetPart()
	if part == nil || part.GetUploadId() == "" {
		cancelFn()
		releaseFn()

		return errors.New("storage: expected non-empty Part as first stream response")
	}

	if !sess.attachStream(stream, cancelFn, releaseFn) {
		return ErrSessionDone
	}

	return nil
}

// GetUploadFileInfo returns the number of bytes uploaded so far for the given
// session. Returns 0 if the storage stream has not yet been opened (no chunks
// uploaded via AppendContent).
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

// uploadSession holds upload state across the POST→PUT lifecycle. The gRPC
// stream to storage is attached lazily on the first AppendContent call, after
// the mime type has been sniffed from the body. A heartbeat goroutine guards
// against idle streams once attached.
type uploadSession struct {
	// Set at creation, never changes.
	name string

	mu        sync.Mutex
	writeLock sync.Mutex

	// Stream-side fields. nil/false until attachStream is called.
	stream    storagev1.FileService_SafeUploadFileClient
	cancelFn  context.CancelFunc
	releaseFn func()

	inactive      bool
	lastOffset    int64
	terminateOnce sync.Once

	// aliveChan receives a signal on each uploaded chunk to reset the idle timer.
	aliveChan chan struct{}

	// terminateChan is closed when the session is terminated, signaling all
	// waiting goroutines (heartbeat, cleanup) to exit.
	terminateChan chan struct{}
}

func newUploadSession(name string) *uploadSession {
	return &uploadSession{
		name:          name,
		aliveChan:     make(chan struct{}, 1),
		terminateChan: make(chan struct{}),
	}
}

// attachStream binds the freshly-opened storage stream to the session and
// starts its heartbeat. It returns false if the session was already terminated
// (idle timeout or max TTL) while the stream was being opened: in that case
// terminate() ran with no stream attached and will not run again (sync.Once),
// so the stream is released here to avoid leaking the gRPC stream and
// connection, and the caller must abort the upload.
func (s *uploadSession) attachStream(stream storagev1.FileService_SafeUploadFileClient, cancelFn context.CancelFunc, releaseFn func()) bool {
	s.mu.Lock()
	if s.inactive {
		s.mu.Unlock()

		cancelFn()
		releaseFn()

		return false
	}

	s.stream = stream
	s.cancelFn = cancelFn
	s.releaseFn = releaseFn
	s.mu.Unlock()

	go s.heartbeat()

	return true
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

// terminate cancels the underlying gRPC stream (if attached) and marks the
// session inactive. Safe to call multiple times and from multiple goroutines,
// and safe to call before a stream is attached.
func (s *uploadSession) terminate() error {
	s.terminateOnce.Do(func() {
		s.mu.Lock()
		if s.cancelFn != nil {
			s.cancelFn()
		}

		release := s.releaseFn
		s.inactive = true
		close(s.terminateChan)
		s.mu.Unlock()

		if release != nil {
			release()
		}
	})

	return nil
}

// finalize signals EOF to the storage server by sending an empty chunk, then
// receives the final file metadata. Only valid after a stream has been attached
// and at least one chunk has been forwarded.
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
