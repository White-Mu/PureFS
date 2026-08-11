package repository

import (
	"database/sql"
	"time"

	"github.com/purefs/purefs/internal/model"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(u *model.User) error {
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	res, err := r.db.Exec(
		`INSERT INTO users (username, email, password_hash, role, totp_secret, totp_enabled, storage_quota, root_dir, is_active, e2ee_salt, e2ee_wrapped_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.PasswordHash, u.Role, u.TOTPSecret, u.TOTPEnabled, u.StorageQuota, u.RootDir, u.IsActive, u.E2EESalt, u.E2EEWrappedKey, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return nil
}

func (r *UserRepo) GetByID(id int64) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, storage_quota, storage_used, root_dir, is_active, ssh_public_key, e2ee_salt, e2ee_wrapped_key, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.TOTPEnabled, &u.StorageQuota, &u.StorageUsed, &u.RootDir, &u.IsActive, &u.SSHPublicKey, &u.E2EESalt, &u.E2EEWrappedKey, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, storage_quota, storage_used, root_dir, is_active, ssh_public_key, e2ee_salt, e2ee_wrapped_key, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.TOTPEnabled, &u.StorageQuota, &u.StorageUsed, &u.RootDir, &u.IsActive, &u.SSHPublicKey, &u.E2EESalt, &u.E2EEWrappedKey, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) List() ([]*model.User, error) {
	rows, err := r.db.Query(
		`SELECT id, username, email, role, storage_quota, storage_used, is_active, created_at, updated_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []*model.User{}
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.StorageQuota, &u.StorageUsed, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepo) UpdateStorageUsed(userID int64, delta int64) error {
	_, err := r.db.Exec(`UPDATE users SET storage_used = storage_used + ?, updated_at = ? WHERE id = ?`, delta, time.Now(), userID)
	return err
}

func (r *UserRepo) Update(u *model.User) error {
	u.UpdatedAt = time.Now()
	_, err := r.db.Exec(
		`UPDATE users SET email=?, password_hash=?, role=?, totp_secret=?, totp_enabled=?, storage_quota=?, is_active=?, updated_at=? WHERE id=?`,
		u.Email, u.PasswordHash, u.Role, u.TOTPSecret, u.TOTPEnabled, u.StorageQuota, u.IsActive, u.UpdatedAt, u.ID,
	)
	return err
}

func (r *UserRepo) SetResetToken(id int64, token string, expires time.Time) error {
	_, err := r.db.Exec(
		`UPDATE users SET reset_token=?, reset_token_expires=?, updated_at=? WHERE id=?`,
		token, expires, time.Now(), id,
	)
	return err
}

// SetE2EEKeys stores the E2EE salt and wrapped master key for a user.
func (r *UserRepo) SetE2EEKeys(id int64, salt, wrappedKey string) error {
	_, err := r.db.Exec(
		`UPDATE users SET e2ee_salt=?, e2ee_wrapped_key=?, updated_at=? WHERE id=?`,
		salt, wrappedKey, time.Now(), id,
	)
	return err
}

// ClearE2EE disables E2EE for a user by removing the salt and wrapped key.
func (r *UserRepo) ClearE2EE(id int64) error {
	_, err := r.db.Exec(
		`UPDATE users SET e2ee_salt='', e2ee_wrapped_key='', updated_at=? WHERE id=?`,
		time.Now(), id,
	)
	return err
}

func (r *UserRepo) GetByResetToken(token string) (*model.User, error) {
	u := &model.User{}
	var resetExpires *time.Time
	err := r.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled,
		 storage_quota, storage_used, root_dir, is_active, ssh_public_key,
		 reset_token, reset_token_expires, e2ee_salt, e2ee_wrapped_key, created_at, updated_at
		 FROM users WHERE reset_token = ? AND reset_token_expires > ?`,
		token, time.Now(),
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.TOTPEnabled,
		&u.StorageQuota, &u.StorageUsed, &u.RootDir, &u.IsActive, &u.SSHPublicKey,
		&u.ResetToken, &resetExpires, &u.E2EESalt, &u.E2EEWrappedKey, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.ResetTokenExpires = resetExpires
	return u, nil
}
