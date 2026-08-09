package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
)

type PermissionHandler struct {
	permRepo *repository.PermissionRepo
	userRepo *repository.UserRepo
}

func NewPermissionHandler(permRepo *repository.PermissionRepo, userRepo *repository.UserRepo) *PermissionHandler {
	return &PermissionHandler{permRepo: permRepo, userRepo: userRepo}
}

func (h *PermissionHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/admin/permissions", h.List)
	r.Post("/api/admin/permissions", h.Create)
	r.Delete("/api/admin/permissions/{id}", h.Delete)
}

func (h *PermissionHandler) List(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserRole(r.Context()) != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	perms, err := h.permRepo.GetByUser(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, perms)
}

func (h *PermissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserRole(r.Context()) != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}

	var req struct {
		UserID   int64  `json:"user_id"`
		FilePath string `json:"file_path"`
		Perm     string `json:"perm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Validate the target user exists
	if _, err := h.userRepo.GetByID(req.UserID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	p := &model.Permission{
		UserID:   req.UserID,
		FilePath: req.FilePath,
		Perm:     req.Perm,
	}
	if err := h.permRepo.Create(p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, p)
}

func (h *PermissionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserRole(r.Context()) != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.permRepo.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
