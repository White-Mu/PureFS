package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/service"
)

type PublicShareHandler struct {
	shareSvc *service.ShareService
	fileSvc  *service.FileService
}

func NewPublicShareHandler(shareSvc *service.ShareService, fileSvc *service.FileService) *PublicShareHandler {
	return &PublicShareHandler{shareSvc: shareSvc, fileSvc: fileSvc}
}

func (h *PublicShareHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/public/shares/{token}", h.GetShare)
	r.Get("/api/public/shares/{token}/content", h.ServeContent)
}

func (h *PublicShareHandler) GetShare(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	share, f, err := h.shareSvc.ValidateShare(token)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":           share.ID,
		"token":        share.Token,
		"file_id":      share.FileID,
		"file_name":    f.Name,
		"file_size":    f.Size,
		"file_type":    f.FileType,
		"mime_type":    f.MimeType,
		"can_download": share.CanDownload,
		"expires_at":   share.ExpiresAt,
		"max_accesses": share.MaxAccesses,
		"access_count": share.AccessCount,
		"is_active":    share.IsActive,
		"created_at":   share.CreatedAt,
	})
}

func (h *PublicShareHandler) ServeContent(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	share, _, err := h.shareSvc.ValidateShare(token)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if !share.CanDownload {
		writeError(w, http.StatusForbidden, "download not allowed")
		return
	}

	reader, fileInfo, err := h.fileSvc.DownloadPublic(share.FileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	h.shareSvc.RecordAccess(token)

	// ?download=1 → force download; otherwise inline (browser preview)
	disposition := "inline"
	if r.URL.Query().Has("download") {
		disposition = "attachment"
	}

	// Disable content sniffing for security
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", disposition+"; filename=\""+fileInfo.Name+"\"")
	w.Header().Set("Content-Type", fileInfo.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
	w.Header().Set("Accept-Ranges", "bytes")

	if seeker, ok := reader.(io.ReadSeeker); ok {
		http.ServeContent(w, r, fileInfo.Name, fileInfo.UpdatedAt, seeker)
	} else {
		io.Copy(w, reader)
	}
}
