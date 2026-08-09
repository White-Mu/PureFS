-- +goose Up
-- +goose StatementBegin

ALTER TABLE users ADD COLUMN ssh_public_key TEXT NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP COLUMN ssh_public_key;

-- +goose StatementEnd
