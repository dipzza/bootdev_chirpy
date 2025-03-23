-- +goose Up
ALTER TABLE users
ADD COLUMN hashed_password VARCHAR NOT NULL
DEFAULT 'unset';

-- +goose Down
DROP TABLE users;