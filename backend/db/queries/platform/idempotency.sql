-- name: ReserveIdempotencyKey :one
-- Claims a key; an expired record with the same key is recycled.
INSERT INTO idempotency_keys (scope, key, fingerprint, expires_at)
VALUES (@scope, @key, @fingerprint, @expires_at)
ON CONFLICT (scope, key) DO UPDATE
   SET fingerprint      = EXCLUDED.fingerprint,
       expires_at       = EXCLUDED.expires_at,
       status_code      = NULL,
       response_headers = NULL,
       response_body    = NULL,
       completed_at     = NULL,
       created_at       = now()
 WHERE idempotency_keys.expires_at < now()
RETURNING created_at;

-- name: GetIdempotencyKey :one
SELECT fingerprint, status_code, response_headers, response_body, completed_at
  FROM idempotency_keys
 WHERE scope = @scope AND key = @key;

-- name: CompleteIdempotencyKey :exec
UPDATE idempotency_keys
   SET status_code      = @status_code,
       response_headers = @response_headers,
       response_body    = @response_body,
       completed_at     = now()
 WHERE scope = @scope AND key = @key;

-- name: DeleteIdempotencyKey :exec
DELETE FROM idempotency_keys WHERE scope = @scope AND key = @key;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys WHERE expires_at < now();
