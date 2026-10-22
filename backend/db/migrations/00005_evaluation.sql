-- +goose Up
-- AI evaluation of projects: the university's rubric weights, evaluations with
-- the scores of every criterion, and the corrections experts made.

CREATE TABLE evaluation_rubrics (
  tenant_id  uuid PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
  -- {"problem": 15, "market": 15, ...}; weights add up to 100.
  weights    jsonb NOT NULL,
  updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER evaluation_rubrics_updated_at BEFORE UPDATE ON evaluation_rubrics FOR EACH ROW EXECUTE FUNCTION set_updated_at();
SELECT app_enable_tenant_rls('evaluation_rubrics');

CREATE TABLE evaluations (
  id              uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id       uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  project_id      uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
  status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'ready', 'failed')),
  locale          text NOT NULL CHECK (locale IN ('kk', 'ru', 'en')),
  -- The rubric used: [{"key": "problem", "weight": 15}, ...].
  rubric          jsonb NOT NULL,
  -- Versions of the steps evaluated: {"problem": 7, ...}.
  step_versions   jsonb NOT NULL DEFAULT '{}',
  ai_total        integer CHECK (ai_total BETWEEN 0 AND 100),
  total           integer CHECK (total BETWEEN 0 AND 100),
  level           text CHECK (level IN ('idea', 'validated_problem', 'mvp_ready', 'investment_ready')),
  summary         text NOT NULL DEFAULT '',
  red_flags       jsonb NOT NULL DEFAULT '[]',
  hypotheses      jsonb NOT NULL DEFAULT '[]',
  experiments     jsonb NOT NULL DEFAULT '[]',
  recommendations jsonb NOT NULL DEFAULT '[]',
  provider        text,
  model           text,
  prompt_version  text,
  error           text,
  requested_by    uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  completed_at    timestamptz
);
CREATE TRIGGER evaluations_updated_at BEFORE UPDATE ON evaluations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX evaluations_project_idx ON evaluations (project_id, id DESC);
SELECT app_enable_tenant_rls('evaluations');

CREATE TABLE evaluation_criteria_scores (
  tenant_id     uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  evaluation_id uuid NOT NULL REFERENCES evaluations (id) ON DELETE CASCADE,
  criterion_key text NOT NULL,
  ai_score      integer NOT NULL CHECK (ai_score BETWEEN 0 AND 10),
  score         integer NOT NULL CHECK (score BETWEEN 0 AND 10),
  justification text NOT NULL,
  PRIMARY KEY (evaluation_id, criterion_key)
);
SELECT app_enable_tenant_rls('evaluation_criteria_scores');

CREATE TABLE evaluation_overrides (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id     uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  evaluation_id uuid NOT NULL REFERENCES evaluations (id) ON DELETE CASCADE,
  criterion_key text NOT NULL,
  old_score     integer NOT NULL,
  new_score     integer NOT NULL CHECK (new_score BETWEEN 0 AND 10),
  comment       text NOT NULL CHECK (char_length(comment) BETWEEN 10 AND 1000),
  author_id     uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX evaluation_overrides_evaluation_idx ON evaluation_overrides (evaluation_id, id);
SELECT app_enable_tenant_rls('evaluation_overrides');

-- +goose Down
DROP TABLE evaluation_overrides;
DROP TABLE evaluation_criteria_scores;
DROP TABLE evaluations;
DROP TABLE evaluation_rubrics;
