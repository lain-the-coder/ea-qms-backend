-- name: CountChangeControlsByState :many
-- Cancelled is deliberately excluded: §9.5.2 lists five "active" states for the
-- Overview cards. A cancelled record can still appear in recent activity.
SELECT current_state, COUNT(*) AS count
FROM change_controls
WHERE current_state <> 'Cancelled'
GROUP BY current_state;

-- name: CountPendingApprovalsForUser :one
SELECT COUNT(*) FROM change_controls
WHERE assigned_approver_id = $1
  AND current_state IN ('Pending Implementation Approval', 'Pending Final Approval');

-- name: ListPendingApprovalsForUser :many
SELECT cc_id, change_title, current_state
FROM change_controls
WHERE assigned_approver_id = $1
  AND current_state IN ('Pending Implementation Approval', 'Pending Final Approval')
ORDER BY last_updated_on DESC
LIMIT $2;

-- name: CountDraftsForUser :one
SELECT COUNT(*) FROM change_controls
WHERE change_owner_id = $1 AND current_state = 'Initiated';

-- name: ListDraftsForUser :many
SELECT cc_id, change_title, current_state
FROM change_controls
WHERE change_owner_id = $1 AND current_state = 'Initiated'
ORDER BY last_updated_on DESC
LIMIT $2;

-- name: ListRecentActivity :many
SELECT cc.cc_id,
       cc.change_title,
       cc.current_state,
       cc.last_updated_on,
       updater.full_name AS last_updated_by_name
FROM change_controls cc
    JOIN users updater ON updater.id = cc.last_updated_by_id
ORDER BY cc.last_updated_on DESC
LIMIT $1;