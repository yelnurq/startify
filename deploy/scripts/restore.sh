#!/usr/bin/env bash
# Restores a production installation from deploy/scripts/backup.sh copies:
# REPLACES the current database and files. Needs the .env of the installation
# the copies come from (its passwords open them).
#
#   deploy/scripts/restore.sh backups/qadam-20261027-031500.dump backups/files-20261027-031500.tar.gz
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "Usage: $0 <qadam-….dump> <files-….tar.gz>" >&2
  exit 2
fi
dump="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
archive="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
project="${COMPOSE_PROJECT_NAME:-qadam}"
env_file="${QADAM_ENV_FILE:-$root/.env}"
compose=(docker compose --env-file "$env_file" -f "$root/deploy/docker-compose.yml" -f "$root/deploy/docker-compose.prod.yml")

read -r -p "The current database and files of '$project' will be replaced. Type yes to continue: " answer
[ "$answer" = "yes" ] || { echo "Cancelled."; exit 1; }

echo "Stopping the application…"
"${compose[@]}" stop caddy web api worker minio

echo "Restoring the database…"
"${compose[@]}" up -d --wait postgres
"${compose[@]}" exec -T postgres pg_restore -U qadam -d qadam --clean --if-exists --no-owner --exit-on-error <"$dump"

echo "Restoring the files…"
docker run --rm -v "${project}_minio-data:/data" -v "$(dirname "$archive"):/backup:ro" alpine:3.22 \
  sh -c "find /data -mindepth 1 -delete && tar xzf '/backup/$(basename "$archive")' -C /data"

echo "Starting…"
"${compose[@]}" up -d
echo "Done."
