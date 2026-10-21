-- name: CreateFile :one
INSERT INTO files (id, tenant_id, owner_id, purpose, visibility, object_key, file_name,
                   content_type, size_bytes, sha256, scan_status)
VALUES (@id, app_current_tenant(), @owner_id, @purpose, @visibility, @object_key, @file_name,
        @content_type, @size_bytes, @sha256, @scan_status)
RETURNING *;

-- name: GetFile :one
SELECT * FROM files WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteFile :exec
UPDATE files SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListPurgeableFiles :many
SELECT id, object_key FROM files
WHERE deleted_at < now() - interval '1 day'
ORDER BY deleted_at
LIMIT @row_limit;

-- name: PurgeFile :exec
DELETE FROM files WHERE id = @id;
