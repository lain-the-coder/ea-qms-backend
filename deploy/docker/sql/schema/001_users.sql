-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name TEXT NOT NULL,
    email TEXT NOT NULL,
    hashed_password TEXT NOT NULL,
    role TEXT NOT NULL CONSTRAINT ck_users_role CHECK (role IN ('CC Owner', 'Approver', 'Viewer', 'Admin')),
    is_active BOOLEAN NOT NULL DEFAULT true, 
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Case-insensitive unique index for login
CREATE UNIQUE INDEX uq_users_email 
ON users (LOWER(email));

-- Optimized composite index for your active approver dropdown
CREATE INDEX idx_users_role_active 
ON users (role, is_active);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
