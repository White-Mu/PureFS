package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/search"
	"github.com/purefs/purefs/internal/service"
)

// SearchHandler exposes the full-text search API endpoint.
type SearchHandler struct {
	svc *service.SearchService
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// RegisterRoutes registers search routes on the given router.
func (h *SearchHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/search", h.Search)
}

// Search handles GET /api/search?q=xxx&type=all|file|document|image&offset=0&limit=50
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	opts := search.SearchOptions{
		Type:  r.URL.Query().Get("type"),
		Limit: 50,
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			opts.Offset = v
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			opts.Limit = v
		}
	}

	results, total, err := h.svc.Search(r.Context(), query, userID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if results == nil {
		results = []*model.File{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": results,
		"total": total,
		"query": query,
	})
}
