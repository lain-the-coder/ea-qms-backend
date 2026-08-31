-- +goose Up
-- +goose StatementBegin
CREATE TABLE refresh_tokens (
    token TEXT PRIMARY KEY,
    user_id UUID NOT NULL,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

-- Foreign Key References
    CONSTRAINT fk_refresh_tokens_user_id
    FOREIGN KEY (user_id) 
    REFERENCES users(id)
    ON DELETE CASCADE
);

-- Index for Revoke-all / lookup by user
CREATE INDEX idx_refresh_tokens_user
ON refresh_tokens (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE refresh_tokens;
-- +goose StatementEnd
