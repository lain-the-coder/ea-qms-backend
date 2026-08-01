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