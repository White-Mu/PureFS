package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/monitoring"
)

// AdminHandler handles admin-only monitoring and management endpoints.
type AdminHandler struct {
	db      *sql.DB
	cfg     *config.Config
	startTime time.Time
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(db *sql.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		db:        db,
		cfg:       cfg,
		startTime: time.Now(),
	}
}

// RegisterRoutes registers all admin routes on the given router.
// The caller is responsible for ensuring only admin users can access these routes.
func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/admin/stats", h.GetStats)
	r.Get("/api/admin/stats/stream", h.StatsStream)
	r.Post("/api/admin/backup", h.Backup)
}

// StatsResponse holds the combined stats returned by the stats endpoint.
type StatsResponse struct {
	CPU        float64               `json:"cpu_percent"`
	Memory     monitoring.MemStats   `json:"memory"`
	Disk       monitoring.DiskStats  `json:"disk"`
	UptimeSecs int64                 `json:"uptime_seconds"`
	NumCPU     int                   `json:"num_cpu"`
	GoVersion  string                `json:"go_version"`
	DB         DBStats               `json:"db"`
}

// DBStats holds database-level statistics.
type DBStats struct {
	UserCount int64 `json:"user_count"`
	FileCount int64 `json:"file_count"`
	ShareCount int64 `json:"share_count"`
}

// GetStats returns a snapshot of current system and database statistics.
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	storagePath := h.cfg.Storage.Local.Path
	if storagePath == "" {
		storagePath = "data/storage"
	}

	stats := monitoring.GetSystemStats(storagePath)

	dbStats, err := h.collectDBStats()
	if err != nil {
		dbStats = DBStats{}
	}

	resp := StatsResponse{
		CPU:        stats.CPU,
		Memory:     stats.Memory,
		Disk:       stats.Disk,
		UptimeSecs: stats.UptimeSecs,
		NumCPU:     stats.NumCPU,
		GoVersion:  stats.GoVersion,
		DB:         dbStats,
	}

	writeJSON(w, http.StatusOK, resp)
}

// StatsStream sends SSE events with system stats every 2 seconds.
// The client can stop the stream by closing the connection.
func (h *AdminHandler) StatsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	storagePath := h.cfg.Storage.Local.Path
	if storagePath == "" {
		storagePath = "data/storage"
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := monitoring.GetSystemStats(storagePath)
			dbStats, err := h.collectDBStats()
			if err != nil {
				dbStats = DBStats{}
			}

			resp := StatsResponse{
				CPU:        stats.CPU,
				Memory:     stats.Memory,
				Disk:       stats.Disk,
				UptimeSecs: stats.UptimeSecs,
				NumCPU:     stats.NumCPU,
				GoVersion:  stats.GoVersion,
				DB:         dbStats,
			}

			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// Backup performs a database backup using VACUUM INTO and copies config.
// It returns the backup file as a downloadable archive.
func (h *AdminHandler) Backup(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join("data", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup directory: "+err.Error())
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	dbBackupPath := filepath.Join(backupDir, fmt.Sprintf("purefs-%s.db", timestamp))

	// VACUUM INTO creates a clean copy of the database
	_, err := h.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", dbBackupPath))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database backup failed: "+err.Error())
		return
	}

	// Copy config file if it exists
	configPath := "config.yaml"
	if env := os.Getenv("PUREFS_CONFIG"); env != "" {
		configPath = env
	}
	if configData, err := os.ReadFile(configPath); err == nil {
		configBackupPath := filepath.Join(backupDir, fmt.Sprintf("config-%s.yaml", timestamp))
		if err := os.WriteFile(configBackupPath, configData, 0644); err != nil {
			// Config backup is non-fatal
			fmt.Printf("Warning: config backup failed: %v\n", err)
		}
	}

	resp := map[string]string{
		"status":    "ok",
		"message":   "backup completed successfully",
		"db_backup": dbBackupPath,
		"timestamp": timestamp,
	}

	// Check if client wants file download
	if r.URL.Query().Get("download") == "true" {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"purefs-%s.db\"", timestamp))
		data, err := os.ReadFile(dbBackupPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read backup file: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// collectDBStats gathers counts from the database.
func (h *AdminHandler) collectDBStats() (DBStats, error) {
	var stats DBStats

	if err := h.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.UserCount); err != nil {
		return stats, fmt.Errorf("count users: %w", err)
	}

	if err := h.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&stats.FileCount); err != nil {
		return stats, fmt.Errorf("count files: %w", err)
	}

	if err := h.db.QueryRow("SELECT COUNT(*) FROM shares").Scan(&stats.ShareCount); err != nil {
		// Shares table might not exist in older schemas — non-fatal
		stats.ShareCount = 0
	}

	return stats, nil
}

// requireAdmin is a middleware helper that checks for admin role.
// Use this as a convenience for standalone admin route groups.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.GetUserRole(r.Context()) != "admin" {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		next.ServeHTTP(w, r)
	})
}
