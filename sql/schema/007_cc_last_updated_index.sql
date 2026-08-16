-- +goose Up
-- +goose StatementBegin

-- Every list query in the system sorts by last_updated_on DESC:
-- ListChangeControls, ListRecentActivity, ListPendingApprovalsForUser,
-- ListDraftsForUser. ListRecentActivity benefits most — it has no WHERE
-- clause, so without this index Postgres sorts the whole table to return
-- five rows.
CREATE INDEX idx_cc_last_updated_on
ON change_controls (last_updated_on DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cc_last_updated_on;
-- +goose StatementEnd