package service

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/storage"
)

// VersionService manages file version snapshots and restoration.
type VersionService struct {
	versionRepo *repository.FileVersionRepo
	store       storage.Storage
	cfg         *config.Config
}

// NewVersionService creates a new VersionService.
func NewVersionService(versionRepo *repository.FileVersionRepo, store storage.Storage, cfg *config.Config) *VersionService {
	return &VersionService{
		versionRepo: versionRepo,
		store:       store,
		cfg:         cfg,
	}
}

// SaveVersion copies the current state of the given file to a versioned
// snapshot under /.versions/{userID}/{fileID}/v{num}_{ts}, then prunes
// excess versions if the count exceeds the configured maximum.
func (s *VersionService) SaveVersion(file *model.File) error {
	nextNum, err := s.versionRepo.GetNextVersionNum(file.ID)
	if err != nil {
		return fmt.Errorf("get next version num: %w", err)
	}

	// Build version storage path
	ts := time.Now().Unix()
	versionPath := buildVersionPath(file.UserID, file.ID, nextNum, ts)

	// Copy the current file to the version path
	if err := s.store.Copy(file.Path, versionPath); err != nil {
		return fmt.Errorf("copy file to version storage: %w", err)
	}

	v := &model.FileVersion{
		FileID:      file.ID,
		VersionNum:  nextNum,
		Size:        file.Size,
		SHA256:      file.SHA256,
		StoragePath: versionPath,
	}

	if err := s.versionRepo.Create(v); err != nil {
		// Attempt to clean up the copied version file on failure
		s.store.Delete(versionPath)
		return fmt.Errorf("create version record: %w", err)
	}

	// Prune old versions if exceeding max
	maxVersions := s.cfg.Versioning.MaxVersions
	if maxVersions <= 0 {
		maxVersions = 10
	}
	if err := s.pruneOldest(file.ID, file.UserID, maxVersions); err != nil {
		// Non-fatal: the version is saved, cleanup just failed
		return nil
	}

	return nil
}

// ListVersions returns all saved versions for a file.
func (s *VersionService) ListVersions(fileID int64) ([]*model.FileVersion, error) {
	return s.versionRepo.ListByFile(fileID)
}

// RestoreVersion restores a file to a previous version by copying the version
// snapshot back over the current file and updating the file record's size
// and SHA256. The caller must provide a function to update the file record.
func (s *VersionService) RestoreVersion(fileID int64, versionID int64, currentFile *model.File, updateFileFn func(f *model.File) error) error {
	ver, err := s.versionRepo.GetByID(versionID)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}
	if ver.FileID != fileID {
		return fmt.Errorf("version does not belong to this file")
	}

	// Verify the version file still exists
	exists, err := s.store.Exists(ver.StoragePath)
	if err != nil {
		return fmt.Errorf("check version file: %w", err)
	}
	if !exists {
		return fmt.Errorf("version file missing from storage")
	}

	// Save the current state as a new version before overwriting
	if err := s.SaveVersion(currentFile); err != nil {
		return fmt.Errorf("save current state before restore: %w", err)
	}

	// Copy the version file over the current file
	if err := s.store.Copy(ver.StoragePath, currentFile.Path); err != nil {
		return fmt.Errorf("restore version to file: %w", err)
	}

	// Update the file record
	currentFile.Size = ver.Size
	currentFile.SHA256 = ver.SHA256
	if err := updateFileFn(currentFile); err != nil {
		return fmt.Errorf("update file record: %w", err)
	}

	return nil
}

// DeleteVersion removes a single version record and its storage file.
func (s *VersionService) DeleteVersion(versionID int64) error {
	ver, err := s.versionRepo.GetByID(versionID)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// Delete the version file from storage
	if delErr := s.store.Delete(ver.StoragePath); delErr != nil {
		// Log but continue; the record is the source of truth
	}

	return s.versionRepo.Delete(versionID)
}

// DeleteAllVersions removes all versions for a file, including storage files.
func (s *VersionService) DeleteAllVersions(fileID int64) error {
	versions, err := s.versionRepo.ListByFile(fileID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		s.store.Delete(v.StoragePath)
	}
	return s.versionRepo.DeleteByFile(fileID)
}

// DownloadVersion opens a version snapshot for reading.
func (s *VersionService) DownloadVersion(versionID int64) (io.ReadCloser, *model.FileVersion, error) {
	ver, err := s.versionRepo.GetByID(versionID)
	if err != nil {
		return nil, nil, fmt.Errorf("version not found: %w", err)
	}
	reader, err := s.store.Open(ver.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open version file: %w", err)
	}
	return reader, ver, nil
}

// pruneOldest removes the oldest versions until the count is at or below the
// maximum allowed.
func (s *VersionService) pruneOldest(fileID int64, userID int64, maxVersions int) error {
	for {
		count, err := s.versionRepo.GetCount(fileID)
		if err != nil {
			return err
		}
		if count <= maxVersions {
			return nil
		}

		oldest, err := s.versionRepo.GetOldest(fileID)
		if err != nil {
			return err
		}

		// Delete the storage file (ignore errors; record is source of truth)
		s.store.Delete(oldest.StoragePath)

		if err := s.versionRepo.Delete(oldest.ID); err != nil {
			return err
		}
	}
}

// buildVersionPath constructs the storage path for a version snapshot.
func buildVersionPath(userID, fileID int64, versionNum int, timestamp int64) string {
	name := fmt.Sprintf("v%d_%d", versionNum, timestamp)
	return filepath.Join(fmt.Sprintf("/.versions/%d/%d", userID, fileID), name)
}
