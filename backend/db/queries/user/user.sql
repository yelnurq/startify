-- name: GetUser :one
SELECT * FROM users WHERE id = @id;

-- name: GetUsersByIDs :many
-- Users are global: only members of the current university are described.
SELECT u.id, u.full_name, u.avatar_file_id, m.faculty_id
  FROM users u
  JOIN memberships m ON m.user_id = u.id AND m.tenant_id = app_current_tenant()
 WHERE u.id = ANY(@ids::uuid[]);

-- name: UpdateUser :one
UPDATE users
   SET full_name = coalesce(sqlc.narg('full_name'), full_name),
       locale = coalesce(sqlc.narg('locale'), locale)
 WHERE id = @id
RETURNING *;

-- name: SetUserAvatar :exec
UPDATE users SET avatar_file_id = @avatar_file_id WHERE id = @id;

-- Memberships ----------------------------------------------------------------

-- name: GetMembership :one
SELECT * FROM memberships WHERE tenant_id = app_current_tenant() AND user_id = @user_id;

-- name: LockMembership :one
SELECT * FROM memberships WHERE tenant_id = app_current_tenant() AND user_id = @user_id FOR UPDATE;

-- name: CreateMembership :one
INSERT INTO memberships (tenant_id, user_id, roles, status, consent_version, consent_at)
VALUES (@tenant_id, @user_id, @roles, @status, @consent_version, @consent_at)
ON CONFLICT (tenant_id, user_id) DO UPDATE
   SET consent_version = coalesce(EXCLUDED.consent_version, memberships.consent_version),
       consent_at = coalesce(EXCLUDED.consent_at, memberships.consent_at)
RETURNING *;

-- name: ActivateMembership :exec
UPDATE memberships SET status = 'active'
 WHERE tenant_id = app_current_tenant() AND user_id = @user_id AND status = 'invited';

-- name: UpdateMembershipStudy :one
UPDATE memberships
   SET faculty_id = @faculty_id,
       department_id = @department_id,
       group_id = @group_id,
       study_level = @study_level,
       course = @course,
       onboarded_at = coalesce(onboarded_at, sqlc.narg('onboarded_at'))
 WHERE tenant_id = app_current_tenant() AND user_id = @user_id
RETURNING *;

-- name: UpdateMembershipAccess :one
UPDATE memberships
   SET roles = coalesce(sqlc.narg('roles')::text[], roles),
       status = coalesce(sqlc.narg('status'), status)
 WHERE tenant_id = app_current_tenant() AND user_id = @user_id
RETURNING *;

-- name: TouchMembership :exec
UPDATE memberships SET last_active_at = now()
 WHERE tenant_id = app_current_tenant() AND user_id = @user_id
   AND (last_active_at IS NULL OR last_active_at < now() - interval '5 minutes');

-- name: ListMembers :many
SELECT u.id, u.email, u.full_name, u.avatar_file_id,
       m.roles, m.status, m.faculty_id, m.group_id, m.course, m.joined_at, m.last_active_at
  FROM memberships m
  JOIN users u ON u.id = m.user_id
 WHERE m.tenant_id = app_current_tenant()
   AND (sqlc.narg('q')::text IS NULL
        OR u.full_name ILIKE '%' || sqlc.narg('q')::text || '%'
        OR u.email::text ILIKE '%' || sqlc.narg('q')::text || '%')
   AND (sqlc.narg('role')::text IS NULL OR sqlc.narg('role')::text = ANY(m.roles))
   AND (sqlc.narg('faculty_id')::uuid IS NULL OR m.faculty_id = sqlc.narg('faculty_id')::uuid)
   AND (sqlc.narg('after_name')::text IS NULL
        OR (u.full_name, u.id) > (sqlc.narg('after_name')::text, sqlc.narg('after_id')::uuid))
 ORDER BY u.full_name, u.id
 LIMIT @row_limit;

-- name: LockAdmins :many
-- Locks the active administrators of the university in a fixed order, so
-- changes that could remove the last one are serialised.
SELECT user_id FROM memberships
 WHERE tenant_id = app_current_tenant() AND 'tenant_admin' = ANY(roles) AND status = 'active'
 ORDER BY user_id
   FOR UPDATE;

-- Profiles -------------------------------------------------------------------

-- name: GetProfile :one
SELECT * FROM user_profiles WHERE user_id = @user_id;

-- name: CreateProfile :one
INSERT INTO user_profiles (user_id, public_slug)
VALUES (@user_id, @public_slug)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: SlugTaken :one
SELECT EXISTS (SELECT 1 FROM user_profiles WHERE public_slug = @public_slug AND user_id <> @user_id);

-- name: UpdateProfile :one
UPDATE user_profiles
   SET headline = @headline,
       bio = @bio,
       skills = @skills,
       interests = @interests,
       links = @links,
       public_slug = @public_slug,
       is_public = @is_public,
       looking_for_team = @looking_for_team
 WHERE user_id = @user_id
RETURNING *;

-- name: UpdateProfileOnboarding :exec
UPDATE user_profiles
   SET skills = @skills, interests = @interests, has_idea = @has_idea, idea = @idea
 WHERE user_id = @user_id;

-- name: GetPublicProfile :one
SELECT p.*, u.full_name, u.avatar_file_id, u.id AS owner_id
  FROM user_profiles p
  JOIN users u ON u.id = p.user_id
 WHERE p.public_slug = @public_slug AND p.is_public AND u.status = 'active';

-- name: SearchPeople :many
SELECT u.id, u.full_name, u.avatar_file_id, m.roles, m.faculty_id, p.public_slug, coalesce(p.is_public, false)::boolean AS is_public
  FROM memberships m
  JOIN users u ON u.id = m.user_id
  LEFT JOIN user_profiles p ON p.user_id = u.id
 WHERE m.tenant_id = app_current_tenant() AND m.status = 'active'
   AND (sqlc.narg('q')::text IS NULL OR u.full_name ILIKE '%' || sqlc.narg('q')::text || '%')
   AND (sqlc.narg('role')::text IS NULL OR sqlc.narg('role')::text = ANY(m.roles))
 ORDER BY u.full_name, u.id
 LIMIT @row_limit;

-- name: ListRecipients :many
SELECT u.id, u.email, u.full_name, u.locale
  FROM users u
  JOIN memberships m ON m.user_id = u.id AND m.tenant_id = app_current_tenant() AND m.status = 'active'
 WHERE u.id = ANY(@ids::uuid[]) AND u.status = 'active';

-- name: CountStudents :one
SELECT count(*) FROM memberships
 WHERE tenant_id = app_current_tenant() AND 'student' = ANY(roles) AND status = 'active';
