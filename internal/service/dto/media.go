package dto

import "io"

// MediaDownloadRequest is the service-layer request to download a file.
type MediaDownloadRequest struct {
	FileID int64 // parsed from URL path
	Offset int64 // 0 for full download; range start for partial content
}

// FileMetadata contains file information returned in the first stream message.
type FileMetadata struct {
	ID       int64
	Name     string
	MimeType string
	Size     int64
}

// FileDownloadResult is the service-layer response for a download request.
// The caller is responsible for closing Body after use.
type FileDownloadResult struct {
	Metadata *FileMetadata
	Body     io.ReadCloser
}