-- +goose Up
-- Platform foundation: extensions, identifier generation, row-level security
-- helpers and the restricted application role.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS vector;

-- Time-ordered UUIDv7 (RFC 9562) for primary keys created inside the database.
-- The application generates the same format with google/uuid.
-- +goose StatementBegin
CREATE FUNCTION uuid_generate_v7() RETURNS uuid
LANGUAGE plpgsql VOLATILE AS $$
DECLARE
  bytes bytea;
BEGIN
  bytes := substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3)
           || gen_random_bytes(10);
  bytes := set_byte(bytes, 6, (b'0111' || get_byte(bytes, 6)::bit(4))::bit(8)::int);
  bytes := set_byte(bytes, 8, (b'10' || get_byte(bytes, 8)::bit(6))::bit(8)::int);
  RETURN encode(bytes, 'hex')::uuid;
END
$$;
-- +goose StatementEnd

-- Tenant of the current transaction, set by the application with
-- set_config('app.tenant_id', ..., true). NULL when unset: policies then match nothing.
CREATE FUNCTION app_current_tenant() RETURNS uuid
LANGUAGE sql STABLE AS $$
  SELECT nullif(current_setting('app.tenant_id', true), '')::uuid
$$;

-- Cross-tenant platform operations (super admin, national showcase, maintenance).
CREATE FUNCTION app_is_system() RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.system', true), '') = 'on'
$$;

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END
$$;
-- +goose StatementEnd

-- The API and worker connect as qadam_app: not a table owner and without
-- BYPASSRLS, so row-level security always applies to it. Deployments create the
-- role with LOGIN and a password before migrating; this is a fallback.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'qadam_app') THEN
    CREATE ROLE qadam_app NOLOGIN;
  END IF;
END
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO qadam_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO qadam_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO qadam_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO qadam_app;
GRANT EXECUTE ON FUNCTION uuid_generate_v7(), app_current_tenant(), app_is_system() TO qadam_app;

-- Stored outcomes of requests sent with an Idempotency-Key header.
-- scope combines tenant and user, so the table needs no row-level security.
CREATE TABLE idempotency_keys (
  scope            text        NOT NULL,
  key              text        NOT NULL,
  fingerprint      text        NOT NULL,
  status_code      integer,
  response_headers jsonb,
  response_body    bytea,
  completed_at     timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  expires_at       timestamptz NOT NULL,
  PRIMARY KEY (scope, key)
);
CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);

-- +goose Down
DROP TABLE idempotency_keys;
DROP FUNCTION set_updated_at();
DROP FUNCTION app_is_system();
DROP FUNCTION app_current_tenant();
DROP FUNCTION uuid_generate_v7();
