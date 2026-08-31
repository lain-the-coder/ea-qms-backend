-- +goose Up
-- +goose StatementBegin
CREATE TABLE esignatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    change_control_id UUID NOT NULL, 
    signer_id UUID NOT NULL,
    signer_name TEXT NOT NULL,
    transition TEXT NOT NULL CONSTRAINT ck_esignatures_transition CHECK (transition IN (
    'T2', 'T3', 'T4', 'T5', 'T6', 'T7', 'T8'
    )),
    meaning TEXT NOT NULL CONSTRAINT ck_esignatures_meaning CHECK (meaning IN (
    'Submitted for Implementation Approval',
    'Cancelled',
    'Approved - Implementation Approval',
    'Rejected - Implementation Approval',
    'Submitted for Final Approval',
    'Approved - Final Approval',
    'Rejected - Final Approval'
    )),
    signed_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

-- Foreign Key References
    CONSTRAINT fk_esignatures_change_control_id 
    FOREIGN KEY (change_control_id) 
    REFERENCES change_controls(id)
    ON DELETE RESTRICT,

    CONSTRAINT fk_esignatures_signer_id
    FOREIGN KEY (signer_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT
);

-- Index for Signature History panel for one CC
CREATE INDEX idx_esignatures_cc
ON esignatures (change_control_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE esignatures;
-- +goose StatementEnd
