package repository

import (
	"database/sql"
	"time"

	"github.com/purefs/purefs/internal/model"
)

// FileVersionRepo manages file_versions table records.
type FileVersionRepo struct {
	db *sql.DB
}

// NewFileVersionRepo creates a new FileVersionRepo.
func NewFileVersionRepo(db *sql.DB) *FileVersionRepo {
	return &FileVersionRepo{db: db}
}

// Create inserts a new file version record.
func (r *FileVersionRepo) Create(v *model.FileVersion) error {
	now := time.Now()
	v.CreatedAt = now
	res, err := r.db.Exec(
		`INSERT INTO file_versions (file_id, version_num, size, sha256, storage_path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		v.FileID, v.VersionNum, v.Size, v.SHA256, v.StoragePath, v.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	v.ID = id
	return nil
}

// GetByID retrieves a single version by its primary key.
func (r *FileVersionRepo) GetByID(id int64) (*model.FileVersion, error) {
	v := &model.FileVersion{}
	err := r.db.QueryRow(
		`SELECT id, file_id, version_num, size, sha256, storage_path, created_at
		 FROM file_versions WHERE id = ?`, id,
	).Scan(&v.ID, &v.FileID, &v.VersionNum, &v.Size, &v.SHA256, &v.StoragePath, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ListByFile returns all versions for a given file, ordered by version number
// descending (newest first).
func (r *FileVersionRepo) ListByFile(fileID int64) ([]*model.FileVersion, error) {
	rows, err := r.db.Query(
		`SELECT id, file_id, version_num, size, sha256, storage_path, created_at
		 FROM file_versions WHERE file_id = ? ORDER BY version_num DESC`, fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*model.FileVersion
	for rows.Next() {
		v := &model.FileVersion{}
		if err := rows.Scan(&v.ID, &v.FileID, &v.VersionNum, &v.Size, &v.SHA256, &v.StoragePath, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// GetCount returns the number of versions stored for a file.
func (r *FileVersionRepo) GetCount(fileID int64) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM file_versions WHERE file_id = ?`, fileID,
	).Scan(&count)
	return count, err
}

// GetOldest returns the oldest version for a file (lowest version_num),
// useful for pruning when max versions is exceeded.
func (r *FileVersionRepo) GetOldest(fileID int64) (*model.FileVersion, error) {
	v := &model.FileVersion{}
	err := r.db.QueryRow(
		`SELECT id, file_id, version_num, size, sha256, storage_path, created_at
		 FROM file_versions WHERE file_id = ? ORDER BY version_num ASC LIMIT 1`, fileID,
	).Scan(&v.ID, &v.FileID, &v.VersionNum, &v.Size, &v.SHA256, &v.StoragePath, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// GetNextVersionNum returns the next version number for a file (max + 1).
func (r *FileVersionRepo) GetNextVersionNum(fileID int64) (int, error) {
	var maxNum sql.NullInt64
	err := r.db.QueryRow(
		`SELECT MAX(version_num) FROM file_versions WHERE file_id = ?`, fileID,
	).Scan(&maxNum)
	if err != nil {
		return 1, nil
	}
	if !maxNum.Valid {
		return 1, nil
	}
	return int(maxNum.Int64) + 1, nil
}

// Delete removes a single version by its ID.
func (r *FileVersionRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM file_versions WHERE id = ?`, id)
	return err
}

// DeleteByFile removes all versions for a given file.
func (r *FileVersionRepo) DeleteByFile(fileID int64) error {
	_, err := r.db.Exec(`DELETE FROM file_versions WHERE file_id = ?`, fileID)
	return err
}
