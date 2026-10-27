-- name: CreateProject :one
INSERT INTO projects (tenant_id, title, tagline, industry, faculty_id, current_step, ip_policy_version, ip_accepted_at, created_by)
VALUES (app_current_tenant(), @title, @tagline, @industry, sqlc.narg('faculty_id'), @current_step,
        @ip_policy_version, sqlc.narg('ip_accepted_at'), @created_by)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = @id AND deleted_at IS NULL;

-- name: LockProject :one
SELECT * FROM projects WHERE id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: LockProjectRow :one
-- Serialises writers of a project's parts. NO KEY UPDATE does not wait for
-- the KEY SHARE locks that inserting a step, a member or a page takes on the
-- project (foreign keys), so concurrent writers queue instead of deadlocking.
SELECT id FROM projects WHERE id = @id AND deleted_at IS NULL FOR NO KEY UPDATE;

-- name: UpdateProject :one
UPDATE projects
   SET title = coalesce(sqlc.narg('title'), title),
       tagline = coalesce(sqlc.narg('tagline'), tagline),
       industry = coalesce(sqlc.narg('industry'), industry),
       hide_sensitive = coalesce(sqlc.narg('hide_sensitive'), hide_sensitive)
 WHERE id = @id
RETURNING *;

-- name: ArchiveProject :exec
UPDATE projects SET archived_at = coalesce(archived_at, now()) WHERE id = @id;

-- name: SetProgress :exec
UPDATE projects SET completed_steps = @completed_steps, current_step = @current_step WHERE id = @id;

-- name: ListMyProjects :many
-- The explicit tenant conditions let the indexes find the university's rows;
-- row-level security alone (… OR app_is_system()) cannot use an index.
SELECT p.* FROM projects p
  JOIN project_members m ON m.project_id = p.id AND m.tenant_id = app_current_tenant() AND m.user_id = @user_id
 WHERE p.tenant_id = app_current_tenant() AND p.deleted_at IS NULL
   AND (sqlc.narg('q')::text IS NULL OR p.title ILIKE '%' || sqlc.narg('q')::text || '%')
   AND (sqlc.narg('before_updated')::timestamptz IS NULL
        OR (p.updated_at, p.id) < (sqlc.narg('before_updated')::timestamptz, sqlc.narg('before_id')::uuid))
 ORDER BY p.updated_at DESC, p.id DESC
 LIMIT @row_limit;

-- name: ListAllProjects :many
SELECT p.* FROM projects p
 WHERE p.tenant_id = app_current_tenant() AND p.deleted_at IS NULL
   AND (sqlc.narg('q')::text IS NULL OR p.title ILIKE '%' || sqlc.narg('q')::text || '%')
   AND (sqlc.narg('before_updated')::timestamptz IS NULL
        OR (p.updated_at, p.id) < (sqlc.narg('before_updated')::timestamptz, sqlc.narg('before_id')::uuid))
 ORDER BY p.updated_at DESC, p.id DESC
 LIMIT @row_limit;

-- name: ListMembers :many
SELECT * FROM project_members
 WHERE project_id = ANY(@project_ids::uuid[])
 ORDER BY project_id, CASE role WHEN 'owner' THEN 0 WHEN 'member' THEN 1 ELSE 2 END, created_at;

-- name: GetMember :one
SELECT * FROM project_members WHERE project_id = @project_id AND user_id = @user_id;

-- name: AddMember :one
INSERT INTO project_members (tenant_id, project_id, user_id, role, title)
VALUES (app_current_tenant(), @project_id, @user_id, @role, @title)
RETURNING *;

-- name: UpdateMemberTitle :one
UPDATE project_members SET title = @title WHERE project_id = @project_id AND user_id = @user_id
RETURNING *;

-- name: SetMemberRole :one
UPDATE project_members SET role = @role WHERE project_id = @project_id AND user_id = @user_id
RETURNING *;

-- name: RemoveMember :exec
DELETE FROM project_members WHERE project_id = @project_id AND user_id = @user_id;

-- name: CountProjects :one
SELECT count(*) FROM projects WHERE tenant_id = app_current_tenant() AND deleted_at IS NULL;

-- name: SetVisibility :exec
UPDATE projects SET visibility = @visibility WHERE id = @id;
