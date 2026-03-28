package service

import (
	"context"
	"log/slog"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	storageclient "github.com/webitel/im-gateway-service/infra/client/storage"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var _ MediaDownloader = (*MediaService)(nil)

// MediaDownloader is the interface consumed by the HTTP handler layer.
type MediaDownloader interface {
	Download(ctx context.Context, req *dto.MediaDownloadRequest) (*dto.FileDownloadResult, error)
}

type MediaService struct {
	logger        *slog.Logger
	storageClient *storageclient.Client
}

func NewMediaService(logger *slog.Logger, storageClient *storageclient.Client) *MediaService {
	return &MediaService{
		logger:        logger,
		storageClient: storageClient,
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
			ID:       meta.GetId(),
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
