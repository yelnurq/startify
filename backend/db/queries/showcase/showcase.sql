-- Showcase pages and contact requests. A page is live inside its university
-- once published_at is set, and outside it only while it is published and
-- public. Listing scopes:
--   national — public published pages of every university;
--   member   — live pages of the current university (signed-in members);
--   public   — public published pages of the current university.

-- name: GetEntry :one
SELECT * FROM showcase_entries
 WHERE project_id = @project_id AND tenant_id = app_current_tenant();

-- name: LockEntry :one
SELECT * FROM showcase_entries
 WHERE project_id = @project_id AND tenant_id = app_current_tenant()
 FOR UPDATE;

-- name: SlugOwner :one
SELECT project_id FROM showcase_entries
 WHERE tenant_id = app_current_tenant() AND slug = @slug;

-- name: UpsertEntry :one
INSERT INTO showcase_entries (
  project_id, tenant_id, slug, visibility, title, tagline, industry, stage, faculty_id, faculty, year,
  video_url, sections, team, needs, tags, show_score, score, level, contact_enabled, search)
VALUES (
  @project_id, app_current_tenant(), @slug, @visibility, @title, @tagline, @industry, @stage, sqlc.narg(faculty_id),
  sqlc.narg(faculty), @year, sqlc.narg(video_url), @sections, @team, @needs, @tags, @show_score, sqlc.narg(score),
  sqlc.narg(level), @contact_enabled, to_tsvector('simple', @search_text::text))
ON CONFLICT (project_id) DO UPDATE
   SET slug = EXCLUDED.slug,
       visibility = EXCLUDED.visibility,
       title = EXCLUDED.title,
       tagline = EXCLUDED.tagline,
       industry = EXCLUDED.industry,
       stage = EXCLUDED.stage,
       faculty_id = EXCLUDED.faculty_id,
       faculty = EXCLUDED.faculty,
       video_url = EXCLUDED.video_url,
       sections = EXCLUDED.sections,
       team = EXCLUDED.team,
       needs = EXCLUDED.needs,
       tags = EXCLUDED.tags,
       show_score = EXCLUDED.show_score,
       score = EXCLUDED.score,
       level = EXCLUDED.level,
       contact_enabled = EXCLUDED.contact_enabled,
       search = EXCLUDED.search,
       version = showcase_entries.version + 1
RETURNING *;

-- name: SetStatus :one
UPDATE showcase_entries
   SET status = @status,
       submitted_at = sqlc.narg(submitted_at),
       published_at = sqlc.narg(published_at),
       review_comment = sqlc.narg(review_comment),
       reviewed_by = sqlc.narg(reviewed_by)
 WHERE project_id = @project_id
RETURNING *;

-- name: SetCover :one
UPDATE showcase_entries
   SET cover_file_id = sqlc.narg(cover_file_id), version = version + 1
 WHERE project_id = @project_id
RETURNING *;

-- name: SetImage :exec
UPDATE showcase_entries SET og_file_id = @og_file_id, og_hash = @og_hash WHERE project_id = @project_id;

-- name: GetLiveBySlug :one
SELECT * FROM showcase_entries
 WHERE tenant_id = app_current_tenant() AND slug = @slug AND published_at IS NOT NULL;

-- name: ListLive :many
SELECT e.project_id, e.tenant_id, e.slug, e.title, e.tagline, e.industry, e.stage, e.faculty, e.year,
       e.cover_file_id, e.needs, e.tags, e.show_score, e.score, e.level, e.published_at
  FROM showcase_entries e
 WHERE CASE @scope::text
         WHEN 'national' THEN e.status = 'published' AND e.visibility = 'public'
         WHEN 'member' THEN e.tenant_id = app_current_tenant() AND e.published_at IS NOT NULL
         ELSE e.tenant_id = app_current_tenant() AND e.status = 'published' AND e.visibility = 'public'
       END
   AND (sqlc.narg(query)::text IS NULL OR e.search @@ to_tsquery('simple', sqlc.narg(query)::text))
   AND (sqlc.narg(industry)::text IS NULL OR e.industry = sqlc.narg(industry)::text)
   AND (sqlc.narg(stage)::text IS NULL OR e.stage = sqlc.narg(stage)::text)
   AND (sqlc.narg(faculty_id)::uuid IS NULL OR e.faculty_id = sqlc.narg(faculty_id)::uuid)
   AND (sqlc.narg(year)::integer IS NULL OR e.year = sqlc.narg(year)::integer)
   AND (sqlc.narg(need)::text IS NULL OR sqlc.narg(need)::text = ANY(e.needs))
   AND (sqlc.narg(tag)::text IS NULL OR sqlc.narg(tag)::text = ANY(e.tags))
   AND (sqlc.narg(before_at)::timestamptz IS NULL
        OR (e.published_at, e.project_id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
 ORDER BY e.published_at DESC, e.project_id DESC
 LIMIT @row_limit;

-- name: ListFacetRows :many
SELECT e.industry, e.stage, e.needs, e.tags, e.year, e.faculty_id, e.faculty
  FROM showcase_entries e
 WHERE CASE @scope::text
         WHEN 'national' THEN e.status = 'published' AND e.visibility = 'public'
         WHEN 'member' THEN e.tenant_id = app_current_tenant() AND e.published_at IS NOT NULL
         ELSE e.tenant_id = app_current_tenant() AND e.status = 'published' AND e.visibility = 'public'
       END
 LIMIT 10000;

-- name: ListFeatured :many
SELECT e.project_id, e.tenant_id, e.slug, e.title, e.tagline, e.industry, e.stage, e.faculty, e.year,
       e.cover_file_id, e.needs, e.tags, e.show_score, e.score, e.level, e.published_at
  FROM showcase_entries e
 WHERE CASE @scope::text
         WHEN 'member' THEN e.tenant_id = app_current_tenant() AND e.published_at IS NOT NULL
         ELSE e.tenant_id = app_current_tenant() AND e.status = 'published' AND e.visibility = 'public'
       END
 ORDER BY CASE WHEN e.show_score THEN e.score END DESC NULLS LAST, e.published_at DESC
 LIMIT @row_limit;

-- name: CountLive :one
-- Pages of the current university that the scope can open.
SELECT count(*) FROM showcase_entries e
 WHERE CASE @scope::text
         WHEN 'member' THEN e.tenant_id = app_current_tenant() AND e.published_at IS NOT NULL
         ELSE e.tenant_id = app_current_tenant() AND e.status = 'published' AND e.visibility = 'public'
       END;

-- name: ListForReview :many
SELECT * FROM showcase_entries
 WHERE tenant_id = app_current_tenant() AND status = @status
 ORDER BY submitted_at NULLS LAST, project_id
 LIMIT 200;

-- name: ListSitemap :many
SELECT slug, updated_at FROM showcase_entries
 WHERE tenant_id = app_current_tenant() AND status = 'published' AND visibility = 'public'
 ORDER BY published_at DESC
 LIMIT 5000;

-- Contact requests --------------------------------------------------------------

-- name: CreateContact :one
INSERT INTO contact_requests (tenant_id, project_id, name, email, organization, kind, message, locale, status)
VALUES (app_current_tenant(), @project_id, @name, @email, sqlc.narg(organization), @kind, @message, @locale, @status)
RETURNING *;

-- name: LockContact :one
SELECT * FROM contact_requests WHERE id = @id FOR UPDATE;

-- name: ModerateContact :one
UPDATE contact_requests
   SET status = @status, moderated_by = @moderated_by, moderated_at = now()
 WHERE id = @id
RETURNING *;

-- name: ListContacts :many
SELECT c.*, coalesce(e.title, '')::text AS project_title
  FROM contact_requests c
  LEFT JOIN showcase_entries e ON e.project_id = c.project_id
 WHERE c.tenant_id = app_current_tenant()
   AND (sqlc.narg(status)::text IS NULL OR c.status = sqlc.narg(status)::text)
 ORDER BY c.id DESC
 LIMIT 200;

-- name: ListProjectContacts :many
SELECT c.*, coalesce(e.title, '')::text AS project_title
  FROM contact_requests c
  LEFT JOIN showcase_entries e ON e.project_id = c.project_id
 WHERE c.project_id = @project_id
   AND (NOT @forwarded_only::boolean OR c.status = 'forwarded')
 ORDER BY c.id DESC
 LIMIT 200;
