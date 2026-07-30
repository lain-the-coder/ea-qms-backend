-- name: ListActiveCCIDsForUser :many
SELECT cc_id FROM change_controls
WHERE (change_owner_id = sqlc.arg(user_id) OR assigned_approver_id = sqlc.arg(user_id))
    AND current_state NOT IN ('Closed', 'Cancelled')
ORDER BY cc_id;

-- name: CreateChangeControl :one
INSERT INTO change_controls (change_owner_id, last_updated_by_id)
VALUES ($1, $2)
RETURNING *;