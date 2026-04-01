package dto

import (
	"io"
	"net/url"
)

// MediaDownloadRequest is the service-layer request to download a file.
type MediaDownloadRequest struct {
	FileID int64 `json:"fileId"`
	Offset int64 `json:"offset"`
}

// FileMetadata contains file information returned in the first stream message.
type FileMetadata struct {
	ID       string   `json:"fileId,omitempty"`
	Name     string   `json:"name,omitempty"`
	MimeType string   `json:"mimeType,omitempty"`
	Size     int64    `json:"size,omitempty"`
	Hash     string   `json:"hash,omitempty"`
	Url      *url.URL `json:"url,omitempty"`
}

// FileInfoResponse contains the file loading info status
type FileInfoResponse struct {
	UploadID string `json:"uploadId,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Name     string `json:"name,omitempty"`
	Hash     []byte `json:"hash,omitempty"`
}

// FileDownloadResult is the service-layer response for a download request.
// The caller is responsible for closing Body after use.
type FileDownloadResult struct {
	Metadata *FileMetadata
	Body     io.ReadCloser
}

// CreateUploadSessionRequest is the service-layer request to create an upload session.
type CreateUploadSessionRequest struct {
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// CreateUploadSessionResponse is returned after a session is successfully created.
type CreateUploadSessionResponse struct {
	UploadID string `json:"uploadId,omitempty"`
}

type SuccessfullyUploadResponse struct {
	FileID   string `json:"fileId,omitempty"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Hash     string `json:"hash,omitempty"`
}
