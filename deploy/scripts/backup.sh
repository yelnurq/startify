#!/usr/bin/env bash
# Backs up a production installation: the database (pg_dump, custom format)
# and the uploaded files (MinIO's volume). Old copies are removed after
# BACKUP_KEEP_DAYS days (14 by default). .env is not copied: keep it apart.
# QADAM_ENV_FILE and COMPOSE_PROJECT_NAME point at another installation.
#
#   deploy/scripts/backup.sh [directory]      # default: backups/ in the repository
#
# Daily at 03:15 (crontab -e of the user who runs Docker):
#   15 3 * * * cd /opt/qadam && deploy/scripts/backup.sh >> backups/backup.log 2>&1
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
dir="${1:-$root/backups}"
keep="${BACKUP_KEEP_DAYS:-14}"
project="${COMPOSE_PROJECT_NAME:-qadam}"
env_file="${QADAM_ENV_FILE:-$root/.env}"
compose=(docker compose --env-file "$env_file" -f "$root/deploy/docker-compose.yml" -f "$root/deploy/docker-compose.prod.yml")
stamp="$(date -u +%Y%m%d-%H%M%S)"

mkdir -p "$dir"
chmod 700 "$dir"

echo "$(date -u +%FT%TZ) database → $dir/qadam-$stamp.dump"
"${compose[@]}" exec -T postgres pg_dump -U qadam -d qadam --format=custom >"$dir/qadam-$stamp.dump.partial"
mv "$dir/qadam-$stamp.dump.partial" "$dir/qadam-$stamp.dump"

echo "$(date -u +%FT%TZ) files → $dir/files-$stamp.tar.gz"
docker run --rm -v "${project}_minio-data:/data:ro" -v "$dir:/backup" alpine:3.22 \
  tar czf "/backup/files-$stamp.tar.gz.partial" -C /data .
mv "$dir/files-$stamp.tar.gz.partial" "$dir/files-$stamp.tar.gz"

find "$dir" -maxdepth 1 -type f \( -name 'qadam-*.dump' -o -name 'files-*.tar.gz' \) -mtime +"$keep" -delete
echo "$(date -u +%FT%TZ) done; copies older than $keep days removed"
