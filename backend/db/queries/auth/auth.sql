-- Users and identities -------------------------------------------------------

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = @email;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: CreateUser :one
INSERT INTO users (email, full_name, locale, email_verified_at)
VALUES (@email, @full_name, @locale, @email_verified_at)
RETURNING *;

-- name: LockUser :one
SELECT * FROM users WHERE id = @id FOR UPDATE;

-- name: SetRegistrant :exec
-- A new registration of an address that was never verified names the account anew.
UPDATE users SET full_name = @full_name, locale = @locale WHERE id = @id AND email_verified_at IS NULL;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = coalesce(email_verified_at, now()) WHERE id = @id;

-- name: GetPasswordHash :one
SELECT password_hash::text FROM user_identities
 WHERE user_id = @user_id AND provider = 'password';

-- name: UpsertPasswordIdentity :exec
INSERT INTO user_identities (user_id, provider, subject, password_hash)
VALUES (@user_id, 'password', @subject, @password_hash)
ON CONFLICT (provider, subject) DO UPDATE SET password_hash = EXCLUDED.password_hash;

-- name: DeletePasswordIdentity :exec
DELETE FROM user_identities WHERE user_id = @user_id AND provider = 'password';

-- name: TouchIdentity :exec
UPDATE user_identities SET last_used_at = now()
 WHERE user_id = @user_id AND provider = @provider;

-- name: GetIdentityUser :one
SELECT u.*
  FROM user_identities i
  JOIN users u ON u.id = i.user_id
 WHERE i.provider = @provider AND i.subject = @subject;

-- name: CreateIdentity :exec
INSERT INTO user_identities (user_id, provider, subject)
VALUES (@user_id, @provider, @subject)
ON CONFLICT (provider, subject) DO NOTHING;

-- Sessions -------------------------------------------------------------------

-- name: CreateSession :one
INSERT INTO sessions (id, tenant_id, user_id, refresh_hash, user_agent, ip, expires_at)
VALUES (@id, @tenant_id, @user_id, @refresh_hash, @user_agent, @ip, @expires_at)
RETURNING *;

-- name: GetSessionByRefreshHash :one
SELECT * FROM sessions WHERE refresh_hash = @refresh_hash;

-- name: GetSessionByPreviousHash :one
SELECT * FROM sessions WHERE previous_refresh_hash = @previous_refresh_hash;

-- name: RotateSession :execrows
UPDATE sessions
   SET previous_refresh_hash = refresh_hash,
       refresh_hash = @new_hash,
       rotated_at = now(),
       last_used_at = now(),
       ip = coalesce(@ip, ip),
       user_agent = coalesce(@user_agent, user_agent)
 WHERE id = @id AND refresh_hash = @old_hash AND revoked_at IS NULL;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = now(), revoked_reason = @reason
 WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeUserSessions :many
UPDATE sessions SET revoked_at = now(), revoked_reason = @reason
 WHERE user_id = @user_id AND revoked_at IS NULL
   AND (sqlc.narg('except_id')::uuid IS NULL OR id <> sqlc.narg('except_id')::uuid)
RETURNING id;

-- name: ListActiveSessions :many
SELECT * FROM sessions
 WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > now()
 ORDER BY last_used_at DESC;

-- name: DeleteStaleSessions :execrows
DELETE FROM sessions
 WHERE expires_at < now() - interval '7 days'
    OR revoked_at < now() - interval '30 days';

-- One-time tokens ------------------------------------------------------------

-- name: CreateAuthToken :exec
INSERT INTO auth_tokens (id, tenant_id, user_id, purpose, token_hash, expires_at)
VALUES (@id, @tenant_id, @user_id, @purpose, @token_hash, @expires_at);

-- name: GetAuthToken :one
SELECT * FROM auth_tokens
 WHERE token_hash = @token_hash AND purpose = ANY(@purposes::text[])
   AND consumed_at IS NULL AND expires_at > now();

-- name: ListActiveLoginCodes :many
SELECT * FROM auth_tokens
 WHERE user_id = @user_id AND purpose = 'login_code'
   AND consumed_at IS NULL AND expires_at > now()
 ORDER BY created_at DESC;

-- name: ConsumeAuthToken :execrows
UPDATE auth_tokens SET consumed_at = now() WHERE id = @id AND consumed_at IS NULL;

-- name: CountLoginCodeAttempt :one
UPDATE auth_tokens SET attempts = attempts + 1 WHERE id = @id RETURNING attempts;

-- name: ConsumeUserTokens :exec
UPDATE auth_tokens SET consumed_at = now()
 WHERE user_id = @user_id AND purpose = ANY(@purposes::text[]) AND consumed_at IS NULL;

-- name: DeleteStaleAuthTokens :execrows
DELETE FROM auth_tokens WHERE expires_at < now() - interval '1 day';
