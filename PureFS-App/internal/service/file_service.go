package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/storage"
)

// checkPerm verifies user has at least the given permission for a file.
// Finds the longest matching permission prefix and evaluates against it.
// If no specific permission is set, falls back to file ownership.
func (s *FileService) checkPerm(userID int64, f *model.File, require string) error {
	// File owner has full access
	if f.UserID == userID {
		return nil
	}

	// Check explicit permissions
	perms, err := s.permRepo.GetByUser(userID)
	if err != nil {
		return err
	}

	// Find the longest matching permission prefix.
	// We require the match to be either an exact match or the permission
	// prefix is a parent directory (ends with "/") of the file path.
	// This prevents "/data/docs" from matching "/data/documentation/file.txt".
	var best *model.Permission
	bestLen := 0
	for i := range perms {
		p := perms[i]
		// Permission matches if it's an exact prefix AND the next character is
		// either end-of-string (exact match) or a "/" (parent directory match).
		matchLen := len(p.FilePath)
		if strings.HasPrefix(f.Path, p.FilePath) &&
			(len(f.Path) == matchLen || f.Path[matchLen] == '/') {
			if matchLen > bestLen {
				bestLen = matchLen
				best = p
			}
		}
	}

	if best == nil {
		return fmt.Errorf("permission denied")
	}

	// Evaluate against the single best (longest) matching permission.
	switch best.Perm {
	case "admin":
		return nil
	case "write":
		return nil
	case "read":
		if require == "read" {
			return nil
		}
	}

	return fmt.Errorf("permission denied")
}

type FileService struct {
	fileRepo    *repository.FileRepo
	permRepo    *repository.PermissionRepo
	userRepo    *repository.UserRepo
	recycleSvc  *RecycleService
	versionSvc  *VersionService
	auditSvc    *AuditService
	store       storage.Storage
	cfg         *config.Config
	searchSvc   *SearchService
}

func NewFileService(fileRepo *repository.FileRepo, permRepo *repository.PermissionRepo, userRepo *repository.UserRepo, recycleSvc *RecycleService, store storage.Storage, cfg *config.Config) *FileService {
	return &FileService{fileRepo: fileRepo, permRepo: permRepo, userRepo: userRepo, recycleSvc: recycleSvc, store: store, cfg: cfg}
}

// SetAuditService sets the audit logging service.
func (s *FileService) SetAuditService(auditSvc *AuditService) {
	s.auditSvc = auditSvc
}

// logAudit records an audit log entry (best-effort, never fails the operation).
func (s *FileService) logAudit(userID int64, action, detail string) {
	if s.auditSvc == nil {
		return
	}
	_ = s.auditSvc.Log(userID, action, detail, "")
}

// SetSearchService sets the search service for async indexing after upload/delete.
func (s *FileService) SetSearchService(searchSvc *SearchService) {
	s.searchSvc = searchSvc
}

// SetVersionService sets the version service for file versioning on upload.
func (s *FileService) SetVersionService(versionSvc *VersionService) {
	s.versionSvc = versionSvc
}

func (s *FileService) CreateDir(userID int64, parentID *int64, name string) (*model.File, error) {
	if parentID != nil {
		parent, err := s.fileRepo.GetByID(*parentID)
		if err != nil {
			return nil, fmt.Errorf("parent not found: %w", err)
		}
		if err := s.checkPerm(userID, parent, "write"); err != nil {
			return nil, err
		}
	}

	userPath := fmt.Sprintf("/users/%d", userID)

	f := &model.File{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
		Path:     filepath.Join(userPath, name),
		FileType: model.FileTypeDir,
	}

	// Create actual directory on storage
	if err := s.store.Mkdir(f.Path); err != nil {
		return nil, fmt.Errorf("create directory on storage: %w", err)
	}

	if err := s.fileRepo.Create(f); err != nil {
		s.store.Delete(f.Path)
		return nil, fmt.Errorf("create directory record: %w", err)
	}

	s.logAudit(userID, "create_dir", f.Path)
	return f, nil
}

func (s *FileService) Upload(userID int64, parentID *int64, name string, reader io.Reader) (*model.File, error) {
	userPath := fmt.Sprintf("/users/%d", userID)
	filePath := filepath.Join(userPath, name)

	// Check if a file with the same name exists in the parent directory.
	// If it does, save a version snapshot before overwriting.
	existingFiles, _, err := s.fileRepo.ListByParent(userID, parentID, model.FileListQuery{})
	if err == nil {
		for _, existing := range existingFiles {
			if existing.Name == name && existing.FileType == model.FileTypeFile {
				if s.versionSvc != nil {
					if verr := s.versionSvc.SaveVersion(existing); verr != nil {
						// Non-fatal; continue with the upload even if version save fails
					}
				}
				break
			}
		}
	}

	tmpPath := filePath + ".uploading"

	// Write upload data
	w, err := s.store.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	h := sha256.New()
	multiWriter := io.MultiWriter(w, h)

	size, err := io.Copy(multiWriter, reader)
	if err != nil {
		w.Close()
		s.store.Delete(tmpPath)
		return nil, fmt.Errorf("write upload data: %w", err)
	}

	if err := w.Close(); err != nil {
		s.store.Delete(tmpPath)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	sha256Hex := hex.EncodeToString(h.Sum(nil))

	// Enforce storage quota
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.StorageQuota > 0 && user.StorageUsed+size > user.StorageQuota {
		s.store.Delete(tmpPath)
		return nil, fmt.Errorf("storage quota exceeded")
	}

	if err := s.store.Rename(tmpPath, filePath); err != nil {
		s.store.Delete(tmpPath)
		return nil, fmt.Errorf("finalize upload: %w", err)
	}

	mimeType := detectMimeType(name)

	f := &model.File{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
		Path:     filePath,
		RealPath: s.store.RealPath(filePath),
		FileType: model.FileTypeFile,
		MimeType: mimeType,
		Size:     size,
		SHA256:   sha256Hex,
	}

	if err := s.fileRepo.Create(f); err != nil {
		return nil, fmt.Errorf("create file record: %w", err)
	}

	// Update user's storage used
	if err := s.userRepo.UpdateStorageUsed(userID, size); err != nil {
		return nil, fmt.Errorf("update storage used: %w", err)
	}

	// Submit async indexing task
	if s.searchSvc != nil {
		s.searchSvc.IndexFileAsync(f.ID, userID)
	}

	s.logAudit(userID, "upload", f.Path)
	return f, nil
}

func (s *FileService) Download(userID int64, fileID int64) (io.ReadCloser, *model.File, error) {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}
	if err := s.checkPerm(userID, f, "read"); err != nil {
		return nil, nil, err
	}

	reader, err := s.store.Open(f.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}

	return reader, f, nil
}

func (s *FileService) List(userID int64, q model.FileListQuery) ([]*model.File, int64, error) {
	q.UserID = &userID
	if q.ParentID != nil {
		return s.fileRepo.ListByParent(userID, q.ParentID, q)
	}
	return s.fileRepo.List(q)
}

func (s *FileService) Rename(userID int64, fileID int64, newName string) (*model.File, error) {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if err := s.checkPerm(userID, f, "write"); err != nil {
		return nil, err
	}

	oldPath := f.Path
	newPath := filepath.Join(filepath.Dir(oldPath), newName)

	if err := s.store.Rename(oldPath, newPath); err != nil {
		if !strings.Contains(err.Error(), "cannot find") && !strings.Contains(err.Error(), "NoSuchKey") && !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("rename on storage: %w", err)
		}
	}

	f.Name = newName
	f.Path = newPath
	if err := s.fileRepo.Rename(fileID, newName, newPath); err != nil {
		return nil, fmt.Errorf("rename record: %w", err)
	}

	s.logAudit(userID, "rename", oldPath+" -> "+newPath)
	return f, nil
}

func (s *FileService) Move(userID int64, fileID int64, targetParentID int64) (*model.File, error) {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if err := s.checkPerm(userID, f, "write"); err != nil {
		return nil, err
	}

	target, err := s.fileRepo.GetByID(targetParentID)
	if err != nil {
		return nil, fmt.Errorf("target not found: %w", err)
	}
	if err := s.checkPerm(userID, target, "write"); err != nil {
		return nil, err
	}

	oldPath := f.Path
	newPath := filepath.Join(target.Path, f.Name)

	if err := s.store.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("move on storage: %w", err)
	}

	f.ParentID = &targetParentID
	f.Path = newPath
	if err := s.fileRepo.Move(fileID, &targetParentID, newPath); err != nil {
		return nil, fmt.Errorf("move record: %w", err)
	}

	return f, nil
}

func (s *FileService) Delete(userID int64, fileID int64) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if err := s.checkPerm(userID, f, "write"); err != nil {
		return err
	}

	if err := s.recycleSvc.Trash(userID, fileID); err != nil {
		return err
	}

	// Submit async de-indexing task
	if s.searchSvc != nil {
		s.searchSvc.RemoveFromIndexAsync(f.Path)
	}

	// Decrease storage used by the file's size
	if f.Size > 0 {
		if err := s.userRepo.UpdateStorageUsed(userID, -f.Size); err != nil {
			return fmt.Errorf("update storage used: %w", err)
		}
	}

	s.logAudit(userID, "delete", f.Path)
	return nil
}

func (s *FileService) SetPinned(userID int64, fileID int64, pinned bool) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}
	if f.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	return s.fileRepo.SetPinned(fileID, pinned)
}

func (s *FileService) SetFavorite(userID int64, fileID int64, fav bool) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}
	if f.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	return s.fileRepo.SetFavorite(fileID, fav)
}

func (s *FileService) GetFile(userID, fileID int64) (*model.File, error) {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, err
	}
	if f.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	return f, nil
}

// UpdateFile updates the file record fields (size, sha256, mime_type, etc.)
// in the database. It is used internally, e.g. by version restore.
func (s *FileService) UpdateFile(f *model.File) error {
	return s.fileRepo.Update(f)
}

// DownloadPublic opens a file for reading without user ownership check.
// The caller (e.g. share handler) is responsible for access validation.
func (s *FileService) DownloadPublic(fileID int64) (io.ReadCloser, *model.File, error) {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	reader, err := s.store.Open(f.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}

	return reader, f, nil
}

// Copy creates a copy of a file within the target directory.
func (s *FileService) Copy(userID int64, fileID int64, targetParentID *int64, newName string) (*model.File, error) {
	src, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("source file not found: %w", err)
	}
	if err := s.checkPerm(userID, src, "read"); err != nil {
		return nil, err
	}

	if targetParentID != nil {
		target, err := s.fileRepo.GetByID(*targetParentID)
		if err != nil {
			return nil, fmt.Errorf("target not found: %w", err)
		}
		if err := s.checkPerm(userID, target, "write"); err != nil {
			return nil, err
		}
	}

	if newName == "" {
		newName = src.Name
	}

	userPath := fmt.Sprintf("/users/%d", userID)
	dstPath := filepath.Join(userPath, newName)
	if targetParentID != nil {
		var target *model.File
		target, err = s.fileRepo.GetByID(*targetParentID)
		if err != nil {
			return nil, fmt.Errorf("target parent not found: %w", err)
		}
		dstPath = filepath.Join(target.Path, newName)
	}

	// Open source and copy bytes
	srcReader, err := s.store.Open(src.Path)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	defer srcReader.Close()

	dstWriter, err := s.store.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("create destination: %w", err)
	}

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(dstWriter, h), srcReader)
	if err != nil {
		dstWriter.Close()
		s.store.Delete(dstPath)
		return nil, fmt.Errorf("copy data: %w", err)
	}
	if err := dstWriter.Close(); err != nil {
		s.store.Delete(dstPath)
		return nil, fmt.Errorf("finalize copy: %w", err)
	}

	f := &model.File{
		UserID:   userID,
		ParentID: targetParentID,
		Name:     newName,
		Path:     dstPath,
		RealPath: s.store.RealPath(dstPath),
		FileType: src.FileType,
		MimeType: src.MimeType,
		Size:     size,
		SHA256:   hex.EncodeToString(h.Sum(nil)),
	}

	if err := s.fileRepo.Create(f); err != nil {
		s.store.Delete(dstPath)
		return nil, fmt.Errorf("create copy record: %w", err)
	}

	// Update storage used
	if size > 0 {
		if err := s.userRepo.UpdateStorageUsed(userID, size); err != nil {
			return nil, fmt.Errorf("update storage used: %w", err)
		}
	}

	s.logAudit(userID, "copy", src.Path+" -> "+dstPath)
	return f, nil
}

func detectMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	mimeMap := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
		".ico":  "image/x-icon",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".mkv":  "video/x-matroska",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".flac": "audio/flac",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".zip":  "application/zip",
		".rar":  "application/vnd.rar",
		".7z":   "application/x-7z-compressed",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".md":   "text/markdown",
		".csv":  "text/csv",
	}
	if mime, ok := mimeMap[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
