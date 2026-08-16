-- name: InsertESignature :exec
INSERT INTO esignatures
  (change_control_id, signer_id, signer_name, transition, meaning, signed_on)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListSignaturesForChangeControl :many
SELECT transition, meaning, signer_name, signed_on
FROM esignatures
WHERE change_control_id = $1
ORDER BY signed_on ASC;