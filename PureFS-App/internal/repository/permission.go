package repository

import (
	"database/sql"

	"github.com/purefs/purefs/internal/model"
)

type PermissionRepo struct {
	db *sql.DB
}

func NewPermissionRepo(db *sql.DB) *PermissionRepo {
	return &PermissionRepo{db: db}
}

func (r *PermissionRepo) Create(p *model.Permission) error {
	_, err := r.db.Exec(
		`INSERT INTO permissions (user_id, file_path, perm) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, file_path) DO UPDATE SET perm = ?`,
		p.UserID, p.FilePath, p.Perm, p.Perm,
	)
	return err
}

func (r *PermissionRepo) GetByUser(userID int64) ([]*model.Permission, error) {
	rows, err := r.db.Query(`SELECT id, user_id, file_path, perm FROM permissions WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []*model.Permission
	for rows.Next() {
		p := &model.Permission{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.FilePath, &p.Perm); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func (r *PermissionRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM permissions WHERE id = ?`, id)
	return err
}
