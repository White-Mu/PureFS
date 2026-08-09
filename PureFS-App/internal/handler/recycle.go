package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/service"
)

type RecycleHandler struct {
	svc *service.RecycleService
}

func NewRecycleHandler(svc *service.RecycleService) *RecycleHandler {
	return &RecycleHandler{svc: svc}
}

func (h *RecycleHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/trash", h.List)
	r.Post("/api/trash/{id}/restore", h.Restore)
	r.Delete("/api/trash/{id}", h.PermanentlyDelete)
	r.Delete("/api/trash", h.EmptyTrash)
}

func (h *RecycleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	items, total, err := h.svc.List(userID, offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
	})
}

func (h *RecycleHandler) Restore(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trash id")
		return
	}

	f, err := h.svc.Restore(userID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, f)
}

func (h *RecycleHandler) PermanentlyDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trash id")
		return
	}

	if err := h.svc.PermanentlyDelete(userID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *RecycleHandler) EmptyTrash(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := h.svc.EmptyTrash(userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
