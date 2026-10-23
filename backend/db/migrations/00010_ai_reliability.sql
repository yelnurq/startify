-- +goose Up
-- AI calls in progress hold a reservation of the tokens they may still use, so
-- that parallel requests cannot overshoot the daily limits; a reservation of a
-- process that died expires on its own.
CREATE TABLE ai_reservations (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
  user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
  tokens     integer NOT NULL CHECK (tokens > 0),
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER ai_reservations_updated_at BEFORE UPDATE ON ai_reservations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX ai_reservations_tenant_idx ON ai_reservations (tenant_id, user_id, expires_at);
SELECT app_enable_tenant_rls('ai_reservations');

-- A knowledge document that keeps failing to index is retried a few times,
-- not every few hours forever.
ALTER TABLE knowledge_documents ADD COLUMN index_failures integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE knowledge_documents DROP COLUMN index_failures;
DROP TABLE ai_reservations;
