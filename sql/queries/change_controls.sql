-- name: ListActiveCCIDsForUser :many
SELECT cc_id FROM change_controls
WHERE (change_owner_id = sqlc.arg(user_id) OR assigned_approver_id = sqlc.arg(user_id))
    AND current_state NOT IN ('Closed', 'Cancelled')
ORDER BY cc_id;

-- name: CreateChangeControl :one
INSERT INTO change_controls (change_owner_id, last_updated_by_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetChangeControlByCcID :one
SELECT
    sqlc.embed(cc),
    owner.full_name        AS owner_name,
    updater.full_name      AS updater_name,
    approver.full_name     AS approver_name,
    impl_by.full_name      AS impl_approval_by_name,
    final_by.full_name     AS final_approval_by_name,
    ev.file_name           AS evidence_file_name,
    ev.file_size           AS evidence_file_size,
    ev.content_type        AS evidence_content_type,
    ev.uploaded_on         AS evidence_uploaded_on
FROM change_controls cc
    JOIN      users owner     ON owner.id     = cc.change_owner_id
    JOIN      users updater   ON updater.id   = cc.last_updated_by_id
    LEFT JOIN users approver  ON approver.id  = cc.assigned_approver_id
    LEFT JOIN users impl_by   ON impl_by.id   = cc.implementation_approval_by_id
    LEFT JOIN users final_by  ON final_by.id  = cc.final_approval_by_id
    LEFT JOIN file_attachments ev ON ev.change_control_id = cc.id AND ev.field_name = 'implementation_evidence'
WHERE cc.cc_id = $1;

-- name: ListChangeControls :many
SELECT
    cc.id,
    cc.cc_id,
    cc.change_title,
    cc.current_state, 
    cc.change_owner_id, 
    owner.full_name                                 AS owner_name, 
    cc.assigned_approver_id, approver.full_name     AS approver_name, 
    cc.created_on, 
    cc.last_updated_on
FROM change_controls cc
    JOIN        users owner     ON owner.id = cc.change_owner_id
    LEFT JOIN   users approver  ON approver.id  = cc.assigned_approver_id
WHERE 1=1
  -- 1. Owner filter (UUID)
  AND (sqlc.narg('owner_id')::uuid IS NULL 
       OR cc.change_owner_id = sqlc.narg('owner_id'))

  -- 2. Assigned Approver filter (UUID)
  AND (sqlc.narg('assigned_approver_id')::uuid IS NULL 
       OR cc.assigned_approver_id = sqlc.narg('assigned_approver_id'))

  -- 3. State filter (text)
  AND (sqlc.narg('state')::text IS NULL 
       OR cc.current_state = sqlc.narg('state'))

  -- 4. Created after filter (date, inclusive >=)
  AND (sqlc.narg('created_after')::date IS NULL 
       OR cc.created_on >= sqlc.narg('created_after'))

  -- 5. Created before filter (date, exclusive < with +1 day buffer)
  AND (sqlc.narg('created_before')::date IS NULL 
       OR cc.created_on < sqlc.narg('created_before') + INTERVAL '1 day')

  -- 6. Search filter (text across multiple columns)
  AND (sqlc.narg('search')::text IS NULL 
       OR cc.cc_id          ILIKE '%' || sqlc.narg('search') || '%'
       OR cc.change_title   ILIKE '%' || sqlc.narg('search') || '%'
       OR owner.full_name   ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY cc.last_updated_on DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountChangeControls :one
SELECT COUNT(*) 
FROM change_controls cc
    JOIN        users owner     ON owner.id = cc.change_owner_id
    LEFT JOIN   users approver  ON approver.id  = cc.assigned_approver_id
WHERE 1=1
  -- 1. Owner filter (UUID)
  AND (sqlc.narg('owner_id')::uuid IS NULL 
       OR cc.change_owner_id = sqlc.narg('owner_id'))

  -- 2. Assigned Approver filter (UUID)
  AND (sqlc.narg('assigned_approver_id')::uuid IS NULL 
       OR cc.assigned_approver_id = sqlc.narg('assigned_approver_id'))

  -- 3. State filter (text)
  AND (sqlc.narg('state')::text IS NULL 
       OR cc.current_state = sqlc.narg('state'))

  -- 4. Created after filter (date, inclusive >=)
  AND (sqlc.narg('created_after')::date IS NULL 
       OR cc.created_on >= sqlc.narg('created_after'))

  -- 5. Created before filter (date, exclusive < with +1 day buffer)
  AND (sqlc.narg('created_before')::date IS NULL 
       OR cc.created_on < sqlc.narg('created_before') + INTERVAL '1 day')

  -- 6. Search filter (text across multiple columns)
  AND (sqlc.narg('search')::text IS NULL 
       OR cc.cc_id          ILIKE '%' || sqlc.narg('search') || '%'
       OR cc.change_title   ILIKE '%' || sqlc.narg('search') || '%'
       OR owner.full_name   ILIKE '%' || sqlc.narg('search') || '%');

-- name: UpdateChangeControlDraft :one
UPDATE change_controls
SET
    -- system
    last_updated_by_id = sqlc.arg('last_updated_by_id'),
    last_updated_on    = NOW(),

    -- Change Definition
    change_title             = sqlc.narg('change_title'),
    change_description       = sqlc.narg('change_description'),
    change_type              = sqlc.narg('change_type'),
    change_category          = sqlc.narg('change_category'),
    department_function      = sqlc.narg('department_function'),
    affected_systems_modules = sqlc.narg('affected_systems_modules'),

    -- Planning
    proposed_implementation_date = sqlc.narg('proposed_implementation_date'),
    target_closure_date          = sqlc.narg('target_closure_date'),
    implementation_window_start  = sqlc.narg('implementation_window_start'),
    implementation_window_end    = sqlc.narg('implementation_window_end'),

    -- Impact & Risk
    reason_for_change     = sqlc.narg('reason_for_change'),
    business_impact       = sqlc.narg('business_impact'),
    expected_downtime     = sqlc.narg('expected_downtime'),
    requires_testing      = sqlc.narg('requires_testing'),
    requires_training     = sqlc.narg('requires_training'),
    risk_rationale        = sqlc.narg('risk_rationale'),
    key_risks_mitigations = sqlc.narg('key_risks_mitigations'),

     -- Implementation Plan
    high_level_implementation_plan = sqlc.narg('high_level_implementation_plan'),
    validation_approach            = sqlc.narg('validation_approach'),
    success_criteria               = sqlc.narg('success_criteria'),
    rollback_backout_plan          = sqlc.narg('rollback_backout_plan'),

    -- Approvals: Initiation
    assigned_approver_id  = sqlc.narg('assigned_approver_id'),
    comments_for_approver = sqlc.narg('comments_for_approver'),

    -- Additional
    comments = sqlc.narg('comments')

WHERE cc_id = sqlc.arg('cc_id')
RETURNING *;

-- name: GetChangeControlForUpdate :one
SELECT * FROM change_controls
WHERE cc_id = $1
FOR UPDATE;

-- name: SubmitForImplApproval :one
UPDATE change_controls
SET current_state = $2,
    implementation_approval_status = $3,
    last_updated_by_id = $4,
    last_updated_on = NOW()
WHERE cc_id = $1
RETURNING *;

-- name: CancelChangeControl :one
UPDATE change_controls
SET current_state = $2,
    implementation_approval_status = $3,
    final_approval_status = $4,
    cancellation_reason = $5,
    last_updated_by_id = $6,
    last_updated_on = NOW()
WHERE cc_id = $1
RETURNING *;

-- name: ApproveImplementation :one
UPDATE change_controls
SET current_state = $2,
    implementation_approval_status = $3,
    decision = $4,
    risk_level = $5,
    decision_comments = $6,
    implementation_approval_by_id = $7,
    implementation_approval_on = $8,
    last_updated_by_id = $9,
    last_updated_on = NOW()
WHERE cc_id = $1
RETURNING *;

-- name: RejectImplementation :one
UPDATE change_controls
SET current_state = $2,
    implementation_approval_status = $3,
    decision = $4,
    risk_level = $5,
    decision_comments = $6,
    last_updated_by_id = $7,
    last_updated_on = NOW()
WHERE cc_id = $1
RETURNING *;

-- name: TouchChangeControl :exec
UPDATE change_controls
SET last_updated_by_id = $2,
    last_updated_on = NOW()
WHERE cc_id = $1;

-- name: GetChangeControlIDByCcID :one
SELECT id FROM change_controls
WHERE cc_id = $1;

-- name: UpdateImplementationDetails :one
UPDATE change_controls
SET
    -- Implementation Details (BRD 29–33)
    actual_implementation_date = sqlc.narg('actual_implementation_date'),
    post_implementation_issues = sqlc.narg('post_implementation_issues'),
    implementation_summary     = sqlc.narg('implementation_summary'),
    deviations_from_plan       = sqlc.narg('deviations_from_plan'),
    validation_performed       = sqlc.narg('validation_performed'),

    -- system
    last_updated_by_id = sqlc.arg('last_updated_by_id'),
    last_updated_on    = NOW()
WHERE cc_id = sqlc.arg('cc_id')
RETURNING *;

-- name: SubmitForFinalApproval :one
UPDATE change_controls
SET current_state = $2,
    final_approval_status = $3,
    last_updated_by_id = $4,
    last_updated_on = NOW()
WHERE cc_id = $1
RETURNING *;