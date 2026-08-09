package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/service"
)

type ShareHandler struct {
	svc *service.ShareService
}

func NewShareHandler(svc *service.ShareService) *ShareHandler {
	return &ShareHandler{svc: svc}
}

func (h *ShareHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/shares", h.Create)
	r.Get("/api/shares", h.List)
	r.Get("/api/shares/{token}", h.GetShare)
	r.Delete("/api/shares/{id}", h.Deactivate)
}

func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	share, err := h.svc.CreateShare(userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, share)
}

func (h *ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	shares, err := h.svc.ListShares(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, shares)
}

func (h *ShareHandler) GetShare(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	share, file, err := h.svc.GetShare(token)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Verify password if set
	password := r.URL.Query().Get("password")
	if share.Password != "" && share.Password != password {
		writeError(w, http.StatusForbidden, "password required")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"share": share,
		"file":  file,
	})
}

func (h *ShareHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid share id")
		return
	}

	if err := h.svc.DeactivateShare(userID, id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
