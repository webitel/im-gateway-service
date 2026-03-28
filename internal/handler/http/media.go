package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// downloadFile handles GET /im/media/{id} and returns the full file.
func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	result, err := h.downloader.Download(r.Context(), &dto.MediaDownloadRequest{
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

// streamFile handles GET /im/media/{id}/stream and supports Range-based partial downloads.
func (h *Handler) streamFile(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
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
			http.Error(w, "invalid Range header", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		isRangeRequest = true
	}

	result, err := h.downloader.Download(r.Context(), &dto.MediaDownloadRequest{
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

	io.Copy(w, result.Body) //nolint:errcheck
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
