package repository

import (
	"database/sql"
	"time"

	"github.com/purefs/purefs/internal/model"
)

type ShareRepo struct {
	db *sql.DB
}

func NewShareRepo(db *sql.DB) *ShareRepo {
	return &ShareRepo{db: db}
}

func (r *ShareRepo) Create(s *model.Share) error {
	res, err := r.db.Exec(
		`INSERT INTO shares (user_id, file_id, token, password, expires_at, max_accesses, can_download, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.UserID, s.FileID, s.Token, s.Password, s.ExpiresAt, s.MaxAccesses, s.CanDownload, s.IsActive, time.Now(),
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.ID = id
	return nil
}

func (r *ShareRepo) GetByToken(token string) (*model.Share, error) {
	s := &model.Share{}
	err := r.db.QueryRow(
		`SELECT id, user_id, file_id, token, password, expires_at, max_accesses, access_count, can_download, is_active, created_at
		 FROM shares WHERE token = ?`, token,
	).Scan(&s.ID, &s.UserID, &s.FileID, &s.Token, &s.Password, &s.ExpiresAt, &s.MaxAccesses, &s.AccessCount, &s.CanDownload, &s.IsActive, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *ShareRepo) ListByUser(userID int64) ([]*model.Share, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, file_id, token, password, expires_at, max_accesses, access_count, can_download, is_active, created_at
		 FROM shares WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shares []*model.Share
	for rows.Next() {
		s := &model.Share{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.FileID, &s.Token, &s.Password, &s.ExpiresAt, &s.MaxAccesses, &s.AccessCount, &s.CanDownload, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	return shares, nil
}

func (r *ShareRepo) IncrementAccess(id int64) error {
	_, err := r.db.Exec(`UPDATE shares SET access_count = access_count + 1 WHERE id = ?`, id)
	return err
}

func (r *ShareRepo) Deactivate(id int64) error {
	_, err := r.db.Exec(`UPDATE shares SET is_active = 0 WHERE id = ?`, id)
	return err
}
