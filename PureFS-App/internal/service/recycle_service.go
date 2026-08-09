package service

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/storage"
)

type RecycleService struct {
	fileRepo    *repository.FileRepo
	recycleRepo *repository.RecycleBinRepo
	userRepo    *repository.UserRepo
	store       storage.Storage
	cfg         *config.Config
}

func NewRecycleService(fileRepo *repository.FileRepo, recycleRepo *repository.RecycleBinRepo, userRepo *repository.UserRepo, store storage.Storage, cfg *config.Config) *RecycleService {
	return &RecycleService{
		fileRepo:    fileRepo,
		recycleRepo: recycleRepo,
		userRepo:    userRepo,
		store:       store,
		cfg:         cfg,
	}
}

func (s *RecycleService) Trash(userID int64, fileID int64) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if f.UserID != userID {
		return fmt.Errorf("permission denied")
	}

	// Build trash path: /.trash/{userID}/{fileID}_{name}
	trashPath := fmt.Sprintf("/.trash/%d/%d_%s", userID, fileID, f.Name)

	// Ensure trash directory exists
	trashDir := filepath.Dir(trashPath)
	if err := s.store.Mkdir(trashDir); err != nil {
		return fmt.Errorf("create trash directory: %w", err)
	}

	// Move file on storage to trash
	if err := s.store.Rename(f.Path, trashPath); err != nil {
		return fmt.Errorf("move file to trash: %w", err)
	}

	isDir := int64(0)
	fileType := "file"
	if f.FileType == model.FileTypeDir {
		isDir = 1
		fileType = "directory"
	}

	retentionDays := s.cfg.Trash.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	item := &model.RecycleBinItem{
		UserID:       userID,
		FileID:       fileID,
		OriginalPath: f.Path,
		OriginalName: f.Name,
		TrashPath:    trashPath,
		FileType:     fileType,
		FileSize:     f.Size,
		IsDir:        isDir,
		ExpireAt:     time.Now().Add(time.Duration(retentionDays) * 24 * time.Hour),
	}

	if err := s.recycleRepo.Create(item); err != nil {
		// Try to move file back on failure
		s.store.Rename(trashPath, f.Path)
		return fmt.Errorf("create recycle record: %w", err)
	}

	// Delete the file record from files table
	if err := s.fileRepo.Delete(fileID); err != nil {
		return fmt.Errorf("delete file record: %w", err)
	}

	return nil
}

func (s *RecycleService) Restore(userID int64, trashID int64) (*model.File, error) {
	item, err := s.recycleRepo.GetByID(trashID)
	if err != nil {
		return nil, fmt.Errorf("trash item not found: %w", err)
	}
	if item.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}

	// Move file back from trash to original path
	if err := s.store.Rename(item.TrashPath, item.OriginalPath); err != nil {
		return nil, fmt.Errorf("restore file from trash: %w", err)
	}

	// Re-create the file record
	fileType := model.FileTypeFile
	if item.IsDir == 1 {
		fileType = model.FileTypeDir
	}

	f := &model.File{
		UserID:   userID,
		Name:     item.OriginalName,
		Path:     item.OriginalPath,
		RealPath: s.store.RealPath(item.OriginalPath),
		FileType: fileType,
		Size:     item.FileSize,
	}

	if err := s.fileRepo.Create(f); err != nil {
		// Try to move file back to trash on failure
		s.store.Rename(item.OriginalPath, item.TrashPath)
		return nil, fmt.Errorf("create file record: %w", err)
	}

	// Delete the recycle bin record
	if err := s.recycleRepo.Delete(trashID); err != nil {
		return nil, fmt.Errorf("delete trash record: %w", err)
	}

	// Increase storage used by restored file size
	if item.FileSize > 0 {
		if err := s.userRepo.UpdateStorageUsed(userID, item.FileSize); err != nil {
			return nil, fmt.Errorf("update storage used: %w", err)
		}
	}

	return f, nil
}

func (s *RecycleService) PermanentlyDelete(userID int64, trashID int64) error {
	item, err := s.recycleRepo.GetByID(trashID)
	if err != nil {
		return fmt.Errorf("trash item not found: %w", err)
	}
	if item.UserID != userID {
		return fmt.Errorf("permission denied")
	}

	// Delete file from storage
	if err := s.store.Delete(item.TrashPath); err != nil {
		return fmt.Errorf("delete trash file from storage: %w", err)
	}

	// Delete recycle bin record
	if err := s.recycleRepo.Delete(trashID); err != nil {
		return fmt.Errorf("delete trash record: %w", err)
	}

	return nil
}

func (s *RecycleService) EmptyTrash(userID int64) error {
	items, _, err := s.recycleRepo.ListByUser(userID, 0, 1000)
	if err != nil {
		return fmt.Errorf("list trash items: %w", err)
	}

	for _, item := range items {
		// Delete from storage, ignore errors for individual files
		s.store.Delete(item.TrashPath)
	}

	// Delete all records for this user
	if err := s.recycleRepo.DeleteByUser(userID); err != nil {
		return fmt.Errorf("delete user trash records: %w", err)
	}

	return nil
}

func (s *RecycleService) CleanupExpired() (int64, error) {
	// Find expired items to delete their storage files first
	items, err := s.recycleRepo.ListExpired()
	if err != nil {
		return 0, fmt.Errorf("list expired items: %w", err)
	}

	for _, item := range items {
		s.store.Delete(item.TrashPath)
	}

	// Delete all expired records
	count, err := s.recycleRepo.DeleteExpired()
	if err != nil {
		return 0, fmt.Errorf("delete expired records: %w", err)
	}

	return count, nil
}

func (s *RecycleService) List(userID int64, offset, limit int) ([]*model.RecycleBinItem, int64, error) {
	return s.recycleRepo.ListByUser(userID, offset, limit)
}
