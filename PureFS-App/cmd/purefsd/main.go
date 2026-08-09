package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pressly/goose/v3"
	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/database"
	"github.com/purefs/purefs/internal/handler"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/search"
	"github.com/purefs/purefs/internal/service"
	"github.com/purefs/purefs/internal/storage"
	"github.com/purefs/purefs/pkg/sftp"
	"github.com/purefs/purefs/webdav"
)

func main() {
	// Load config
	cfgPath := "config.yaml"
	if env := os.Getenv("PUREFS_CONFIG"); env != "" {
		cfgPath = env
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Save default config if not exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := cfg.Save(cfgPath); err != nil {
			log.Printf("Warning: failed to save default config: %v", err)
		} else {
			log.Printf("Default config saved to %s", cfgPath)
		}
	}

	// Open database
	db, err := database.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := goose.SetDialect(cfg.Database.Driver); err != nil {
		log.Fatalf("Failed to set dialect: %v", err)
	}

	// Use embedded migrations
	goose.SetBaseFS(database.MigrationsFS)

	if err := goose.Up(db, "migrations"); err != nil {
		// Fallback: try loading from filesystem
		migrationsDir := filepath.Join("internal", "database", "migrations")
		if _, err := os.Stat(migrationsDir); err == nil {
			goose.SetBaseFS(nil)
			if err := goose.Up(db, migrationsDir); err != nil {
				log.Printf("Warning: migration error (non-fatal): %v", err)
			}
		} else {
			log.Printf("Warning: embedded migration error (non-fatal): %v", err)
		}
	}

	// Initialize storage
	store, keyManager, err := storage.NewFromConfigWithEncryption(cfg.Storage.Driver, cfg.Storage.Local.Path, storage.S3Config{
		Endpoint:  cfg.Storage.S3.Endpoint,
		Region:    cfg.Storage.S3.Region,
		Bucket:    cfg.Storage.S3.Bucket,
		AccessKey: cfg.Storage.S3.AccessKey,
		SecretKey: cfg.Storage.S3.SecretKey,
		Secure:    cfg.Storage.S3.Secure,
	}, cfg.Encryption)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepo(db)
	fileRepo := repository.NewFileRepo(db)
	shareRepo := repository.NewShareRepo(db)
	auditRepo := repository.NewAuditLogRepo(db)
	permRepo := repository.NewPermissionRepo(db)
	recycleRepo := repository.NewRecycleBinRepo(db)
	versionRepo := repository.NewFileVersionRepo(db)

	// Initialize services
	userSvc := service.NewUserService(userRepo, cfg)
	recycleSvc := service.NewRecycleService(fileRepo, recycleRepo, userRepo, store, cfg)
	fileSvc := service.NewFileService(fileRepo, permRepo, userRepo, recycleSvc, store, cfg)
	versionSvc := service.NewVersionService(versionRepo, store, cfg)
	fileSvc.SetVersionService(versionSvc)
	shareSvc := service.NewShareService(shareRepo, fileRepo)
	auditSvc := service.NewAuditService(auditRepo)
	integritySvc := service.NewIntegrityService(store, cfg)

	// Initialize full-text search
	var searchSvc *service.SearchService
	if cfg.Search.Enabled {
		ftsEngine, err := search.NewFTS5Engine(db)
		if err != nil {
			log.Printf("Warning: failed to initialize search engine: %v", err)
		} else {
			searchSvc = service.NewSearchService(ftsEngine, fileRepo, store)
			fileSvc.SetSearchService(searchSvc)
			searchSvc.StartQueue()
			defer searchSvc.StopQueue()
			log.Println("Full-text search enabled (SQLite FTS5)")
		}
	}

	// Initialize admin monitoring handler
	adminHandler := handler.NewAdminHandler(db, cfg)

	// Initialize router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS)
	r.Use(middleware.ContentType)

	// Health check
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"0.1.0"}`))
	})

	// Public routes (no auth required)
	publicShareHandler := handler.NewPublicShareHandler(shareSvc, fileSvc)
	publicShareHandler.RegisterRoutes(r)

	// Public auth routes with rate limiting
	authHandler := handler.NewUserHandler(userSvc)

	// Rate limit: login = 5 requests per minute, register = 3 requests per hour
	r.With(middleware.RateLimit(5, 1*time.Minute)).Post("/api/auth/login", authHandler.Login)
	r.With(middleware.RateLimit(3, 1*time.Hour)).Post("/api/auth/register", authHandler.Register)

	// Password reset (public, no auth)
	r.With(middleware.RateLimit(3, 10*time.Minute)).Post("/api/auth/forgot-password", authHandler.ForgotPassword)
	r.With(middleware.RateLimit(5, 10*time.Minute)).Post("/api/auth/reset-password", authHandler.ResetPassword)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.Auth.JWTSecret))

		// User profile and TOTP routes (authenticated)
		r.Get("/api/users/me", authHandler.GetMe)
		r.Get("/api/users", authHandler.ListUsers)
		r.Post("/api/auth/totp/setup", authHandler.SetupTOTP)
		r.Post("/api/auth/totp/enable", authHandler.EnableTOTP)
			r.Post("/api/auth/totp/disable", authHandler.DisableTOTP)
		r.Post("/api/auth/refresh", authHandler.Refresh)

		fileHandler := handler.NewFileHandler(fileSvc)
		fileHandler.RegisterRoutes(r)

		shareHandler := handler.NewShareHandler(shareSvc)
		shareHandler.RegisterRoutes(r)

		recycleHandler := handler.NewRecycleHandler(recycleSvc)
		recycleHandler.RegisterRoutes(r)

		versionHandler := handler.NewVersionHandler(versionSvc, fileSvc)
		versionHandler.RegisterRoutes(r)

		// Search endpoint
		if searchSvc != nil {
			searchHandler := handler.NewSearchHandler(searchSvc)
			searchHandler.RegisterRoutes(r)
		}

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if middleware.GetUserRole(req.Context()) != "admin" {
						http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, req)
				})
			})
			auditHandler := handler.NewAuditHandler(auditSvc)
			auditHandler.RegisterRoutes(r)
			permHandler := handler.NewPermissionHandler(permRepo, userRepo)
			permHandler.RegisterRoutes(r)
			adminHandler.RegisterRoutes(r)
			// Admin user management
			r.Post("/api/admin/users", authHandler.AdminCreateUser)
			r.Patch("/api/admin/users/{id}/toggle", authHandler.AdminToggleUser)
		})
	})

	// WebDAV endpoint (authenticated)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.Auth.JWTSecret))
		r.HandleFunc("/webdav/*", webdav.Handler(fileRepo, store, cfg))
	})

	// Serve static frontend
	fsRoot := os.Getenv("PUREFS_WEB_ROOT")
	if fsRoot == "" {
		fsRoot = "../PureFS-Web/dist"
	}
	if stat, err := os.Stat(fsRoot); err == nil && stat.IsDir() {
		fileServer := http.FileServer(http.Dir(fsRoot))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback - serve index.html for unknown routes
			path := filepath.Join(fsRoot, r.URL.Path)
			if _, err := os.Stat(path); err != nil {
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	} else {
		// No frontend built, show API info
		fmt.Println("No frontend dist found at", fsRoot, "- running API only")
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("PureFS starting on %s\n", addr)
	fmt.Printf("API: http://localhost:%d/api\n", cfg.Server.Port)
	fmt.Printf("WebDAV: http://localhost:%d/webdav\n", cfg.Server.Port)

	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Start periodic integrity check if enabled
	if cfg.Integrity.Enabled {
		go integritySvc.StartPeriodicCheck("/", cfg.Integrity.Interval, nil)
	}

	// Start key rotation if encryption is enabled
	if cfg.Encryption.Enabled && keyManager != nil && cfg.Encryption.RotationIntervalHours > 0 {
		go func() {
			interval := time.Duration(cfg.Encryption.RotationIntervalHours) * time.Hour
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				newVersion, err := keyManager.Rotate()
				if err != nil {
					log.Printf("Key rotation error: %v", err)
				} else {
					log.Printf("Key rotated to version %d", newVersion)
				}
			}
		}()
	}

	// Start periodic trash cleanup (every 6 hours)
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			count, err := recycleSvc.CleanupExpired()
			if err != nil {
				log.Printf("Trash cleanup error: %v", err)
			} else if count > 0 {
				log.Printf("Trash cleanup: permanently deleted %d expired items", count)
			}
		}
	}()

	// Start SFTP server if enabled
	if cfg.SFTP.Enabled {
		sftpServer, err := sftp.New(cfg, store, userRepo, fileRepo)
		if err != nil {
			log.Printf("SFTP server initialization error: %v", err)
		} else {
			go func() {
				if err := sftpServer.Start(); err != nil {
					log.Printf("SFTP server error: %v", err)
				}
			}()
			fsPort := cfg.SFTP.Port
			if fsPort == 0 {
				fsPort = 2022
			}
			log.Printf("SFTP server started on port %d", fsPort)
		}
	}

	<-quit
	fmt.Println("Shutting down...")
}
