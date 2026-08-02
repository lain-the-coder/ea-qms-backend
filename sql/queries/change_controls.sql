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
    final_by.full_name     AS final_approval_by_name
FROM change_controls cc
    JOIN      users owner     ON owner.id     = cc.change_owner_id
    JOIN      users updater   ON updater.id   = cc.last_updated_by_id
    LEFT JOIN users approver  ON approver.id  = cc.assigned_approver_id
    LEFT JOIN users impl_by   ON impl_by.id   = cc.implementation_approval_by_id
    LEFT JOIN users final_by  ON final_by.id  = cc.final_approval_by_id
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