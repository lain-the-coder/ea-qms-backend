-- name: ListActiveCCIDsForUser :many
SELECT cc_id FROM change_controls
WHERE (change_owner_id = sqlc.arg(user_id) OR assigned_approver_id = sqlc.arg(user_id))
    AND current_state NOT IN ('Closed', 'Cancelled')
ORDER BY cc_id;