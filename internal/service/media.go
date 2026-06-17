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
	"github.com/webitel/webitel-go-kit/pkg/semconv"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	storageclient "github.com/webitel/im-gateway-service/infra/client/storage"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type Media interface {
	Download(ctx context.Context, req *dto.MediaDownloadRequest) (*dto.FileDownloadResult, error)
	CreateUploadSession(ctx context.Context, name string) (string, error)
	AppendContent(ctx context.Context, uploadID string, body io.Reader) (*dto.FileMetadata, error)
	TerminateUploadSession(uploadID string) error
	GetUploadFileInfo(ctx context.Context, uploadID string) (int64, error)
}

var (
	ErrSessionNotFound = errors.New("upload session not found")
	ErrSessionConflict = errors.New("upload already in progress for this session")
	ErrSessionDone     = errors.New("upload session already complete or canceled")
	ErrEmptyBody       = errors.New("upload: empty body")
)

const (
	uploadMaxTTL = 10 * time.Minute
	sniffSize    = 512
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
	log := s.logger.With(slog.String("upload_id", id))

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	log.Debug("upload session created")

	go func() {
		select {
		case <-sess.terminateChan:
		case <-time.After(uploadMaxTTL):
			sess.terminate()
		}

		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()

		log.Debug("upload session removed")
	}()

	return id, nil
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

	if sess.isActive() {
		return sess.offset(), nil
	}

	storageID := sess.storageID()
	if storageID == "" {
		return 0, ErrSessionDone
	}

	return s.storageClient.GetUploadInfo(ctx, storageID)
}

// AppendContent streams body chunks to storage. On the first call for a session
// the body is peeked to detect the mime type, then the SafeUploadFile stream is
// opened and the Metadata frame is sent with the sniffed mime. Subsequent bytes
// (including the peeked ones) are forwarded chunk-by-chunk.
func (s *MediaService) AppendContent(ctx context.Context, uploadID string, body io.Reader) (*dto.FileMetadata, error) {
	log := s.logger.With(slog.String("upload_id", uploadID))

	s.mu.Lock()
	sess, found := s.sessions[uploadID]
	s.mu.Unlock()

	if !found {
		log.Debug("append content: session not found")

		return nil, ErrSessionNotFound
	}

	sess.writeLock.Lock()
	defer sess.writeLock.Unlock()

	if !sess.isActive() {
		log.Debug("append content: session already inactive")

		return nil, ErrSessionDone
	}

	log.Debug("append content started")

	reader := bufio.NewReaderSize(body, s.chunkSize)

	if sess.stream == nil {
		if err := s.startStorageStream(ctx, sess, reader); err != nil {
			log.Debug("append content: failed to start storage stream", slog.String(semconv.ErrorKey, err.Error()))

			return nil, err
		}
	}

	if err := s.streamChunks(ctx, sess, reader, log); err != nil {
		return nil, err
	}

	log.Debug("append content: body fully read, finalizing", slog.Int64("bytes_streamed", sess.offset()))

	meta, err := sess.finalize()
	if err != nil {
		log.Debug("append content: finalize failed", slog.String(semconv.ErrorKey, err.Error()))

		sess.terminate()

		return nil, err
	}

	log.Debug("append content: upload finalized",
		slog.String("file_id", meta.ID),
		slog.Int64("size", meta.Size),
		slog.String("mime_type", meta.MimeType))

	sess.terminate()

	return meta, nil
}

// TerminateUploadSession cancels an in-progress upload session, marking it
// inactive and aborting the underlying storage stream if one was attached. The
// partial upload is discarded rather than finalized, and the session is removed
// from the registry by its janitor goroutine. Returns ErrSessionNotFound if no
// session exists for the given ID.
func (s *MediaService) TerminateUploadSession(uploadID string) error {
	s.mu.Lock()
	sess, found := s.sessions[uploadID]
	s.mu.Unlock()

	if !found {
		return ErrSessionNotFound
	}

	sess.terminate()

	s.logger.Debug("upload session terminated by user", slog.String("upload_id", uploadID))

	return nil
}

// streamChunks forwards body from reader to the storage stream chunk-by-chunk,
// updating the session offset and pinging the heartbeat after each chunk. It
// returns nil when the body is exhausted, or the first send/read/context error.
// A send error terminates the session (the storage stream is broken); a read
// error leaves the session resumable.
func (s *MediaService) streamChunks(ctx context.Context, sess *uploadSession, reader *bufio.Reader, log *slog.Logger) error {
	buf := make([]byte, s.chunkSize)

	for {
		select {
		case <-ctx.Done():
			log.Debug("append content: context canceled", slog.String(semconv.ErrorKey, ctx.Err().Error()))

			return ctx.Err()
		default:
		}

		n, readErr := reader.Read(buf)
		if n > 0 {
			if sendErr := sess.stream.Send(&storagev1.SafeUploadFileRequest{
				Data: &storagev1.SafeUploadFileRequest_Chunk{Chunk: buf[:n]},
			}); sendErr != nil {
				log.Debug("append content: failed to send chunk to storage",
					slog.Int("chunk_bytes", n), slog.String(semconv.ErrorKey, sendErr.Error()))

				sess.terminate()

				return sendErr
			}

			// Signal the heartbeat goroutine that the session is still active.
			select {
			case sess.aliveChan <- struct{}{}:
			default:
			}

			sess.addOffset(n)
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}

			log.Debug("append content: read error", slog.String(semconv.ErrorKey, readErr.Error()))

			return readErr
		}
	}
}

// startStorageStream peeks the first 512 bytes of body, detects the mime type,
// opens the SafeUploadFile gRPC stream, and sends the Metadata frame with the
// sniffed mime. The peeked bytes remain in reader for the chunk loop to forward.
func (s *MediaService) startStorageStream(ctx context.Context, sess *uploadSession, reader *bufio.Reader) error {
	log := s.logger.With(slog.String("name", sess.name))

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}

	sniff, peekErr := reader.Peek(sniffSize)
	if peekErr != nil && !errors.Is(peekErr, io.EOF) {
		log.Debug("start storage stream: peek failed", slog.String(semconv.ErrorKey, peekErr.Error()))

		return peekErr
	}

	if len(sniff) == 0 {
		log.Debug("start storage stream: empty body")

		return ErrEmptyBody
	}

	mime := http.DetectContentType(sniff)

	log.Debug("start storage stream: sniffed mime type",
		slog.String("mime_type", mime), slog.Int("sniff_bytes", len(sniff)))

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

	if !sess.attachStream(stream, cancelFn, releaseFn, part.GetUploadId()) {
		log.Debug("start storage stream: session terminated before attach",
			slog.String("storage_upload_id", part.GetUploadId()))

		return ErrSessionDone
	}

	log.Debug("start storage stream: storage session opened",
		slog.String("storage_upload_id", part.GetUploadId()), slog.String("mime_type", mime))

	return nil
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
