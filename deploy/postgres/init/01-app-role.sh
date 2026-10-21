#!/bin/sh
# Runs once, when the data directory is initialised. Creates the restricted
# application role: it owns no tables and has no BYPASSRLS, so row-level
# security always applies to the API and the worker. Migrations grant it access.
set -eu

psql -v ON_ERROR_STOP=1 -v app_password="$APP_DB_PASSWORD" \
  --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
CREATE ROLE qadam_app LOGIN PASSWORD :'app_password';
SQL
