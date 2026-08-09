-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS recycle_bin (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id INTEGER NOT NULL,
    original_path TEXT NOT NULL,
    original_name TEXT NOT NULL,
    trash_path TEXT NOT NULL,
    file_type TEXT NOT NULL DEFAULT 'file',
    file_size INTEGER NOT NULL DEFAULT 0,
    is_dir INTEGER NOT NULL DEFAULT 0,
    deleted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expire_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_recycle_bin_user_id ON recycle_bin(user_id);
CREATE INDEX IF NOT EXISTS idx_recycle_bin_expire_at ON recycle_bin(expire_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS recycle_bin;
-- +goose StatementEnd
