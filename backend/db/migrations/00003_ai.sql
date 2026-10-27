-- +goose Up
-- AI layer: per-university provider settings and limits, an account of every
-- model call (tokens, cost, prompt version), and the knowledge base the mentor
-- cites (documents split into chunks with embeddings).

CREATE TABLE tenant_ai_settings (
  tenant_id           uuid PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
  -- {"mentor": [{"provider": "anthropic", "model": "claude-sonnet-5-5"}, {"provider": "openai"}], ...}
  routes              jsonb NOT NULL DEFAULT '{}',
  -- Only providers inside the university network may be used.
  local_only          boolean NOT NULL DEFAULT false,
  daily_user_tokens   integer NOT NULL DEFAULT 200000 CHECK (daily_user_tokens BETWEEN 0 AND 100000000),
  daily_tenant_tokens bigint NOT NULL DEFAULT 20000000 CHECK (daily_tenant_tokens BETWEEN 0 AND 100000000000),
  mentor_mode         text NOT NULL DEFAULT 'coach' CHECK (mentor_mode IN ('coach', 'coauthor')),
  updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER tenant_ai_settings_updated_at BEFORE UPDATE ON tenant_ai_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('tenant_ai_settings');

CREATE TABLE ai_usage (
  id                 uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id          uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  user_id            uuid REFERENCES users (id) ON DELETE SET NULL,
  feature            text NOT NULL CHECK (feature IN ('mentor', 'evaluation', 'generation', 'summary', 'embedding')),
  provider           text NOT NULL,
  model              text NOT NULL,
  prompt_version     text,
  input_tokens       integer NOT NULL DEFAULT 0,
  output_tokens      integer NOT NULL DEFAULT 0,
  cache_read_tokens  integer NOT NULL DEFAULT 0,
  cache_write_tokens integer NOT NULL DEFAULT 0,
  -- Estimated cost in millionths of a US dollar.
  cost_micros        bigint NOT NULL DEFAULT 0,
  latency_ms         integer NOT NULL DEFAULT 0,
  status             text NOT NULL CHECK (status IN ('ok', 'error', 'refused', 'invalid_output', 'cancelled')),
  error_code         text,
  created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ai_usage_tenant_time_idx ON ai_usage (tenant_id, created_at DESC);
CREATE INDEX ai_usage_user_time_idx ON ai_usage (tenant_id, user_id, created_at DESC);
SELECT app_enable_tenant_rls('ai_usage');

CREATE TABLE knowledge_documents (
  id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id   uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  -- The extracted text, kept to re-index when the embedding model changes.
  body        text NOT NULL,
  file_id     uuid REFERENCES files (id) ON DELETE SET NULL,
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed')),
  chunk_count integer NOT NULL DEFAULT 0,
  error       text,
  created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER knowledge_documents_updated_at BEFORE UPDATE ON knowledge_documents FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX knowledge_documents_tenant_idx ON knowledge_documents (tenant_id, created_at DESC);
SELECT app_enable_tenant_rls('knowledge_documents');

-- Embeddings of different models have different sizes, so the column has no
-- fixed dimension and searches compare only vectors of the same model. A
-- university has thousands of chunks, which an exact scan handles (D-030).
CREATE TABLE knowledge_chunks (
  id              uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id       uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  document_id     uuid NOT NULL REFERENCES knowledge_documents (id) ON DELETE CASCADE,
  ordinal         integer NOT NULL,
  heading         text,
  content         text NOT NULL,
  embedding       vector NOT NULL,
  embedding_model text NOT NULL,
  search          tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(heading, '') || ' ' || content)) STORED,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (document_id, ordinal)
);
CREATE INDEX knowledge_chunks_tenant_model_idx ON knowledge_chunks (tenant_id, embedding_model);
CREATE INDEX knowledge_chunks_search_idx ON knowledge_chunks USING gin (search);
SELECT app_enable_tenant_rls('knowledge_chunks');

-- +goose Down
DROP TABLE knowledge_chunks;
DROP TABLE knowledge_documents;
DROP TABLE ai_usage;
DROP TABLE tenant_ai_settings;
