-- name: ListDocumentSignatures :many
SELECT doc_type, doc_id, role, user_id, name, signed_at
FROM document_signatures
WHERE doc_type = $1 AND doc_id = $2
ORDER BY signed_at;

-- name: InsertDocumentSignature :execrows
INSERT INTO document_signatures (doc_type, doc_id, role, user_id, name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (doc_type, doc_id, role) DO NOTHING;

-- name: DeleteDocumentSignature :exec
DELETE FROM document_signatures WHERE doc_type = $1 AND doc_id = $2 AND role = $3;

-- name: DeleteDocumentSignatures :exec
DELETE FROM document_signatures WHERE doc_type = $1 AND doc_id = $2;
