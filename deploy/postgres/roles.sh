#!/bin/sh
# Creates the login roles of the API (qadam_app) and of the worker and the
# seed (qadam_worker), and keeps their passwords those of .env. Runs on every
# start of the stack (the db-roles service, like minio-init), so an existing
# database gets a new role or a changed password too. Neither role owns
# tables or bypasses row-level security; migrations grant them access.
# qadam_worker is a member of qadam_app (its table privileges) and the only
# role that may enter the system scope (migration 00014).
set -eu

export PGPASSWORD="$POSTGRES_PASSWORD"
psql -v ON_ERROR_STOP=1 -h "${POSTGRES_HOST:-postgres}" -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -v app_password="$APP_DB_PASSWORD" -v worker_password="$WORKER_DB_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I', name)
  FROM unnest(ARRAY['qadam_app', 'qadam_worker']) AS name
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = name) \gexec
ALTER ROLE qadam_app WITH LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD :'app_password';
ALTER ROLE qadam_worker WITH LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD :'worker_password';
GRANT qadam_app TO qadam_worker;
SQL
echo "database roles qadam_app and qadam_worker are ready"
