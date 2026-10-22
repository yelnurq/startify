-- name: GetRubric :one
SELECT * FROM evaluation_rubrics WHERE tenant_id = app_current_tenant();

-- name: UpsertRubric :one
INSERT INTO evaluation_rubrics (tenant_id, weights, updated_by)
VALUES (app_current_tenant(), @weights, @updated_by)
ON CONFLICT (tenant_id) DO UPDATE SET weights = EXCLUDED.weights, updated_by = EXCLUDED.updated_by
RETURNING *;

-- name: CreateEvaluation :one
INSERT INTO evaluations (tenant_id, project_id, locale, rubric, step_versions, requested_by)
VALUES (app_current_tenant(), @project_id, @locale, @rubric, @step_versions, @requested_by)
RETURNING *;

-- name: GetEvaluation :one
SELECT * FROM evaluations WHERE id = @id AND project_id = @project_id;

-- name: LockEvaluation :one
SELECT * FROM evaluations WHERE id = @id AND project_id = @project_id FOR UPDATE;

-- name: ListEvaluations :many
SELECT * FROM evaluations WHERE project_id = @project_id ORDER BY id DESC LIMIT 100;

-- name: PreviousReady :one
SELECT * FROM evaluations
 WHERE project_id = @project_id AND status = 'ready' AND id < @before_id
 ORDER BY id DESC
 LIMIT 1;

-- name: LatestReady :many
SELECT DISTINCT ON (project_id) project_id, id, total, level, completed_at FROM evaluations
 WHERE project_id = ANY(@project_ids::uuid[]) AND status = 'ready'
 ORDER BY project_id, id DESC;

-- name: CountSince :one
SELECT count(*)::int FROM evaluations WHERE project_id = @project_id AND created_at > @since;

-- name: SetRunning :exec
UPDATE evaluations SET status = 'running' WHERE id = @id;

-- name: CompleteEvaluation :exec
UPDATE evaluations
   SET status = 'ready', ai_total = @total, total = @total, level = @level, summary = @summary,
       red_flags = @red_flags, hypotheses = @hypotheses, experiments = @experiments,
       recommendations = @recommendations, provider = @provider, model = @model,
       prompt_version = @prompt_version, error = NULL, completed_at = now()
 WHERE id = @id;

-- name: FailEvaluation :exec
UPDATE evaluations SET status = 'failed', error = @error, completed_at = now() WHERE id = @id;

-- name: SetTotal :exec
UPDATE evaluations SET total = @total, level = @level WHERE id = @id;

-- name: InsertScore :exec
INSERT INTO evaluation_criteria_scores (tenant_id, evaluation_id, criterion_key, ai_score, score, justification)
VALUES (app_current_tenant(), @evaluation_id, @criterion_key, @score, @score, @justification);

-- name: ListScores :many
SELECT * FROM evaluation_criteria_scores WHERE evaluation_id = ANY(@evaluation_ids::uuid[]);

-- name: SetScore :one
UPDATE evaluation_criteria_scores SET score = @score
 WHERE evaluation_id = @evaluation_id AND criterion_key = @criterion_key
RETURNING *;

-- name: InsertOverride :exec
INSERT INTO evaluation_overrides (tenant_id, evaluation_id, criterion_key, old_score, new_score, comment, author_id)
VALUES (app_current_tenant(), @evaluation_id, @criterion_key, @old_score, @new_score, @comment, @author_id);

-- name: ListOverrides :many
SELECT * FROM evaluation_overrides WHERE evaluation_id = @evaluation_id ORDER BY id;

-- name: CountOverrides :many
SELECT evaluation_id, count(*)::int AS n FROM evaluation_overrides
 WHERE evaluation_id = ANY(@evaluation_ids::uuid[])
 GROUP BY evaluation_id;
