-- name: InsertAuditLog :exec
INSERT INTO audit_logs (
    entity_type, entity_id, action_type,
    field_name, old_value, new_value,
    performed_by_id, performed_by_name, created_on
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9);