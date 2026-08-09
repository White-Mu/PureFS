# PureFS Development Phases

> Written for the next developer taking over this project. Covers what was built across four phases, current status, and how to get started.

---

## Project Overview

**PureFS** — A fully open-source, self-hostable private cloud drive. Go backend + React/TypeScript frontend, with local/S3 storage backends, WebDAV protocol support, multi-user permissions, share links, 2FA, audit logging, and more.

**Tech Stack:**
- Backend: Go + Chi Router + SQLite (modernc, pure Go) + goose migrations
- Frontend: React 18 + TypeScript + Vite + Zustand + Axios
- Storage: Local filesystem / S3-compatible object storage (dual driver)
- Deployment: Docker multi-stage builds, targeting x86_64 & ARM64

---

## Phase 1: Foundation & Core Backend

**What was done: the project skeleton, from zero to a working structure.**

### 1.1 Requirements & Architecture
- `docs/REQUIREMENTS.md` — Full requirements specification
- `docs/ARCHITECTURE.md` — Layered architecture design (API layer → Business layer → Data access layer → Storage layer)
- `docs/DEV.md` — Development conventions (code style, commit conventions, branching strategy, test requirements)
- Other docs under `docs/`: API.md, AUTH.md, DATABASE.md, DEPLOY.md, PLUGIN.md, SEARCH.md, STORAGE.md

### 1.2 Configuration System (`internal/config/config.go`)
- YAML-driven config, auto-generates `config.yaml` on first run
- Covers: Server, Database, Storage (local/s3), Auth (JWT/encryption), Log, Integrity
- `Default()` returns sensible defaults; `Load()` falls back to defaults when no config file exists

### 1.3 Database Layer (`internal/database/`)
- SQLite via `modernc.org/sqlite` (pure Go, no CGO required)
- Embedded migration files (`embed.FS`), auto-run via goose on startup
- `migrations/001_init.sql` creates 6 core tables: users, files, shares, permissions, audit_logs, integrity_records
- WAL mode + busy_timeout + foreign_keys + cache_size pragmas configured

### 1.4 Data Models (`internal/model/`)
- `File` — files/directories with SHA256, pin, favorite, encryption flags
- `User` — role (admin/user), TOTP, storage quota
- `Share` — share links with token, password, expiry, access count limit
- `Permission` — path-based read/write/admin permissions
- `AuditLog` — action log (action, detail, IP)
- `IntegrityRecord` — file integrity check records
- `dto.go` — request/response DTOs (FileListQuery, CreateFileRequest, etc.)

### 1.5 Storage Abstraction Layer (`internal/storage/`)
- `Storage` interface defines 10 generic file operations (Open/Create/Delete/Stat/List/Mkdir/Rename/Copy/Exists/RealPath)
- `LocalStorage` — full local disk implementation with SHA256 computation
- `S3Storage` — S3-compatible implementation (MinIO/R2/OSS), with intentional panic on streaming Create (use the upload pipeline instead)
- `NewFromConfig()` factory switches driver based on config

### 1.6 Auth Infrastructure
- `internal/auth/password.go` — bcrypt password hashing & verification
- `internal/auth/totp.go` — TOTP 2FA (key generation, code verification, QR URI)
- `pkg/jwtutil/jwt.go` — JWT signing & parsing (HS256)
- `internal/middleware/auth.go` — Auth middleware, extracts JWT from Authorization header or query token, injects UserID/Username/Role into context
- `internal/middleware/cors.go` — CORS middleware

### 1.7 Repository Layer (`internal/repository/`)
- Five repositories: `UserRepo`, `FileRepo`, `ShareRepo`, `AuditLogRepo`, `PermissionRepo`
- All SQL queries encapsulated: CRUD, list (with sort/filter/pagination), move/rename, pin/favorite toggles

---

## Phase 2: API & Business Logic

**What was done: all API endpoints functional, business logic complete.**

### 2.1 HTTP Handlers (`internal/handler/`)
- `router.go` — base router + `writeJSON`/`writeError` helpers
- `file.go` — file operations: list, create directory, upload, download, preview, rename, move, delete, pin, favorite
- `user.go` — user API: register, login, profile, TOTP setup/enable/disable
- `share.go` — share management: create share, list shares, deactivate share
- `public_share.go` — public share access: view/download shared files via token + password
- `audit.go` — audit log API (admin only)
- `permission.go` — permission management: grant/list/delete folder permissions per user

### 2.2 Business Logic Layer (`internal/service/`)
- `file_service.go` — core file logic: upload (with SHA256 hashing, temp file + atomic rename), download, rename, move (source + target permission checks), delete, permission resolution (owner first → explicit permissions → deny)
- `user_service.go` — user registration/login/JWT issuance/TOTP
- `share_service.go` — share creation (random token generation), validation (password/expiry/access count checks), deactivation
- `audit_service.go` — audit log recording & querying
- `integrity_service.go` — periodic file integrity verification service

### 2.3 Entry Point (`cmd/purefsd/main.go`)
- Full startup flow: load config → open database → run migrations → init storage → init repos/services → register routes → start HTTP server
- Graceful shutdown (SIGINT/SIGTERM)
- Static frontend file serving with SPA fallback
- Route structure:
  ```
  /api/health          — health check (public)
  /api/auth/*          — authentication (public)
  /api/shares/:token   — public share access (public)
  /api/files/*         — file operations (authenticated)
  /api/shares/*        — share management (authenticated)
  /api/users/*         — user management (authenticated)
  /api/admin/*         — admin endpoints (admin role required)
  /webdav/*            — WebDAV (authenticated)
  /*                   — SPA frontend file serving
  ```

---

## Phase 3: WebDAV Protocol & Frontend

**What was done: standard protocol mounting + full web UI.**

### 3.1 WebDAV Support (`webdav/webdav.go`)
- Built on `golang.org/x/net/webdav` standard library
- `webdavFileSystem` adapts the storage `Storage` interface
- Supported operations: Mkdir, OpenFile (read/write), RemoveAll, Rename, Stat
- User-space isolation: WebDAV paths auto-map under `/users/{userID}/`
- JWT authentication (shared Auth middleware with the API layer)
- PCs/phones/NAS devices can mount directly via `http://host:8080/webdav`

### 3.2 Frontend Foundation
- Vite + React 18 + TypeScript strict mode
- Routing: react-router-dom v7 (`/login`, `/`, `/favorites`, `/pinned`, `/recent`, `/shares`, `/settings`, `/admin`, `/share/:token`)
- State management: Zustand (`useAuthStore` + `useUIStore`)
- API client: Axios + interceptors (auto-attach Bearer token, auto-redirect to login on 401)
- Styling: plain CSS (globals.css for design tokens + components.css for component styles), dark mode support

### 3.3 Pages (6 total)
| Page | Functionality |
|------|---------------|
| `LoginPage.tsx` | Login form, registration form, TOTP code input |
| `FilesPage.tsx` | Core file browser: list/grid/timeline views, search, sort, new folder, drag-and-drop upload, multi-select batch delete, context menu, inline rename, file preview |
| `SharePage.tsx` | Public share page (token access, optional password, online preview/download) |
| `SharesPage.tsx` | My shares list (view, deactivate, copy link) |
| `SettingsPage.tsx` | User settings (profile, TOTP setup, change password) |
| `AdminPage.tsx` | Admin dashboard (system overview, user management, audit logs, permission management) |

### 3.4 Components (9 total)
| Component | Functionality |
|-----------|---------------|
| `Sidebar.tsx` | Left nav (All Files, Favorites, Pinned, Recent, Shares, Settings, Admin) + storage usage + dark mode toggle + logout |
| `FileRow.tsx` | File list row (icon, name, size, date, pin/favorite badges, select checkbox) |
| `FileGrid.tsx` | Large icon grid view (thumbnail, name, selection) |
| `Breadcrumb.tsx` | Breadcrumb navigation (clickable segments) |
| `ContextMenu.tsx` | Right-click menu (preview, download, share, rename, refresh, delete, pin/unpin, favorite/unfavorite) |
| `SelectionToolbar.tsx` | Multi-select batch action bar (selected count, delete button, clear selection) |
| `ShareDialog.tsx` | Share creation dialog (password, expiry, access limit, download permission) |
| `UploadOverlay.tsx` | Upload progress overlay (filename, progress bar, completion status) |
| `FilePreview.tsx` | File preview (images, video, text, PDF, code highlighting) |

---

## Phase 4: Docker Deployment & Build Artifacts

**What was done: one-command deploy to any environment.**

### 4.1 Multi-stage Docker Build (`Dockerfile`)
- **Stage 1**: golang:1.26-alpine compiles backend (CGO_ENABLED=0 static binary)
- **Stage 2**: node:22-alpine compiles frontend (npm ci + npm run build)
- **Stage 3**: alpine:3.21 runtime image (minimal: ca-certificates + tzdata + curl)
- Non-root user (purefs)
- HEALTHCHECK configured (30s interval)
- `PUREFS_WEB_ROOT=web/dist` env var for frontend directory

### 4.2 Docker Compose (`docker-compose.yml`)
- Single-service deployment, port 8080
- Volume mount `./data:/app/data`
- Config path via `PUREFS_CONFIG` env var
- Auto-restart (unless-stopped)

### 4.3 Build Artifacts
| File | Description |
|------|-------------|
| `PureFS-App/purefsd.exe` | Windows x86_64 binary |
| `PureFS-App/purefs-linux` | Linux x86_64 binary |
| `PureFS-App/purefs-linux-arm64` | Linux ARM64 binary (Raspberry Pi, etc.) |
| `PureFS-Web/dist/` | Pre-built frontend assets |
| `purefs-deploy.tar.gz` | x86_64 full deploy package (binary + frontend + config template) |
| `purefs-deploy-arm64.tar.gz` | ARM64 full deploy package |

---

## Current Status

### Completed ✓
- [x] File CRUD (upload/download/preview/rename/move/delete)
- [x] Three browse views (list / grid / timeline)
- [x] User authentication (register/login/JWT/2FA TOTP)
- [x] Multi-user permissions (path-based read/write/admin)
- [x] Share links (password protection/expiry/access limit/download control)
- [x] WebDAV protocol support
- [x] S3-compatible storage backend (MinIO/R2/OSS)
- [x] File integrity verification (SHA256, periodic checks)
- [x] Audit logging
- [x] Dark mode (sidebar toggle)
- [x] Pin & favorite markers (with dedicated pages)
- [x] Docker deployment (multi-stage, x86_64 + ARM64)
- [x] SFTP server (embedded SSH, password + key auth)
- [x] Server-side transparent encryption (AES-256-GCM, key rotation)
- [x] Recycle bin (30-day auto-cleanup)
- [x] Full-text search (SQLite FTS5, async indexing)
- [x] File versioning (N-version retention on overwrite)
- [x] Rate limiting middleware
- [x] Password strength validation
- [x] Context menu (favorite/pin/rename/download/share/delete)
- [x] Inline rename with ✓/✕ buttons
- [x] Storage quota enforcement
- [x] Admin dashboard (system stats, backup, user/permission management)
- [x] Internationalization setup (zh-CN / en-US locale files)

### Remaining □
- [x] SMB / SFTP protocol support (SFTP done, SMB deferred)
- [ ] Plugin system (gRPC sidecar — design doc exists in PLUGIN.md)
- [x] Server-side transparent encryption (AES-256-GCM with key rotation)
- [x] Trash/recycle bin (30-day auto-cleanup)
- [x] Full-text search (SQLite FTS5 — SEARCH.md)
- [ ] Desktop client (Wails framework — incremental sync, virtual drive)
- [ ] Mobile client (React Native — photo backup, offline cache)
- [x] Trash auto-cleanup
- [x] Internationalization (i18n) — infrastructure ready, partial implementation
- [ ] Test coverage

---

## Quick Start (for the next developer)

### 1. Start the backend
```bash
cd PureFS-App
export GOPROXY=https://goproxy.cn,direct
export GOMEMLIMIT="2097152000"
export GOGC="50"
go run ./cmd/purefsd/
```

### 2. Start the frontend (dev mode)
```bash
cd PureFS-Web
npm install --registry=https://registry.npmmirror.com
npx vite
```

### 3. Register an admin user
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","email":"admin@example.com","password":"purefs123"}'
```
Then manually update the `role` column to `admin` in `data/purefs.db`.

### 4. Key Dependencies
- **Go 1.26+** — configure the Chinese proxy (`GOPROXY=https://goproxy.cn,direct`) since github.com is blocked in CN
- **Node.js 22+** + npm for the frontend
- Database is SQLite (pure Go, no external dependency)

---

## Notes for the Next Session

1. Re-authorize Bash permissions for Go/NPM etc. (already configured in `.claude/settings.local.json`)
2. If GitHub access is needed, configure: `git config --global http.proxy https://gh.jasonzeng.dev`
3. Set Go proxy env vars: `GOPROXY=https://goproxy.cn,direct`, `GONOSUMCHECK="*"`, `GONOSUMDB="*"`
4. Set Go memory limits: `GOMEMLIMIT="2097152000"`, `GOGC="50"`
