-- +goose Up
-- +goose StatementBegin
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CONSTRAINT ck_audit_logs_entity_type CHECK (entity_type IN (
    'ChangeControl', 'User'
    )),
    entity_id UUID NOT NULL,
    action_type TEXT NOT NULL CONSTRAINT ck_audit_logs_action_type CHECK (action_type IN (
    'Created',
    'StateChanged',
    'FieldUpdated',
    'UserAdded',
    'UserRoleChanged',
    'UserUpdated',
    'UserDeactivated',
    'SignatureCaptured',  
    'SignatureFailed'     
    )),
    field_name TEXT,
    old_value TEXT,
    new_value TEXT,
    performed_by_id UUID NOT NULL,
    performed_by_name TEXT NOT NULL,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

-- Foreign Key References
    CONSTRAINT fk_audit_logs_performed_by_id 
    FOREIGN KEY (performed_by_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT
);

-- Composite Index for audit history of one record
CREATE INDEX idx_audit_entity
ON audit_logs (entity_type, entity_id);

-- Index for Recent-activity queries
CREATE INDEX idx_audit_created_on
ON audit_logs (created_on DESC);

-- Index for "What did this user do"
CREATE INDEX idx_audit_performed_by
ON audit_logs (performed_by_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE audit_logs;
-- +goose StatementEnd
