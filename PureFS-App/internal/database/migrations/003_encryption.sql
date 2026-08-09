-- +goose Up
-- +goose StatementBegin

ALTER TABLE files ADD COLUMN dek_ciphertext TEXT NOT NULL DEFAULT '';
ALTER TABLE files ADD COLUMN kek_version INTEGER NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE files DROP COLUMN dek_ciphertext;
ALTER TABLE files DROP COLUMN kek_version;

-- +goose StatementEnd
