-- name: UpsertFileAttachment :one
INSERT INTO file_attachments (
    change_control_id,
    field_name,
    file_name,
    file_size,
    content_type,
    file_data,
    uploaded_by_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (change_control_id, field_name) DO UPDATE
SET file_name      = EXCLUDED.file_name,
    file_size      = EXCLUDED.file_size,
    content_type   = EXCLUDED.content_type,
    file_data      = EXCLUDED.file_data,
    uploaded_by_id = EXCLUDED.uploaded_by_id,
    uploaded_on    = NOW()
RETURNING id, change_control_id, field_name, file_name, file_size,
          content_type, uploaded_by_id, uploaded_on;

-- name: GetFileAttachment :one
SELECT file_name, file_data, content_type, file_size
FROM file_attachments
WHERE change_control_id = $1 AND field_name = $2;

-- name: FileAttachmentExists :one
SELECT EXISTS(
    SELECT 1 FROM file_attachments
    WHERE change_control_id = $1 AND field_name = $2
);