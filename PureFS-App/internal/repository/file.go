package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/purefs/purefs/internal/model"
)

type FileRepo struct {
	db *sql.DB
}

func NewFileRepo(db *sql.DB) *FileRepo {
	return &FileRepo{db: db}
}

func (r *FileRepo) Create(f *model.File) error {
	now := time.Now()
	f.CreatedAt = now
	f.UpdatedAt = now
	res, err := r.db.Exec(
		`INSERT INTO files (user_id, parent_id, name, path, real_path, file_type, mime_type, size, sha256, is_pinned, is_favorite, is_encrypted, dek_ciphertext, kek_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.UserID, f.ParentID, f.Name, f.Path, f.RealPath, f.FileType, f.MimeType, f.Size, f.SHA256, f.IsPinned, f.IsFavorite, f.IsEncrypted, f.DEKCiphertext, f.KEKVersion, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	f.ID = id
	return nil
}

func (r *FileRepo) GetByID(id int64) (*model.File, error) {
	f := &model.File{}
	err := r.db.QueryRow(
		`SELECT id, user_id, parent_id, name, path, real_path, file_type, mime_type, size, sha256, is_pinned, is_favorite, is_encrypted, dek_ciphertext, kek_version, created_at, updated_at
		 FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.Path, &f.RealPath, &f.FileType, &f.MimeType, &f.Size, &f.SHA256, &f.IsPinned, &f.IsFavorite, &f.IsEncrypted, &f.DEKCiphertext, &f.KEKVersion, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *FileRepo) GetByUserAndPath(userID int64, path string) (*model.File, error) {
	f := &model.File{}
	err := r.db.QueryRow(
		`SELECT id, user_id, parent_id, name, path, real_path, file_type, mime_type, size, sha256, is_pinned, is_favorite, is_encrypted, dek_ciphertext, kek_version, created_at, updated_at
		 FROM files WHERE user_id = ? AND path = ?`, userID, path,
	).Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.Path, &f.RealPath, &f.FileType, &f.MimeType, &f.Size, &f.SHA256, &f.IsPinned, &f.IsFavorite, &f.IsEncrypted, &f.DEKCiphertext, &f.KEKVersion, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *FileRepo) List(q model.FileListQuery) ([]*model.File, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if q.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *q.UserID)
	}

	if q.Search != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}

	if q.FileType != nil {
		where = append(where, "file_type = ?")
		args = append(args, string(*q.FileType))
	}

	if q.IsFavorite != nil && *q.IsFavorite {
		where = append(where, "is_favorite = 1")
	}

	if q.IsPinned != nil && *q.IsPinned {
		where = append(where, "is_pinned = 1")
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM files WHERE %s", whereClause)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortBy := "created_at"
	sortOrder := "DESC"
	if q.SortBy != "" {
		sortBy = q.SortBy
	}
	if q.SortOrder != "" {
		sortOrder = q.SortOrder
	}

	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}

	query := fmt.Sprintf(
		`SELECT id, user_id, parent_id, name, path, real_path, file_type, mime_type, size, sha256, is_pinned, is_favorite, is_encrypted, dek_ciphertext, kek_version, created_at, updated_at
		 FROM files WHERE %s ORDER BY is_pinned DESC, %s %s LIMIT ? OFFSET ?`,
		whereClause, sortBy, sortOrder,
	)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []*model.File
	for rows.Next() {
		f := &model.File{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.Path, &f.RealPath, &f.FileType, &f.MimeType, &f.Size, &f.SHA256, &f.IsPinned, &f.IsFavorite, &f.IsEncrypted, &f.DEKCiphertext, &f.KEKVersion, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	return files, total, nil
}

func (r *FileRepo) ListByParent(userID int64, parentID *int64, q model.FileListQuery) ([]*model.File, int64, error) {
	where := []string{"user_id = ?", "parent_id IS ?"}
	args := []interface{}{userID, parentID}

	if q.Search != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}

	if q.FileType != nil {
		where = append(where, "file_type = ?")
		args = append(args, string(*q.FileType))
	}

	if q.IsFavorite != nil && *q.IsFavorite {
		where = append(where, "is_favorite = 1")
	}

	if q.IsPinned != nil && *q.IsPinned {
		where = append(where, "is_pinned = 1")
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM files WHERE %s", whereClause)
	r.db.QueryRow(countQuery, args...).Scan(&total)

	sortBy := "created_at"
	sortOrder := "DESC"
	if q.SortBy != "" {
		sortBy = q.SortBy
	}
	if q.SortOrder != "" {
		sortOrder = q.SortOrder
	}

	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}

	query := fmt.Sprintf(
		`SELECT id, user_id, parent_id, name, path, real_path, file_type, mime_type, size, sha256, is_pinned, is_favorite, is_encrypted, dek_ciphertext, kek_version, created_at, updated_at
		 FROM files WHERE %s ORDER BY is_pinned DESC, %s %s LIMIT ? OFFSET ?`,
		whereClause, sortBy, sortOrder,
	)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []*model.File
	for rows.Next() {
		f := &model.File{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.Path, &f.RealPath, &f.FileType, &f.MimeType, &f.Size, &f.SHA256, &f.IsPinned, &f.IsFavorite, &f.IsEncrypted, &f.DEKCiphertext, &f.KEKVersion, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	return files, total, nil
}

func (r *FileRepo) Update(f *model.File) error {
	f.UpdatedAt = time.Now()
	_, err := r.db.Exec(
		`UPDATE files SET parent_id=?, name=?, path=?, real_path=?, mime_type=?, size=?, sha256=?, is_pinned=?, is_favorite=?, is_encrypted=?, updated_at=?
		 WHERE id=?`,
		f.ParentID, f.Name, f.Path, f.RealPath, f.MimeType, f.Size, f.SHA256, f.IsPinned, f.IsFavorite, f.IsEncrypted, f.UpdatedAt, f.ID,
	)
	return err
}

func (r *FileRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM files WHERE id = ?`, id)
	return err
}

func (r *FileRepo) SetPinned(id int64, pinned bool) error {
	_, err := r.db.Exec(`UPDATE files SET is_pinned = ?, updated_at = ? WHERE id = ?`, pinned, time.Now(), id)
	return err
}

func (r *FileRepo) SetFavorite(id int64, fav bool) error {
	_, err := r.db.Exec(`UPDATE files SET is_favorite = ?, updated_at = ? WHERE id = ?`, fav, time.Now(), id)
	return err
}

func (r *FileRepo) Move(id int64, newParentID *int64, newPath string) error {
	_, err := r.db.Exec(`UPDATE files SET parent_id = ?, path = ?, updated_at = ? WHERE id = ?`, newParentID, newPath, time.Now(), id)
	return err
}

func (r *FileRepo) Rename(id int64, newName, newPath string) error {
	_, err := r.db.Exec(`UPDATE files SET name = ?, path = ?, updated_at = ? WHERE id = ?`, newName, newPath, time.Now(), id)
	return err
}

func (r *FileRepo) DeleteByUserAndPath(userID int64, path string) error {
	_, err := r.db.Exec(`DELETE FROM files WHERE user_id = ? AND path = ?`, userID, path)
	return err
}
