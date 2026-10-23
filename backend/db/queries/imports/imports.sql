-- name: CreateImportJob :one
INSERT INTO import_jobs (tenant_id, created_by, kind, file_name, columns, rows, summary)
VALUES (app_current_tenant(), @created_by, @kind, @file_name, @columns, @rows, @summary)
RETURNING *;

-- name: GetImportJob :one
SELECT * FROM import_jobs WHERE id = @id;

-- name: LockImportJob :one
SELECT * FROM import_jobs WHERE id = @id FOR UPDATE;

-- name: MarkImportCommitted :exec
UPDATE import_jobs SET status = 'committed', committed_at = now(), result = @result WHERE id = @id;

