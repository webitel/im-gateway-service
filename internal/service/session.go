package service

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

const uploadIdleTimeout = 3 * time.Minute

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

	// storageUploadID is the upload ID assigned by the storage service, captured
	// from the first SafeUploadFile Part response. Used to query storage for the
	// authoritative uploaded size after the local stream goes inactive.
	storageUploadID string

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
func (s *uploadSession) attachStream(stream storagev1.FileService_SafeUploadFileClient, cancelFn context.CancelFunc, releaseFn func(), storageUploadID string) bool {
	s.mu.Lock()
	if s.inactive || s.stream != nil {
		s.mu.Unlock()

		cancelFn()
		releaseFn()

		return false
	}

	s.stream = stream
	s.cancelFn = cancelFn
	s.releaseFn = releaseFn
	s.storageUploadID = storageUploadID
	s.mu.Unlock()

	go s.heartbeat()

	return true
}

func (s *uploadSession) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return !s.inactive
}

// offset returns the number of bytes forwarded to storage so far.
func (s *uploadSession) offset() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.lastOffset
}

// addOffset advances the forwarded-bytes counter by n.
func (s *uploadSession) addOffset(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastOffset += int64(n)
}

// storageID returns the upload ID assigned by the storage service, or "" if no
// stream has been attached yet.
func (s *uploadSession) storageID() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.storageUploadID
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
			s.terminate()

			return
		}
	}
}

// terminate cancels the underlying gRPC stream (if attached) and marks the
// session inactive. Safe to call multiple times and from multiple goroutines,
// and safe to call before a stream is attached.
func (s *uploadSession) terminate() {
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
