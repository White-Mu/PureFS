package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/service"
)

type FileHandler struct {
	svc *service.FileService
}

func NewFileHandler(svc *service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

func (h *FileHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/files", h.List)
	r.Post("/api/files/dir", h.CreateDir)
	r.Post("/api/files/upload", h.Upload)
	r.Get("/api/files/{id}", h.Get)
	r.Get("/api/files/{id}/download", h.Download)
	r.Get("/api/files/{id}/preview", h.Preview)
	r.Patch("/api/files/{id}/rename", h.Rename)
	r.Patch("/api/files/{id}/move", h.Move)
	r.Delete("/api/files/{id}", h.Delete)
	r.Post("/api/files/{id}/copy", h.Copy)
	r.Patch("/api/files/{id}/pin", h.SetPinned)
	r.Patch("/api/files/{id}/favorite", h.SetFavorite)
	r.Post("/api/files/batch/delete", h.BatchDelete)
}

func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	q := model.FileListQuery{
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
		Search:    r.URL.Query().Get("search"),
		View:      r.URL.Query().Get("view"),
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			q.Offset = v
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			q.Limit = v
		}
	}
	if parentStr := r.URL.Query().Get("parent_id"); parentStr != "" {
		if v, err := strconv.ParseInt(parentStr, 10, 64); err == nil {
			q.ParentID = &v
		}
	}
	if fileType := r.URL.Query().Get("file_type"); fileType != "" {
		t := model.FileType(fileType)
		q.FileType = &t
	}
	if fav := r.URL.Query().Get("is_favorite"); fav == "true" {
		v := true; q.IsFavorite = &v
	}
	if pin := r.URL.Query().Get("is_pinned"); pin == "true" {
		v := true; q.IsPinned = &v
	}

	files, total, err := h.svc.List(userID, q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": files,
		"total": total,
	})
}

func (h *FileHandler) CreateDir(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	f, err := h.svc.CreateDir(userID, req.ParentID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, f)
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file in form")
		return
	}
	defer file.Close()

	var parentID *int64
	if pidStr := r.FormValue("parent_id"); pidStr != "" {
		if v, err := strconv.ParseInt(pidStr, 10, 64); err == nil {
			parentID = &v
		}
	}

	name := header.Filename
	if nameOverride := r.FormValue("name"); nameOverride != "" {
		name = nameOverride
	}

	f, err := h.svc.Upload(userID, parentID, name, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, f)
}

func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	f, err := h.svc.GetFile(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	writeJSON(w, http.StatusOK, f)
}

func (h *FileHandler) Preview(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	reader, f, err := h.svc.Download(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", f.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	w.Header().Set("Cache-Control", "private, max-age=3600")

	if seeker, ok := reader.(io.ReadSeeker); ok {
		http.ServeContent(w, r, f.Name, f.UpdatedAt, seeker)
	} else {
		io.Copy(w, reader)
	}
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	reader, f, err := h.svc.Download(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	if seeker, ok := reader.(io.ReadSeeker); ok {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+f.Name+"\"")
		w.Header().Set("Content-Type", f.MimeType)
		w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
		http.ServeContent(w, r, f.Name, f.UpdatedAt, seeker)
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+f.Name+"\"")
		w.Header().Set("Content-Type", f.MimeType)
		w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
		io.Copy(w, reader)
	}
}

func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var req model.RenameFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	f, err := h.svc.Rename(userID, id, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, f)
}

func (h *FileHandler) Move(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var req model.MoveFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	f, err := h.svc.Move(userID, id, req.TargetParentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, f)
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	if err := h.svc.Delete(userID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandler) SetPinned(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var body struct {
		Pinned bool `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.svc.SetPinned(userID, id, body.Pinned); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *FileHandler) SetFavorite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var body struct {
		Favorite bool `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.svc.SetFavorite(userID, id, body.Favorite); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *FileHandler) Copy(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var body struct {
		TargetParentID *int64 `json:"target_parent_id"`
		NewName        string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Allow empty body (copy to same directory with same name)
		body = struct {
			TargetParentID *int64 `json:"target_parent_id"`
			NewName        string `json:"new_name"`
		}{}
	}

	f, err := h.svc.Copy(userID, id, body.TargetParentID, body.NewName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, f)
}

func (h *FileHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "no file ids provided")
		return
	}

	var failed []int64
	for _, id := range body.IDs {
		if err := h.svc.Delete(userID, id); err != nil {
			failed = append(failed, id)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": len(body.IDs) - len(failed),
		"failed":  failed,
	})
}
