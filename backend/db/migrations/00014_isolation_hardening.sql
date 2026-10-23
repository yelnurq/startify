-- +goose Up
-- Tenant isolation, fixed forward after the audit (Н-37 – Н-43).
--
-- Like every migration here, this one runs in a single transaction: the
-- indexes are built and the foreign keys validated under locks that block
-- writes (not reads) to the tables involved for the build. At pilot sizes
-- that is milliseconds; a migration of a large installation that needs
-- CREATE INDEX CONCURRENTLY goes into a file of its own that goose runs
-- outside a transaction (its NO TRANSACTION annotation).

-- 1. The system scope belongs to the worker (Н-38) -----------------------------
-- app.system is a setting any connection can change, so on its own it would
-- let a SQL injection in the API read every university. From now on it counts
-- only for members of qadam_worker: the role of the worker and the seed, never
-- of the API. Deployments create the role with LOGIN and a password before
-- migrating (deploy/postgres/roles.sh); this is a fallback. The worker gets the
-- table privileges of the application through membership in qadam_app.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'qadam_worker') THEN
    CREATE ROLE qadam_worker NOLOGIN;
  END IF;
  IF NOT pg_has_role('qadam_worker', 'qadam_app', 'MEMBER') THEN
    GRANT qadam_app TO qadam_worker;
  END IF;
  -- The functions below run as the schema owner and must see every
  -- university; a superuser is a member of every role.
  IF NOT pg_has_role(current_user, 'qadam_worker', 'MEMBER') THEN
    EXECUTE format('GRANT qadam_worker TO %I', current_user);
  END IF;
END
$$;
-- +goose StatementEnd

CREATE OR REPLACE FUNCTION app_is_system() RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.system', true), '') = 'on'
     AND pg_has_role(current_user, 'qadam_worker', 'MEMBER')
$$;

-- The two cross-tenant statements the API needs (D-068): a change of
-- credentials ends the sessions and voids the one-time links of the account in
-- every university. They run with the owner's rights and do nothing else, so
-- the API holds no general way into the system scope. The scope is switched on
-- for the statement and back off before returning (a SET clause of the
-- function would need a superuser to create it); an error rolls the setting
-- back with the transaction or savepoint.
-- +goose StatementBegin
CREATE FUNCTION app_revoke_user_sessions(p_user_id uuid, p_reason text, p_except_id uuid)
RETURNS SETOF uuid
LANGUAGE plpgsql VOLATILE SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  previous text := coalesce(current_setting('app.system', true), '');
BEGIN
  PERFORM set_config('app.system', 'on', true);
  IF NOT app_is_system() THEN
    RAISE EXCEPTION 'app_revoke_user_sessions: its owner % is not a member of qadam_worker', current_user;
  END IF;
  RETURN QUERY
    WITH revoked AS (
      UPDATE sessions SET revoked_at = now(), revoked_reason = p_reason
       WHERE user_id = p_user_id AND revoked_at IS NULL
         AND (p_except_id IS NULL OR id <> p_except_id)
      RETURNING id)
    SELECT id FROM revoked;
  PERFORM set_config('app.system', previous, true);
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION app_consume_user_tokens(p_user_id uuid, p_purposes text[])
RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  previous text := coalesce(current_setting('app.system', true), '');
  n bigint;
BEGIN
  PERFORM set_config('app.system', 'on', true);
  IF NOT app_is_system() THEN
    RAISE EXCEPTION 'app_consume_user_tokens: its owner % is not a member of qadam_worker', current_user;
  END IF;
  UPDATE auth_tokens SET consumed_at = now()
   WHERE user_id = p_user_id AND purpose = ANY(p_purposes) AND consumed_at IS NULL;
  GET DIAGNOSTICS n = ROW_COUNT;
  PERFORM set_config('app.system', previous, true);
  RETURN n;
END
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION app_revoke_user_sessions(uuid, text, uuid), app_consume_user_tokens(uuid, text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app_revoke_user_sessions(uuid, text, uuid), app_consume_user_tokens(uuid, text[]) TO qadam_app;

-- 2. Widened reads, strict writes (Н-37) ----------------------------------------
-- Public files and published public pages are readable from every university
-- (national showcase, avatars of members of several universities), but only
-- their own university writes them. One FOR ALL policy applied the widened
-- USING to UPDATE as well: a member of two universities changing the avatar
-- uploaded in the other one hit the WITH CHECK and got a 500.
DROP POLICY tenant_isolation ON files;
CREATE POLICY tenant_read ON files FOR SELECT
  USING (tenant_id = app_current_tenant() OR app_is_system() OR (visibility = 'public' AND deleted_at IS NULL));
CREATE POLICY tenant_insert ON files FOR INSERT
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());
CREATE POLICY tenant_update ON files FOR UPDATE
  USING (tenant_id = app_current_tenant() OR app_is_system())
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());
CREATE POLICY tenant_delete ON files FOR DELETE
  USING (tenant_id = app_current_tenant() OR app_is_system());

DROP POLICY tenant_isolation ON showcase_entries;
CREATE POLICY tenant_read ON showcase_entries FOR SELECT
  USING (tenant_id = app_current_tenant() OR app_is_system() OR (status = 'published' AND visibility = 'public'));
CREATE POLICY tenant_insert ON showcase_entries FOR INSERT
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());
CREATE POLICY tenant_update ON showcase_entries FOR UPDATE
  USING (tenant_id = app_current_tenant() OR app_is_system())
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());
CREATE POLICY tenant_delete ON showcase_entries FOR DELETE
  USING (tenant_id = app_current_tenant() OR app_is_system());

-- 3. Registries of universities (Н-39) ------------------------------------------
-- The university is resolved from the request host before any scope exists,
-- and its names, colours and domains are public: everyone reads them. Only the
-- system scope (seed, platform administration) creates or changes them; the
-- application never writes them in a university's scope.
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY registry_read ON tenants FOR SELECT USING (true);
CREATE POLICY registry_insert ON tenants FOR INSERT WITH CHECK (app_is_system());
CREATE POLICY registry_update ON tenants FOR UPDATE USING (app_is_system()) WITH CHECK (app_is_system());
CREATE POLICY registry_delete ON tenants FOR DELETE USING (app_is_system());

ALTER TABLE tenant_domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_domains FORCE ROW LEVEL SECURITY;
CREATE POLICY registry_read ON tenant_domains FOR SELECT USING (true);
CREATE POLICY registry_insert ON tenant_domains FOR INSERT WITH CHECK (app_is_system());
CREATE POLICY registry_update ON tenant_domains FOR UPDATE USING (app_is_system()) WITH CHECK (app_is_system());
CREATE POLICY registry_delete ON tenant_domains FOR DELETE USING (app_is_system());

-- 4. Pictures are files (Н-42) ---------------------------------------------------
-- A purged file can no longer leave a dangling avatar, seal or campus photo.
-- NOT VALID first, then VALIDATE: the pattern a large table needs (in one
-- transaction it saves nothing, see the top of the file).
ALTER TABLE users ADD CONSTRAINT users_avatar_file_id_fkey
  FOREIGN KEY (avatar_file_id) REFERENCES files (id) ON DELETE SET NULL NOT VALID;
ALTER TABLE users VALIDATE CONSTRAINT users_avatar_file_id_fkey;
ALTER TABLE tenants ADD CONSTRAINT tenants_logo_file_id_fkey
  FOREIGN KEY (logo_file_id) REFERENCES files (id) ON DELETE SET NULL NOT VALID;
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_logo_file_id_fkey;
ALTER TABLE tenants ADD CONSTRAINT tenants_cover_file_id_fkey
  FOREIGN KEY (cover_file_id) REFERENCES files (id) ON DELETE SET NULL NOT VALID;
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_cover_file_id_fkey;

-- 5. Indexes (Н-41) --------------------------------------------------------------
-- Replies of a comment thread, and the cascade when a comment is deleted.
CREATE INDEX project_comments_parent_idx ON project_comments (parent_id) WHERE parent_id IS NOT NULL;
-- Purging a file checks every reference to it (ON DELETE SET NULL); the
-- sweep of unused avatars looks them up too. tenants has a handful of rows.
CREATE INDEX knowledge_documents_file_idx ON knowledge_documents (file_id) WHERE file_id IS NOT NULL;
CREATE INDEX project_materials_file_idx ON project_materials (file_id) WHERE file_id IS NOT NULL;
CREATE INDEX showcase_entries_cover_idx ON showcase_entries (cover_file_id) WHERE cover_file_id IS NOT NULL;
CREATE INDEX showcase_entries_og_idx ON showcase_entries (og_file_id) WHERE og_file_id IS NOT NULL;
CREATE INDEX users_avatar_idx ON users (avatar_file_id) WHERE avatar_file_id IS NOT NULL;
-- People search and the member list match any part of a name (ILIKE '%…%').
CREATE INDEX users_full_name_trgm_idx ON users USING gin (full_name gin_trgm_ops);
-- Deleting an account removes its replies on the teammates board.
CREATE INDEX team_responses_author_idx ON team_responses (author_id);

-- 6. updated_at everywhere rows change (Н-43) -------------------------------------
-- A constant default adds the column without rewriting the table; rows that
-- existed before carry the time of this migration.
ALTER TABLE files ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER files_updated_at BEFORE UPDATE ON files FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE sessions ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER sessions_updated_at BEFORE UPDATE ON sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE import_jobs ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER import_jobs_updated_at BEFORE UPDATE ON import_jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE evaluation_criteria_scores ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE evaluation_criteria_scores ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER evaluation_criteria_scores_updated_at BEFORE UPDATE ON evaluation_criteria_scores FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE user_identities ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER user_identities_updated_at BEFORE UPDATE ON user_identities FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE tenant_domains ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER tenant_domains_updated_at BEFORE UPDATE ON tenant_domains FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER tenant_domains_updated_at ON tenant_domains;
ALTER TABLE tenant_domains DROP COLUMN updated_at;
DROP TRIGGER user_identities_updated_at ON user_identities;
ALTER TABLE user_identities DROP COLUMN updated_at;
DROP TRIGGER evaluation_criteria_scores_updated_at ON evaluation_criteria_scores;
ALTER TABLE evaluation_criteria_scores DROP COLUMN updated_at;
ALTER TABLE evaluation_criteria_scores DROP COLUMN created_at;
DROP TRIGGER import_jobs_updated_at ON import_jobs;
ALTER TABLE import_jobs DROP COLUMN updated_at;
DROP TRIGGER sessions_updated_at ON sessions;
ALTER TABLE sessions DROP COLUMN updated_at;
DROP TRIGGER files_updated_at ON files;
ALTER TABLE files DROP COLUMN updated_at;

DROP INDEX team_responses_author_idx;
DROP INDEX users_full_name_trgm_idx;
DROP INDEX users_avatar_idx;
DROP INDEX showcase_entries_og_idx;
DROP INDEX showcase_entries_cover_idx;
DROP INDEX project_materials_file_idx;
DROP INDEX knowledge_documents_file_idx;
DROP INDEX project_comments_parent_idx;

ALTER TABLE tenants DROP CONSTRAINT tenants_cover_file_id_fkey;
ALTER TABLE tenants DROP CONSTRAINT tenants_logo_file_id_fkey;
ALTER TABLE users DROP CONSTRAINT users_avatar_file_id_fkey;

DROP POLICY registry_delete ON tenant_domains;
DROP POLICY registry_update ON tenant_domains;
DROP POLICY registry_insert ON tenant_domains;
DROP POLICY registry_read ON tenant_domains;
ALTER TABLE tenant_domains NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_domains DISABLE ROW LEVEL SECURITY;
DROP POLICY registry_delete ON tenants;
DROP POLICY registry_update ON tenants;
DROP POLICY registry_insert ON tenants;
DROP POLICY registry_read ON tenants;
ALTER TABLE tenants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenants DISABLE ROW LEVEL SECURITY;

DROP POLICY tenant_delete ON showcase_entries;
DROP POLICY tenant_update ON showcase_entries;
DROP POLICY tenant_insert ON showcase_entries;
DROP POLICY tenant_read ON showcase_entries;
CREATE POLICY tenant_isolation ON showcase_entries
  USING (tenant_id = app_current_tenant() OR app_is_system() OR (status = 'published' AND visibility = 'public'))
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());
DROP POLICY tenant_delete ON files;
DROP POLICY tenant_update ON files;
DROP POLICY tenant_insert ON files;
DROP POLICY tenant_read ON files;
CREATE POLICY tenant_isolation ON files
  USING (tenant_id = app_current_tenant() OR app_is_system() OR (visibility = 'public' AND deleted_at IS NULL))
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());

DROP FUNCTION app_consume_user_tokens(uuid, text[]);
DROP FUNCTION app_revoke_user_sessions(uuid, text, uuid);
CREATE OR REPLACE FUNCTION app_is_system() RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.system', true), '') = 'on'
$$;
-- The roles stay: deployments create and use them.
