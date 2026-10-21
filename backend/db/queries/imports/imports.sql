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

-- name: FindUsersByEmails :many
SELECT u.id, lower(u.email::text)::text AS email, (m.user_id IS NOT NULL)::boolean AS is_member
  FROM users u
  LEFT JOIN memberships m ON m.user_id = u.id AND m.tenant_id = app_current_tenant()
 WHERE lower(u.email::text) = ANY(@emails::text[]);

-- name: CreateInvitedUser :one
INSERT INTO users (email, full_name, locale)
VALUES (@email, @full_name, @locale)
RETURNING *;
