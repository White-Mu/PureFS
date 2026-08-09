package repository

import (
	"database/sql"
	"time"

	"github.com/purefs/purefs/internal/model"
)

type RecycleBinRepo struct {
	db *sql.DB
}

func NewRecycleBinRepo(db *sql.DB) *RecycleBinRepo {
	return &RecycleBinRepo{db: db}
}

func (r *RecycleBinRepo) Create(item *model.RecycleBinItem) error {
	now := time.Now()
	item.DeletedAt = now
	res, err := r.db.Exec(
		`INSERT INTO recycle_bin (user_id, file_id, original_path, original_name, trash_path, file_type, file_size, is_dir, deleted_at, expire_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.UserID, item.FileID, item.OriginalPath, item.OriginalName, item.TrashPath,
		item.FileType, item.FileSize, item.IsDir, item.DeletedAt, item.ExpireAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	item.ID = id
	return nil
}

func (r *RecycleBinRepo) ListByUser(userID int64, offset, limit int) ([]*model.RecycleBinItem, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var total int64
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM recycle_bin WHERE user_id = ?`, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		`SELECT id, user_id, file_id, original_path, original_name, trash_path, file_type, file_size, is_dir, deleted_at, expire_at
		 FROM recycle_bin WHERE user_id = ? ORDER BY deleted_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*model.RecycleBinItem
	for rows.Next() {
		item := &model.RecycleBinItem{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.FileID, &item.OriginalPath, &item.OriginalName,
			&item.TrashPath, &item.FileType, &item.FileSize, &item.IsDir, &item.DeletedAt, &item.ExpireAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (r *RecycleBinRepo) GetByID(id int64) (*model.RecycleBinItem, error) {
	item := &model.RecycleBinItem{}
	err := r.db.QueryRow(
		`SELECT id, user_id, file_id, original_path, original_name, trash_path, file_type, file_size, is_dir, deleted_at, expire_at
		 FROM recycle_bin WHERE id = ?`, id,
	).Scan(&item.ID, &item.UserID, &item.FileID, &item.OriginalPath, &item.OriginalName,
		&item.TrashPath, &item.FileType, &item.FileSize, &item.IsDir, &item.DeletedAt, &item.ExpireAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *RecycleBinRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM recycle_bin WHERE id = ?`, id)
	return err
}

func (r *RecycleBinRepo) DeleteExpired() (int64, error) {
	res, err := r.db.Exec(`DELETE FROM recycle_bin WHERE expire_at <= ?`, time.Now())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *RecycleBinRepo) DeleteByUser(userID int64) error {
	_, err := r.db.Exec(`DELETE FROM recycle_bin WHERE user_id = ?`, userID)
	return err
}

func (r *RecycleBinRepo) ListExpired() ([]*model.RecycleBinItem, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, file_id, original_path, original_name, trash_path, file_type, file_size, is_dir, deleted_at, expire_at
		 FROM recycle_bin WHERE expire_at <= ?`, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.RecycleBinItem
	for rows.Next() {
		item := &model.RecycleBinItem{}
		if err := rows.Scan(&item.ID, &item.UserID, &item.FileID, &item.OriginalPath, &item.OriginalName,
			&item.TrashPath, &item.FileType, &item.FileSize, &item.IsDir, &item.DeletedAt, &item.ExpireAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
