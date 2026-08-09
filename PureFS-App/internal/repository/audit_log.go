package repository

import (
	"database/sql"

	"github.com/purefs/purefs/internal/model"
)

type AuditLogRepo struct {
	db *sql.DB
}

func NewAuditLogRepo(db *sql.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(log *model.AuditLog) error {
	_, err := r.db.Exec(
		`INSERT INTO audit_logs (user_id, action, detail, ip, created_at) VALUES (?, ?, ?, ?, datetime('now'))`,
		log.UserID, log.Action, log.Detail, log.IP,
	)
	return err
}

func (r *AuditLogRepo) List(limit, offset int) ([]*model.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, user_id, action, detail, ip, created_at FROM audit_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*model.AuditLog
	for rows.Next() {
		l := &model.AuditLog{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.Action, &l.Detail, &l.IP, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
