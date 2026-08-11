-- +goose Up
-- +goose StatementBegin

-- E2EE support. Client-side encryption keeps file contents (and names remain
-- server-visible) encrypted at rest such that the server only ever stores and
-- serves ciphertext. Keys never leave the client except in wrapped form.
--
-- users.e2ee_salt: random per-user salt used with the client's E2EE passphrase
--   to derive the key-encryption-key (PBKDF2) that unlocks the master key.
-- users.e2ee_wrapped_key: the client-generated 32-byte master key, encrypted
--   with the passphrase-derived KEK (AES-256-GCM). The server cannot decrypt.
-- files.is_e2ee: 1 when the file bytes stored on disk are client-encrypted.
--   files.dek_ciphertext / files.kek_version are reused: kek_version=0 marks a
--   client-wrapped DEK (server-side encryption always uses kek_version >= 1).

ALTER TABLE users ADD COLUMN e2ee_salt TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN e2ee_wrapped_key TEXT NOT NULL DEFAULT '';
ALTER TABLE files ADD COLUMN is_e2ee INTEGER NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE files DROP COLUMN is_e2ee;
ALTER TABLE users DROP COLUMN e2ee_wrapped_key;
ALTER TABLE users DROP COLUMN e2ee_salt;

-- +goose StatementEnd
