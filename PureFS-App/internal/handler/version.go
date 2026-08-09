package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/service"
)

// VersionHandler exposes REST endpoints for file version management.
type VersionHandler struct {
	versionSvc *service.VersionService
	fileSvc    *service.FileService
}

// NewVersionHandler creates a new VersionHandler.
func NewVersionHandler(versionSvc *service.VersionService, fileSvc *service.FileService) *VersionHandler {
	return &VersionHandler{
		versionSvc: versionSvc,
		fileSvc:    fileSvc,
	}
}

// RegisterRoutes registers version-related routes under the given router.
func (h *VersionHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/files/{id}/versions", h.ListVersions)
	r.Post("/api/files/{id}/versions/{vid}/restore", h.RestoreVersion)
	r.Delete("/api/files/{id}/versions/{vid}", h.DeleteVersion)
}

// ListVersions returns all saved versions for a file.
// GET /api/files/{id}/versions
func (h *VersionHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	// Verify ownership
	f, err := h.fileSvc.GetFile(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	_ = f

	versions, err := h.versionSvc.ListVersions(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if versions == nil {
		versions = []*model.FileVersion{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": versions,
	})
}

// RestoreVersion restores a file to a previous version.
// POST /api/files/{id}/versions/{vid}/restore
func (h *VersionHandler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	vid, err := strconv.ParseInt(chi.URLParam(r, "vid"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version id")
		return
	}

	// Verify ownership
	f, err := h.fileSvc.GetFile(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if err := h.versionSvc.RestoreVersion(id, vid, f, func(updated *model.File) error {
		return h.fileSvc.UpdateFile(updated)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// DeleteVersion removes a specific version of a file.
// DELETE /api/files/{id}/versions/{vid}
func (h *VersionHandler) DeleteVersion(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	vid, err := strconv.ParseInt(chi.URLParam(r, "vid"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version id")
		return
	}

	// Verify ownership
	_, err = h.fileSvc.GetFile(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if err := h.versionSvc.DeleteVersion(vid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
