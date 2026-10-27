-- +goose Up
-- Incubator: projects with their teams, the answers of every step with the
-- history of each field, comments, conversations with the AI mentor and the
-- generated pitch materials.

-- The version of the university's intellectual property policy a team
-- accepts when it creates a project; empty while the policy text is empty.
ALTER TABLE tenant_settings ADD COLUMN ip_policy_version text NOT NULL DEFAULT '';

CREATE TABLE projects (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id         uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  title             text NOT NULL CHECK (char_length(title) BETWEEN 2 AND 120),
  tagline           text NOT NULL DEFAULT '' CHECK (char_length(tagline) <= 160),
  industry          text NOT NULL CHECK (industry <> ''),
  visibility        text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'university', 'public')),
  -- Hide unit economics and technology from everyone outside the team.
  hide_sensitive    boolean NOT NULL DEFAULT false,
  faculty_id        uuid REFERENCES faculties (id) ON DELETE SET NULL,
  template_version  integer NOT NULL DEFAULT 1,
  -- Denormalised progress for lists, dashboards and analytics.
  current_step      text NOT NULL,
  completed_steps   integer NOT NULL DEFAULT 0 CHECK (completed_steps >= 0),
  ip_policy_version text NOT NULL DEFAULT '',
  ip_accepted_at    timestamptz,
  created_by        uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  archived_at       timestamptz,
  deleted_at        timestamptz
);
CREATE TRIGGER projects_updated_at BEFORE UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX projects_tenant_idx ON projects (tenant_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX projects_title_trgm_idx ON projects USING gin (title gin_trgm_ops);
SELECT app_enable_tenant_rls('projects');

CREATE TABLE project_members (
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role       text NOT NULL CHECK (role IN ('owner', 'member', 'supervisor')),
  -- The role in the team in the members' own words: "CTO", "designer".
  title      text NOT NULL DEFAULT '' CHECK (char_length(title) <= 60),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, user_id)
);
CREATE TRIGGER project_members_updated_at BEFORE UPDATE ON project_members FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX project_members_one_owner_idx ON project_members (project_id) WHERE role = 'owner';
CREATE INDEX project_members_user_idx ON project_members (tenant_id, user_id);
SELECT app_enable_tenant_rls('project_members');

-- The answers of a step; the row appears with the first save. Version grows
-- with every save and is the ETag of the step.
CREATE TABLE project_step_data (
  tenant_id        uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  step_key         text NOT NULL,
  data             jsonb NOT NULL DEFAULT '{}',
  version          integer NOT NULL DEFAULT 0,
  status           text NOT NULL DEFAULT 'not_started'
                   CHECK (status IN ('not_started', 'in_progress', 'completed', 'approved')),
  completed_at     timestamptz,
  completed_by     uuid REFERENCES users (id) ON DELETE SET NULL,
  approved_by      uuid REFERENCES users (id) ON DELETE SET NULL,
  approval_comment text,
  -- The latest AI review: {ready, summary, items, prompt_version, model}.
  ai_check         jsonb,
  ai_checked_at    timestamptz,
  updated_by       uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, step_key)
);
CREATE TRIGGER project_step_data_updated_at BEFORE UPDATE ON project_step_data FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('project_step_data');

-- Every value a field had. Autosaves of one author within a few minutes are
-- folded into one revision (its value and version are updated).
CREATE TABLE project_revisions (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  step_key   text NOT NULL,
  field_key  text NOT NULL,
  value      jsonb NOT NULL,
  version    integer NOT NULL,
  author_id  uuid REFERENCES users (id) ON DELETE SET NULL,
  source     text NOT NULL CHECK (source IN ('edit', 'mentor', 'restore')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER project_revisions_updated_at BEFORE UPDATE ON project_revisions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX project_revisions_field_idx ON project_revisions (project_id, step_key, field_key, id DESC);
CREATE INDEX project_revisions_activity_idx ON project_revisions (project_id, updated_at DESC);
SELECT app_enable_tenant_rls('project_revisions');

CREATE TABLE project_comments (
  id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id   uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  step_key    text NOT NULL,
  -- NULL for a comment on the whole step.
  field_key   text,
  parent_id   uuid REFERENCES project_comments (id) ON DELETE CASCADE,
  author_id   uuid REFERENCES users (id) ON DELETE SET NULL,
  body        text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  mentions    uuid[] NOT NULL DEFAULT '{}',
  resolved_at timestamptz,
  resolved_by uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE TRIGGER project_comments_updated_at BEFORE UPDATE ON project_comments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX project_comments_step_idx ON project_comments (project_id, step_key, id);
CREATE INDEX project_comments_mentions_idx ON project_comments USING gin (mentions);
SELECT app_enable_tenant_rls('project_comments');

-- Conversations with a model about a subject (a step of a project). The AI
-- module owns them; the subject belongs to the calling module.
CREATE TABLE ai_conversations (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  subject_type text NOT NULL CHECK (subject_type IN ('project_step')),
  subject_id   uuid NOT NULL,
  topic        text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, subject_type, subject_id, topic)
);
CREATE TRIGGER ai_conversations_updated_at BEFORE UPDATE ON ai_conversations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('ai_conversations');

CREATE TABLE ai_messages (
  id              uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id       uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  conversation_id uuid NOT NULL REFERENCES ai_conversations (id) ON DELETE CASCADE,
  role            text NOT NULL CHECK (role IN ('user', 'assistant')),
  author_id       uuid REFERENCES users (id) ON DELETE SET NULL,
  content         text NOT NULL,
  status          text NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'cancelled', 'error')),
  provider        text,
  model           text,
  prompt_version  text,
  -- Sources cited, the mentor mode and similar details of the answer.
  meta            jsonb NOT NULL DEFAULT '{}',
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ai_messages_conversation_idx ON ai_messages (conversation_id, id);
SELECT app_enable_tenant_rls('ai_messages');

CREATE TABLE project_materials (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('one_pager', 'pitch_deck', 'passport')),
  format       text NOT NULL CHECK (format IN ('pdf', 'pptx')),
  locale       text NOT NULL CHECK (locale IN ('kk', 'ru', 'en')),
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed')),
  file_id      uuid REFERENCES files (id) ON DELETE SET NULL,
  error        text,
  requested_by uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER project_materials_updated_at BEFORE UPDATE ON project_materials FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX project_materials_project_idx ON project_materials (project_id, id DESC);
SELECT app_enable_tenant_rls('project_materials');

-- +goose Down
DROP TABLE project_materials;
DROP TABLE ai_messages;
DROP TABLE ai_conversations;
DROP TABLE project_comments;
DROP TABLE project_revisions;
DROP TABLE project_step_data;
DROP TABLE project_members;
DROP TABLE projects;
ALTER TABLE tenant_settings DROP COLUMN ip_policy_version;
