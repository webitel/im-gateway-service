package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// downloadFile returns the full file.
func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		renderError(w, http.StatusBadRequest, "api.bad_args", "invalid file id")
		return
	}

	result, err := h.media.Download(r.Context(), &dto.MediaDownloadRequest{
		FileID: fileID,
		Offset: 0,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	defer result.Body.Close()

	w.Header().Set("Content-Type", result.Metadata.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, result.Metadata.Name))
	if result.Metadata.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(result.Metadata.Size, 10))
	}

	if _, err := io.Copy(w, result.Body); err != nil {
		h.writeError(w, err)
	}
}

// streamFile supports Range-based partial downloads.
func (h *Handler) streamFile(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		renderError(w, http.StatusBadRequest, "api.bad_args", "invalid file id")
		return
	}

	var (
		offset         int64
		isRangeRequest bool
	)

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		var parseErr error
		offset, parseErr = parseRangeStart(rangeHeader)
		if parseErr != nil {
			renderError(w, http.StatusRequestedRangeNotSatisfiable, "api.bad_args", "invalid Range header")
			return
		}
		isRangeRequest = true
	}

	result, err := h.media.Download(r.Context(), &dto.MediaDownloadRequest{
		FileID: fileID,
		Offset: offset,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	defer result.Body.Close()

	w.Header().Set("Content-Type", result.Metadata.MimeType)
	w.Header().Set("Accept-Ranges", "bytes")

	if isRangeRequest {
		if result.Metadata.Size > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, result.Metadata.Size-1, result.Metadata.Size))
			w.Header().Set("Content-Length", strconv.FormatInt(result.Metadata.Size-offset, 10))
		}
		w.WriteHeader(http.StatusPartialContent)
	} else {
		if result.Metadata.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(result.Metadata.Size, 10))
		}
	}

	if _, err := io.Copy(w, result.Body); err != nil {
		h.writeError(w, err)
		return
	}
}

// createUploadSession establish session to upload file
func (h *Handler) createUploadSession(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUploadSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		renderError(w, http.StatusBadRequest, "api.bad_args", "invalid request body")
		return
	}

	uploadID, err := h.media.CreateUploadSession(r.Context(), req.Name, req.MimeType)
	if err != nil {
		h.logger.Error("failed to create upload session", slog.String("error", err.Error()))
		h.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto.CreateUploadSessionResponse{UploadID: uploadID}); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// getUploadFileInfo returns the uploaded size during the active upload session
func (h *Handler) getUploadFileInfo(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		renderError(w, http.StatusBadRequest, "api.bad_args", "missing uploadId")
		return
	}

	size, err := h.media.GetUploadFileInfo(r.Context(), uploadID)
	if err != nil {
		h.logger.Error("failed to get upload file info", slog.String("error", err.Error()))
		h.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto.FileInfoResponse{UploadID: uploadID, Size: size}); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// uploadFile forwards file to storage
func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		renderError(w, http.StatusBadRequest, "api.bad_args", "missing uploadId")
		return
	}

	meta, err := h.media.AppendContent(r.Context(), uploadID, r.Body)
	if err != nil {
		h.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto.SuccessfullyUploadResponse{
		FileID:   meta.ID,
		Name:     meta.Name,
		MimeType: meta.MimeType,
		Size:     meta.Size,
		Hash:     meta.Hash,
	}); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// parseRangeStart extracts the start byte offset from a Range header value.
// Accepts "bytes=START-" and "bytes=START-END" formats.
func parseRangeStart(rangeHeader string) (int64, error) {
	const prefix = "bytes="
	if !strings.HasPrefix(rangeHeader, prefix) {
		return 0, fmt.Errorf("unsupported range unit in %q", rangeHeader)
	}
	spec := strings.TrimPrefix(rangeHeader, prefix)
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("missing range start in %q", rangeHeader)
	}
	return strconv.ParseInt(parts[0], 10, 64)
}
