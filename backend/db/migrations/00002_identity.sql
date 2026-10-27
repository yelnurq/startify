-- +goose Up
-- Universities (tenants), users and their memberships, sessions, one-time
-- tokens, university structure, profiles, files, audit trail, imports and the
-- teammates board.
--
-- Every table with tenant_id is protected by row-level security (D-006).
-- tenants and tenant_domains are platform registries: they are read before a
-- tenant is known (host resolution) and hold no personal data. users,
-- user_identities and user_profiles are global (D-007): the service layer
-- exposes them only through memberships of the current tenant.

-- +goose StatementBegin
CREATE FUNCTION app_enable_tenant_rls(tbl regclass) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', tbl);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', tbl);
  EXECUTE format(
    'CREATE POLICY tenant_isolation ON %s
       USING (tenant_id = app_current_tenant() OR app_is_system())
       WITH CHECK (tenant_id = app_current_tenant() OR app_is_system())', tbl);
END
$$;
-- +goose StatementEnd

CREATE TABLE tenants (
  id             uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  slug           citext NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$'),
  name           jsonb NOT NULL,
  short_name     jsonb NOT NULL,
  default_locale text NOT NULL DEFAULT 'kk' CHECK (default_locale IN ('kk', 'ru', 'en')),
  primary_color  text NOT NULL DEFAULT '#00A3C4' CHECK (primary_color ~ '^#[0-9A-Fa-f]{6}$'),
  logo_file_id   uuid,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER tenants_updated_at BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE tenant_domains (
  domain     citext PRIMARY KEY,
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tenant_domains_primary_idx ON tenant_domains (tenant_id) WHERE is_primary;

CREATE TABLE tenant_settings (
  tenant_id          uuid PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
  self_signup        boolean NOT NULL DEFAULT true,
  email_domains      text[] NOT NULL DEFAULT '{}',
  consent_version    text NOT NULL,
  consent_text       jsonb NOT NULL,
  ip_policy_text     jsonb NOT NULL DEFAULT '{}',
  contact_moderation boolean NOT NULL DEFAULT true,
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER tenant_settings_updated_at BEFORE UPDATE ON tenant_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('tenant_settings');

CREATE TABLE tenant_oidc_providers (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id         uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  name              jsonb NOT NULL,
  issuer            text NOT NULL,
  client_id         text NOT NULL,
  client_secret_enc bytea NOT NULL,
  scopes            text[] NOT NULL DEFAULT '{openid,email,profile}',
  enabled           boolean NOT NULL DEFAULT true,
  auto_provision    boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER tenant_oidc_providers_updated_at BEFORE UPDATE ON tenant_oidc_providers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('tenant_oidc_providers');

CREATE TABLE users (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  email             citext NOT NULL UNIQUE,
  email_verified_at timestamptz,
  full_name         text NOT NULL,
  locale            text NOT NULL DEFAULT 'kk' CHECK (locale IN ('kk', 'ru', 'en')),
  avatar_file_id    uuid,
  is_super_admin    boolean NOT NULL DEFAULT false,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_identities (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  -- 'password' or 'oidc:<provider id>'
  provider      text NOT NULL,
  subject       text NOT NULL,
  password_hash text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_used_at  timestamptz,
  UNIQUE (provider, subject),
  CHECK ((provider = 'password') = (password_hash IS NOT NULL))
);
CREATE UNIQUE INDEX user_identities_one_password_idx ON user_identities (user_id) WHERE provider = 'password';

CREATE TABLE user_profiles (
  user_id          uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  public_slug      citext NOT NULL UNIQUE,
  headline         text,
  bio              text,
  skills           text[] NOT NULL DEFAULT '{}',
  interests        text[] NOT NULL DEFAULT '{}',
  links            jsonb NOT NULL DEFAULT '{}',
  is_public        boolean NOT NULL DEFAULT false,
  looking_for_team boolean NOT NULL DEFAULT false,
  has_idea         boolean NOT NULL DEFAULT false,
  idea             text,
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER user_profiles_updated_at BEFORE UPDATE ON user_profiles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX user_profiles_skills_idx ON user_profiles USING gin (skills);

CREATE TABLE faculties (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  code       text NOT NULL CHECK (length(code) BETWEEN 1 AND 32),
  name       jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, code)
);
CREATE TRIGGER faculties_updated_at BEFORE UPDATE ON faculties FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('faculties');

CREATE TABLE departments (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  faculty_id uuid NOT NULL REFERENCES faculties (id) ON DELETE CASCADE,
  code       text NOT NULL CHECK (length(code) BETWEEN 1 AND 32),
  name       jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, code)
);
CREATE TRIGGER departments_updated_at BEFORE UPDATE ON departments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX departments_faculty_idx ON departments (faculty_id);
SELECT app_enable_tenant_rls('departments');

CREATE TABLE study_groups (
  id             uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id      uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  faculty_id     uuid NOT NULL REFERENCES faculties (id) ON DELETE CASCADE,
  department_id  uuid REFERENCES departments (id) ON DELETE SET NULL,
  code           text NOT NULL CHECK (length(code) BETWEEN 1 AND 40),
  study_level    text NOT NULL DEFAULT 'bachelor' CHECK (study_level IN ('bachelor', 'master', 'doctorate')),
  admission_year smallint CHECK (admission_year BETWEEN 1990 AND 2100),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, code)
);
CREATE TRIGGER study_groups_updated_at BEFORE UPDATE ON study_groups FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX study_groups_code_prefix_idx ON study_groups (tenant_id, lower(code) text_pattern_ops);
SELECT app_enable_tenant_rls('study_groups');

CREATE TABLE memberships (
  tenant_id       uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  roles           text[] NOT NULL DEFAULT '{student}' CHECK (
                    cardinality(roles) > 0 AND roles <@ ARRAY[
                      'student', 'supervisor', 'mentor', 'jury', 'innovation_manager',
                      'tenant_admin', 'leadership', 'partner']::text[]),
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'suspended')),
  faculty_id      uuid REFERENCES faculties (id) ON DELETE SET NULL,
  department_id   uuid REFERENCES departments (id) ON DELETE SET NULL,
  group_id        uuid REFERENCES study_groups (id) ON DELETE SET NULL,
  study_level     text CHECK (study_level IN ('bachelor', 'master', 'doctorate')),
  course          smallint CHECK (course BETWEEN 1 AND 7),
  onboarded_at    timestamptz,
  consent_version text,
  consent_at      timestamptz,
  joined_at       timestamptz NOT NULL DEFAULT now(),
  last_active_at  timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, user_id)
);
CREATE TRIGGER memberships_updated_at BEFORE UPDATE ON memberships FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX memberships_user_idx ON memberships (user_id);
CREATE INDEX memberships_roles_idx ON memberships USING gin (roles);
CREATE INDEX memberships_faculty_idx ON memberships (tenant_id, faculty_id);
SELECT app_enable_tenant_rls('memberships');

-- One row per sign-in. The refresh token rotates on every refresh; presenting
-- the previous one again means it leaked, and the session is revoked.
CREATE TABLE sessions (
  id                    uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id             uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  user_id               uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  refresh_hash          bytea NOT NULL UNIQUE,
  previous_refresh_hash bytea,
  rotated_at            timestamptz,
  user_agent            text,
  ip                    inet,
  created_at            timestamptz NOT NULL DEFAULT now(),
  last_used_at          timestamptz NOT NULL DEFAULT now(),
  expires_at            timestamptz NOT NULL,
  revoked_at            timestamptz,
  revoked_reason        text
);
CREATE INDEX sessions_user_active_idx ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_previous_hash_idx ON sessions (previous_refresh_hash) WHERE previous_refresh_hash IS NOT NULL;
CREATE INDEX sessions_expires_idx ON sessions (expires_at);
SELECT app_enable_tenant_rls('sessions');

-- E-mail verification, password reset and invitation links, and sign-in codes.
-- Only keyed hashes are stored.
CREATE TABLE auth_tokens (
  id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id   uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  purpose     text NOT NULL CHECK (purpose IN ('verify_email', 'reset_password', 'invite', 'login_code')),
  token_hash  bytea NOT NULL UNIQUE,
  attempts    integer NOT NULL DEFAULT 0,
  expires_at  timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_tokens_user_active_idx ON auth_tokens (user_id, purpose) WHERE consumed_at IS NULL;
CREATE INDEX auth_tokens_expires_idx ON auth_tokens (expires_at);
SELECT app_enable_tenant_rls('auth_tokens');

CREATE TABLE files (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  owner_id     uuid REFERENCES users (id) ON DELETE SET NULL,
  purpose      text NOT NULL CHECK (purpose IN ('avatar', 'tenant_logo', 'project_cover', 'attachment', 'knowledge', 'generated')),
  visibility   text NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'tenant', 'private')),
  object_key   text NOT NULL UNIQUE,
  file_name    text NOT NULL,
  content_type text NOT NULL,
  size_bytes   bigint NOT NULL CHECK (size_bytes >= 0),
  sha256       text NOT NULL,
  scan_status  text NOT NULL CHECK (scan_status IN ('clean', 'not_scanned')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz
);
CREATE INDEX files_owner_idx ON files (owner_id);
ALTER TABLE files ENABLE ROW LEVEL SECURITY;
ALTER TABLE files FORCE ROW LEVEL SECURITY;
-- Public files (avatars, covers of published projects) are readable from any
-- university, e.g. on the national showcase.
CREATE POLICY tenant_isolation ON files
  USING (tenant_id = app_current_tenant() OR app_is_system() OR (visibility = 'public' AND deleted_at IS NULL))
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());

CREATE TABLE audit_log (
  id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id   uuid REFERENCES tenants (id) ON DELETE CASCADE,
  actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
  action      text NOT NULL,
  target_type text,
  target_id   text,
  ip          inet,
  user_agent  text,
  metadata    jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_tenant_time_idx ON audit_log (tenant_id, created_at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, created_at DESC);
SELECT app_enable_tenant_rls('audit_log');
-- The trail is append-only for the application.
REVOKE UPDATE, DELETE ON audit_log FROM qadam_app;

CREATE TABLE import_jobs (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
  kind         text NOT NULL CHECK (kind IN ('structure', 'students')),
  status       text NOT NULL DEFAULT 'preview' CHECK (status IN ('preview', 'committed')),
  file_name    text NOT NULL,
  columns      text[] NOT NULL,
  rows         jsonb NOT NULL,
  summary      jsonb NOT NULL,
  result       jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  committed_at timestamptz
);
CREATE INDEX import_jobs_tenant_time_idx ON import_jobs (tenant_id, created_at DESC);
SELECT app_enable_tenant_rls('import_jobs');

CREATE TABLE team_requests (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  author_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  title        text NOT NULL,
  description  text NOT NULL,
  roles_needed text[] NOT NULL CHECK (cardinality(roles_needed) BETWEEN 1 AND 5),
  skills       text[] NOT NULL DEFAULT '{}',
  status       text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  closed_at    timestamptz
);
CREATE TRIGGER team_requests_updated_at BEFORE UPDATE ON team_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX team_requests_open_idx ON team_requests (tenant_id, id DESC) WHERE status = 'open';
CREATE INDEX team_requests_author_idx ON team_requests (author_id);
CREATE INDEX team_requests_skills_idx ON team_requests USING gin (skills);
CREATE INDEX team_requests_title_trgm_idx ON team_requests USING gin (title gin_trgm_ops);
CREATE INDEX team_requests_description_trgm_idx ON team_requests USING gin (description gin_trgm_ops);
SELECT app_enable_tenant_rls('team_requests');

CREATE TABLE team_responses (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  request_id uuid NOT NULL REFERENCES team_requests (id) ON DELETE CASCADE,
  author_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  message    text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (request_id, author_id)
);
SELECT app_enable_tenant_rls('team_responses');

-- +goose Down
DROP TABLE team_responses;
DROP TABLE team_requests;
DROP TABLE import_jobs;
DROP TABLE audit_log;
DROP TABLE files;
DROP TABLE auth_tokens;
DROP TABLE sessions;
DROP TABLE memberships;
DROP TABLE study_groups;
DROP TABLE departments;
DROP TABLE faculties;
DROP TABLE user_profiles;
DROP TABLE user_identities;
DROP TABLE users;
DROP TABLE tenant_oidc_providers;
DROP TABLE tenant_settings;
DROP TABLE tenant_domains;
DROP TABLE tenants;
DROP FUNCTION app_enable_tenant_rls(regclass);
