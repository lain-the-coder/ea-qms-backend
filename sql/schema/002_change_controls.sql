-- +goose Up
-- +goose StatementBegin

CREATE SEQUENCE cc_number_seq;

CREATE TABLE change_controls (

-- Identification (BRD 1–6)
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cc_number BIGINT NOT NULL DEFAULT nextval('cc_number_seq'),
    cc_id TEXT NOT NULL GENERATED ALWAYS AS (
       'CC-' || CASE
                  WHEN cc_number < 1000 THEN LPAD(cc_number::text, 3, '0')
                  ELSE cc_number::text
              END
    ) STORED,
    current_state TEXT NOT NULL DEFAULT 'Initiated' CONSTRAINT ck_cc_current_state CHECK (current_state IN (
    'Initiated',
    'Pending Implementation Approval',
    'In Implementation',
    'Pending Final Approval',
    'Closed',
    'Cancelled'
    )),
    change_owner_id UUID NOT NULL,
    last_updated_by_id UUID NOT NULL,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

-- Change Definition (BRD 7–12)
    change_title TEXT,
    change_description TEXT,
    change_type TEXT CONSTRAINT ck_cc_change_type CHECK (change_type IN (
    'Application', 'Infrastructure', 'Database', 'Security',
    'Network', 'Hardware', 'Process', 'Other'
    )),
    change_category TEXT CONSTRAINT ck_cc_change_category CHECK (change_category IN (
    'Normal', 'Standard'
    )),
    department_function TEXT CONSTRAINT ck_cc_department_function CHECK (department_function IN (
    'IT', 'Operations', 'Security', 'QA', 'Facilities', 'Other'
    )),
    affected_systems_modules TEXT,

-- Planning (BRD 13–16)
    proposed_implementation_date DATE,
    target_closure_date DATE,
    implementation_window_start TIME,
    implementation_window_end TIME,

-- Impact & Risk (BRD 17–24) [supporting_documents not an actual field]
    reason_for_change TEXT,
    business_impact TEXT,
    expected_downtime TEXT CONSTRAINT ck_cc_expected_downtime CHECK (expected_downtime IN (
    'Yes', 'No', 'Unknown'
    )),
    requires_testing TEXT CONSTRAINT ck_cc_requires_testing CHECK (requires_testing IN (
    'Yes - Full testing',
    'Yes - Partial testing',
    'No'
    )),
    requires_training TEXT CONSTRAINT ck_cc_requires_training CHECK (requires_training IN (
    'Yes', 'No', 'Not applicable'
    )),
    risk_rationale TEXT,
    key_risks_mitigations TEXT,

-- Implementation Plan (BRD 25–28) 
    high_level_implementation_plan TEXT,
    validation_approach TEXT,
    success_criteria TEXT,
    rollback_backout_plan TEXT,

-- Implementation Details (BRD 29–34) [implementation_evidence not an actual field]
    actual_implementation_date DATE,
    post_implementation_issues TEXT CONSTRAINT ck_cc_post_impl_issues CHECK (post_implementation_issues IN (
    'None',
    'Minor issues resolved',
    'Issues requiring follow-up'
    )),
    implementation_summary TEXT,
    deviations_from_plan TEXT,
    validation_performed TEXT,

-- Approvals — Initiation (BRD 35–36)
    assigned_approver_id UUID,
    comments_for_approver TEXT,

-- Implementation Approval (BRD 37–41)
    decision TEXT CONSTRAINT ck_cc_decision CHECK (decision IN (
    'Approve', 'Reject'
    )),
    risk_level TEXT CONSTRAINT ck_cc_risk_level CHECK (risk_level IN (
    'Low', 'Medium', 'High'
    )),
    decision_comments TEXT,
    implementation_approval_by_id UUID,
    implementation_approval_on TIMESTAMPTZ,

-- Final Approval (BRD 42–45)
    final_decision TEXT CONSTRAINT ck_cc_final_decision CHECK (final_decision IN (
    'Approve', 'Reject'
    )),
    final_comments TEXT,
    final_approval_by_id UUID,
    final_approval_on TIMESTAMPTZ,

-- Status (BRD 46–48) 
    implementation_approval_status TEXT NOT NULL DEFAULT 'Not Submitted' CONSTRAINT ck_cc_impl_approval_status CHECK (implementation_approval_status IN (
    'Not Submitted', 'Pending', 'Approved', 'N/A'
    )),
    final_approval_status TEXT NOT NULL DEFAULT 'Not Submitted' CONSTRAINT ck_cc_final_approval_status CHECK (final_approval_status IN (
    'Not Submitted', 'Pending', 'Approved', 'N/A'
    )),
    actual_closure_date TIMESTAMPTZ,

-- Additional (BRD 49–50)
    comments TEXT,
    cancellation_reason TEXT,

-- Unique named constraint
    CONSTRAINT uq_change_controls_cc_id UNIQUE (cc_id),

-- Foreign Key References
    CONSTRAINT fk_change_controls_change_owner_id 
    FOREIGN KEY (change_owner_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT,

    CONSTRAINT fk_change_controls_last_updated_by_id 
    FOREIGN KEY (last_updated_by_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT,

    CONSTRAINT fk_change_controls_assigned_approver_id  
    FOREIGN KEY (assigned_approver_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT,

    CONSTRAINT fk_change_controls_implementation_approval_by_id  
    FOREIGN KEY (implementation_approval_by_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT,

    CONSTRAINT fk_change_controls_final_approval_by_id  
    FOREIGN KEY (final_approval_by_id) 
    REFERENCES users(id)
    ON DELETE RESTRICT
);

-- Index for Current State
CREATE INDEX idx_cc_current_state
ON change_controls (current_state);

-- Index for Change Owner ID
CREATE INDEX idx_cc_owner
ON change_controls (change_owner_id);

-- Index for Change Approver
CREATE INDEX idx_cc_approver
ON change_controls (assigned_approver_id);

-- Index for Change Controls Created On date
CREATE INDEX idx_cc_created_on
ON change_controls (created_on DESC);

-- Composite Index for quick search of each CC owner at the states their records are in
CREATE INDEX idx_cc_owner_state
ON change_controls (change_owner_id, current_state);

-- Composite Index for quick search of each CC Approvers at the states their records are in
CREATE INDEX idx_cc_approver_state
ON change_controls (assigned_approver_id, current_state);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE change_controls;
DROP SEQUENCE IF EXISTS cc_number_seq;
-- +goose StatementEnd
