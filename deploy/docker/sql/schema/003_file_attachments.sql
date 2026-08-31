-- +goose Up
-- +goose StatementBegin
CREATE TABLE file_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    change_control_id UUID NOT NULL,
    field_name TEXT NOT NULL CONSTRAINT ck_file_attachments_field_name CHECK (field_name IN (
    'supporting_documents', 'implementation_evidence'
    )), 
    file_name TEXT NOT NULL,
    file_data BYTEA NOT NULL,
    content_type TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    uploaded_by_id UUID NOT NULL,
    uploaded_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

-- Unique named constraint
    CONSTRAINT uq_file_attachments_cc_field UNIQUE (change_control_id, field_name),

-- Foreign Key References
    CONSTRAINT fk_file_attachments_change_control_id 
    FOREIGN KEY (change_control_id) 
    REFERENCES change_controls(id)
    ON DELETE CASCADE,

    CONSTRAINT fk_file_attachments_uploaded_by_id 
    FOREIGN KEY (uploaded_by_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE file_attachments;
-- +goose StatementEnd
