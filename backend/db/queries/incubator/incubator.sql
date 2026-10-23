-- name: EnsureStep :exec
INSERT INTO project_step_data (tenant_id, project_id, step_key)
VALUES (app_current_tenant(), @project_id, @step_key)
ON CONFLICT (project_id, step_key) DO NOTHING;

-- name: GetStep :one
SELECT * FROM project_step_data WHERE project_id = @project_id AND step_key = @step_key;

-- name: LockStep :one
SELECT * FROM project_step_data WHERE project_id = @project_id AND step_key = @step_key FOR UPDATE;

-- name: ListSteps :many
SELECT * FROM project_step_data WHERE project_id = @project_id;

-- name: SaveStep :one
-- A save bumps the version; completion survives only while the minimum holds
-- (the caller passes the resulting status).
UPDATE project_step_data
   SET data = @data,
       version = version + 1,
       status = @status,
       completed_at = CASE WHEN @status::text IN ('completed', 'approved') THEN completed_at END,
       completed_by = CASE WHEN @status::text IN ('completed', 'approved') THEN completed_by END,
       updated_by = @updated_by
 WHERE project_id = @project_id AND step_key = @step_key
RETURNING *;

-- name: CompleteStep :one
UPDATE project_step_data
   SET status = 'completed', completed_at = now(), completed_by = @user_id
 WHERE project_id = @project_id AND step_key = @step_key
RETURNING *;

-- name: ApproveStep :one
UPDATE project_step_data
   SET status = 'approved', approved_by = @user_id, approval_comment = @comment,
       completed_at = coalesce(completed_at, now()), completed_by = coalesce(completed_by, @user_id)
 WHERE project_id = @project_id AND step_key = @step_key
RETURNING *;

-- name: SetStepCheck :one
UPDATE project_step_data SET ai_check = @ai_check, ai_checked_at = now()
 WHERE project_id = @project_id AND step_key = @step_key
RETURNING *;

-- name: LastRevision :one
SELECT * FROM project_revisions
 WHERE project_id = @project_id AND step_key = @step_key AND field_key = @field_key
 ORDER BY id DESC
 LIMIT 1;

-- name: InsertRevision :exec
INSERT INTO project_revisions (tenant_id, project_id, step_key, field_key, value, version, author_id, source)
VALUES (app_current_tenant(), @project_id, @step_key, @field_key, @value, @version, sqlc.narg('author_id'), @source);

-- name: FoldRevision :exec
UPDATE project_revisions SET value = @value, version = @version WHERE id = @id;

-- name: ListRevisions :many
SELECT * FROM project_revisions
 WHERE project_id = @project_id AND step_key = @step_key
   AND (sqlc.narg('field_key')::text IS NULL OR field_key = sqlc.narg('field_key')::text)
   AND (sqlc.narg('before_id')::uuid IS NULL OR id < sqlc.narg('before_id')::uuid)
 ORDER BY id DESC
 LIMIT @row_limit;

-- name: GetRevision :one
SELECT * FROM project_revisions WHERE id = @id AND project_id = @project_id AND step_key = @step_key;

-- name: ActivityDays :many
SELECT DISTINCT (updated_at AT TIME ZONE @zone::text)::date AS day
  FROM project_revisions
 WHERE project_id = @project_id AND updated_at > now() - interval '120 days'
 ORDER BY day DESC;

-- name: RecentRevisions :many
SELECT id, step_key, field_key, author_id, updated_at FROM project_revisions
 WHERE project_id = @project_id
 ORDER BY updated_at DESC
 LIMIT @row_limit;

-- name: RecentCompletions :many
SELECT step_key, status, completed_at, completed_by, approved_by FROM project_step_data
 WHERE project_id = @project_id AND completed_at IS NOT NULL
 ORDER BY completed_at DESC
 LIMIT @row_limit;

-- name: CreateComment :one
INSERT INTO project_comments (tenant_id, project_id, step_key, field_key, parent_id, author_id, body, mentions)
VALUES (app_current_tenant(), @project_id, @step_key, sqlc.narg('field_key'), sqlc.narg('parent_id'), @author_id, @body, @mentions)
RETURNING *;

-- name: GetComment :one
SELECT * FROM project_comments WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: ListComments :many
SELECT * FROM project_comments
 WHERE project_id = @project_id AND deleted_at IS NULL
   AND (sqlc.narg('step_key')::text IS NULL OR step_key = sqlc.narg('step_key')::text)
   AND (sqlc.narg('after_id')::uuid IS NULL OR id > sqlc.narg('after_id')::uuid)
 ORDER BY id
 LIMIT @row_limit;

-- name: UpdateComment :one
UPDATE project_comments
   SET body = coalesce(sqlc.narg('body'), body),
       mentions = coalesce(sqlc.narg('mentions')::uuid[], mentions),
       resolved_at = CASE
                       WHEN sqlc.narg('resolved')::boolean IS NULL THEN resolved_at
                       WHEN sqlc.narg('resolved')::boolean THEN coalesce(resolved_at, now())
                     END,
       resolved_by = CASE
                       WHEN sqlc.narg('resolved')::boolean IS NULL THEN resolved_by
                       WHEN sqlc.narg('resolved')::boolean THEN coalesce(resolved_by, @user_id)
                     END
 WHERE id = @id
RETURNING *;

-- name: DeleteComment :exec
UPDATE project_comments SET deleted_at = now() WHERE id = @id OR parent_id = @id;

-- name: OpenComments :many
SELECT step_key, count(*)::int AS open FROM project_comments
 WHERE project_id = @project_id AND deleted_at IS NULL AND resolved_at IS NULL AND parent_id IS NULL
 GROUP BY step_key;

-- name: RecentComments :many
SELECT id, step_key, field_key, author_id, created_at FROM project_comments
 WHERE project_id = @project_id AND deleted_at IS NULL
 ORDER BY created_at DESC
 LIMIT @row_limit;

-- name: CreateMaterial :one
INSERT INTO project_materials (tenant_id, project_id, kind, format, locale, requested_by)
VALUES (app_current_tenant(), @project_id, @kind, @format, @locale, @requested_by)
RETURNING *;

-- name: GetMaterial :one
SELECT * FROM project_materials WHERE id = @id AND project_id = @project_id;

-- name: ListMaterials :many
SELECT * FROM project_materials WHERE project_id = @project_id ORDER BY id DESC LIMIT 50;

-- name: SetMaterialReady :execrows
UPDATE project_materials SET status = 'ready', file_id = @file_id, error = NULL WHERE id = @id AND status = 'pending';

-- name: SetMaterialFailed :exec
UPDATE project_materials SET status = 'failed', error = @error WHERE id = @id AND status = 'pending';

-- name: FailStaleMaterials :execrows
-- Materials whose job died would stay "in progress" forever.
UPDATE project_materials SET status = 'failed', error = @error WHERE status = 'pending' AND updated_at < @before;

-- name: PruneMaterials :many
-- Keeps the newest finished materials of a project; the rest go with their files.
DELETE FROM project_materials
 WHERE id IN (
   SELECT m.id FROM project_materials m
    WHERE m.project_id = @project_id AND m.status <> 'pending'
    ORDER BY m.id DESC
   OFFSET @keep::int)
RETURNING file_id;

-- name: CountReadyMaterials :one
SELECT count(*)::int FROM project_materials WHERE project_id = @project_id AND status = 'ready';
