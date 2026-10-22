-- +goose Up
-- Showcase: the public pages of projects with their moderation, and the
-- contact requests partners send to teams.

CREATE TABLE showcase_entries (
  project_id      uuid PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
  tenant_id       uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  slug            text NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) BETWEEN 3 AND 60),
  status          text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_review', 'published', 'rejected')),
  visibility      text NOT NULL DEFAULT 'university' CHECK (visibility IN ('university', 'public')),
  title           text NOT NULL CHECK (char_length(title) BETWEEN 2 AND 120),
  tagline         text NOT NULL DEFAULT '' CHECK (char_length(tagline) <= 160),
  industry        text NOT NULL,
  stage           text NOT NULL CHECK (stage IN ('idea', 'market', 'product', 'launch')),
  -- Kept with the page so that the national showcase needs no other tables.
  faculty_id      uuid REFERENCES faculties (id) ON DELETE SET NULL,
  faculty         jsonb,
  year            integer NOT NULL,
  cover_file_id   uuid REFERENCES files (id) ON DELETE SET NULL,
  video_url       text,
  -- {"problem": "...", "solution": "...", "market": "...", "traction": "...", "ask": "..."}
  sections        jsonb NOT NULL DEFAULT '{}',
  -- [{"name": "...", "role": "...", "avatar_url": "/api/v1/files/.../content"}]
  team            jsonb NOT NULL DEFAULT '[]',
  needs           text[] NOT NULL DEFAULT '{}',
  tags            text[] NOT NULL DEFAULT '{}',
  show_score      boolean NOT NULL DEFAULT false,
  score           integer,
  level           text,
  contact_enabled boolean NOT NULL DEFAULT true,
  -- Increases with every save by the team (ETag).
  version         integer NOT NULL DEFAULT 1,
  -- The link preview image and the hash of what it shows.
  og_file_id      uuid REFERENCES files (id) ON DELETE SET NULL,
  og_hash         text,
  review_comment  text,
  reviewed_by     uuid REFERENCES users (id) ON DELETE SET NULL,
  submitted_at    timestamptz,
  published_at    timestamptz,
  -- Written by the application from the texts (Kazakh, Russian, English).
  search          tsvector NOT NULL DEFAULT ''::tsvector,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, slug)
);
CREATE TRIGGER showcase_entries_updated_at BEFORE UPDATE ON showcase_entries FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX showcase_entries_published_idx ON showcase_entries (tenant_id, published_at DESC) WHERE status = 'published';
CREATE INDEX showcase_entries_public_idx ON showcase_entries (published_at DESC) WHERE status = 'published' AND visibility = 'public';
CREATE INDEX showcase_entries_search_idx ON showcase_entries USING gin (search);
ALTER TABLE showcase_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE showcase_entries FORCE ROW LEVEL SECURITY;
-- Published public pages are readable from any university (national showcase).
CREATE POLICY tenant_isolation ON showcase_entries
  USING (tenant_id = app_current_tenant() OR app_is_system() OR (status = 'published' AND visibility = 'public'))
  WITH CHECK (tenant_id = app_current_tenant() OR app_is_system());

CREATE TABLE contact_requests (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id    uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  name         text NOT NULL CHECK (char_length(name) BETWEEN 2 AND 120),
  email        citext NOT NULL,
  organization text,
  kind         text NOT NULL CHECK (kind IN ('investment', 'partnership', 'pilot', 'mentoring', 'hiring', 'other')),
  message      text NOT NULL CHECK (char_length(message) BETWEEN 20 AND 2000),
  locale       text NOT NULL CHECK (locale IN ('kk', 'ru', 'en')),
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'forwarded', 'rejected')),
  moderated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  moderated_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER contact_requests_updated_at BEFORE UPDATE ON contact_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX contact_requests_tenant_idx ON contact_requests (tenant_id, status, id DESC);
CREATE INDEX contact_requests_project_idx ON contact_requests (project_id, id DESC);
SELECT app_enable_tenant_rls('contact_requests');

-- +goose Down
DROP TABLE contact_requests;
DROP TABLE showcase_entries;
