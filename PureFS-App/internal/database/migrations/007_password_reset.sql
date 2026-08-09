-- +goose Up
-- +goose StatementBegin

ALTER TABLE users ADD COLUMN reset_token TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN reset_token_expires DATETIME;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP COLUMN reset_token;
ALTER TABLE users DROP COLUMN reset_token_expires;

-- +goose StatementEnd
