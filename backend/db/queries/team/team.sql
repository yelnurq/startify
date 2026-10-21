-- name: CreateTeamRequest :one
INSERT INTO team_requests (tenant_id, author_id, title, description, roles_needed, skills)
VALUES (app_current_tenant(), @author_id, @title, @description, @roles_needed, @skills)
RETURNING *;

-- name: GetTeamRequest :one
SELECT * FROM team_requests WHERE id = @id;

-- name: ListTeamRequests :many
SELECT * FROM team_requests r
 WHERE (CASE WHEN @mine::boolean THEN r.author_id = @viewer_id ELSE r.status = 'open' END)
   AND (sqlc.narg('role')::text IS NULL OR sqlc.narg('role')::text = ANY(r.roles_needed))
   AND (sqlc.narg('skill')::text IS NULL
        OR EXISTS (SELECT 1 FROM unnest(r.skills) s WHERE lower(s) = lower(sqlc.narg('skill')::text)))
   AND (sqlc.narg('q')::text IS NULL
        OR r.title ILIKE '%' || sqlc.narg('q')::text || '%'
        OR r.description ILIKE '%' || sqlc.narg('q')::text || '%'
        OR EXISTS (SELECT 1 FROM unnest(r.skills) s WHERE s ILIKE '%' || sqlc.narg('q')::text || '%'))
   AND (sqlc.narg('before_id')::uuid IS NULL OR r.id < sqlc.narg('before_id')::uuid)
 ORDER BY r.id DESC
 LIMIT @row_limit;

-- name: UpdateTeamRequest :one
UPDATE team_requests
   SET title = coalesce(sqlc.narg('title'), title),
       description = coalesce(sqlc.narg('description'), description),
       roles_needed = coalesce(sqlc.narg('roles_needed')::text[], roles_needed),
       skills = coalesce(sqlc.narg('skills')::text[], skills),
       status = coalesce(sqlc.narg('status'), status),
       closed_at = CASE
                     WHEN sqlc.narg('status')::text = 'closed' THEN coalesce(closed_at, now())
                     WHEN sqlc.narg('status')::text = 'open' THEN NULL
                     ELSE closed_at
                   END
 WHERE id = @id
RETURNING *;

-- name: CountTeamResponses :many
SELECT request_id, count(*)::int AS responses
  FROM team_responses
 WHERE request_id = ANY(@ids::uuid[])
 GROUP BY request_id;

-- name: RespondedRequests :many
SELECT request_id FROM team_responses
 WHERE request_id = ANY(@ids::uuid[]) AND author_id = @author_id;

-- name: CreateTeamResponse :one
INSERT INTO team_responses (tenant_id, request_id, author_id, message)
VALUES (app_current_tenant(), @request_id, @author_id, @message)
RETURNING *;

-- name: ListTeamResponses :many
SELECT * FROM team_responses WHERE request_id = @request_id ORDER BY id DESC;

-- name: LockTeamRequest :one
SELECT * FROM team_requests WHERE id = @id FOR UPDATE;
