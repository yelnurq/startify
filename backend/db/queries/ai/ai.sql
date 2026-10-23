-- name: GetSettings :one
SELECT * FROM tenant_ai_settings WHERE tenant_id = app_current_tenant();

-- name: UpsertSettings :one
INSERT INTO tenant_ai_settings (tenant_id, routes, local_only, daily_user_tokens, daily_tenant_tokens, mentor_mode)
VALUES (app_current_tenant(), @routes, @local_only, @daily_user_tokens, @daily_tenant_tokens, @mentor_mode)
ON CONFLICT (tenant_id) DO UPDATE
   SET routes = EXCLUDED.routes,
       local_only = EXCLUDED.local_only,
       daily_user_tokens = EXCLUDED.daily_user_tokens,
       daily_tenant_tokens = EXCLUDED.daily_tenant_tokens,
       mentor_mode = EXCLUDED.mentor_mode
RETURNING *;

-- name: InsertUsage :exec
INSERT INTO ai_usage (tenant_id, user_id, feature, provider, model, prompt_version, input_tokens, output_tokens,
                      cache_read_tokens, cache_write_tokens, cost_micros, latency_ms, status, error_code)
VALUES (app_current_tenant(), @user_id, @feature, @provider, @model, @prompt_version, @input_tokens, @output_tokens,
        @cache_read_tokens, @cache_write_tokens, @cost_micros, @latency_ms, @status, @error_code);

-- name: TokensSince :one
-- Tokens that count against the daily limits: embeddings are run by the
-- indexer for the whole university and are accounted, not limited (D-034).
SELECT coalesce(sum(input_tokens + output_tokens), 0)::bigint AS tokens
  FROM ai_usage
 WHERE tenant_id = app_current_tenant() AND created_at >= @since AND feature <> 'embedding'
   AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid);

-- name: LockLimits :exec
-- Serialises the limit check and the reservation of one university's calls.
SELECT pg_advisory_xact_lock(hashtextextended('ai-limits:' || app_current_tenant()::text, 0));

-- name: ReservedTokens :one
SELECT coalesce(sum(tokens), 0)::bigint AS tokens
  FROM ai_reservations
 WHERE tenant_id = app_current_tenant() AND expires_at > now()
   AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid);

-- name: CreateReservation :one
INSERT INTO ai_reservations (tenant_id, user_id, tokens, expires_at)
VALUES (app_current_tenant(), @user_id, @tokens, now() + make_interval(secs => @ttl_seconds::float8))
RETURNING id;

-- name: DeleteReservation :exec
DELETE FROM ai_reservations WHERE id = @id;

-- name: DeleteExpiredReservations :execrows
DELETE FROM ai_reservations WHERE expires_at < now();

-- name: UsageByFeature :many
SELECT feature,
       count(*)::int AS requests,
       coalesce(sum(input_tokens), 0)::bigint AS input_tokens,
       coalesce(sum(output_tokens), 0)::bigint AS output_tokens,
       coalesce(sum(cost_micros), 0)::bigint AS cost_micros,
       (count(*) FILTER (WHERE status <> 'ok'))::int AS failures
  FROM ai_usage
 WHERE tenant_id = app_current_tenant() AND created_at >= @since
 GROUP BY feature
 ORDER BY feature;

-- name: UsageByDay :many
SELECT (created_at AT TIME ZONE @zone::text)::date AS day,
       count(*)::int AS requests,
       coalesce(sum(input_tokens + output_tokens), 0)::bigint AS tokens,
       coalesce(sum(cost_micros), 0)::bigint AS cost_micros
  FROM ai_usage
 WHERE tenant_id = app_current_tenant() AND created_at >= @since
 GROUP BY 1
 ORDER BY 1;

-- name: UsageByModel :many
SELECT provider, model,
       count(*)::int AS requests,
       coalesce(sum(input_tokens + output_tokens), 0)::bigint AS tokens,
       coalesce(sum(cost_micros), 0)::bigint AS cost_micros
  FROM ai_usage
 WHERE tenant_id = app_current_tenant() AND created_at >= @since
 GROUP BY provider, model
 ORDER BY tokens DESC;
