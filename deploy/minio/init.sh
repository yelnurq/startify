#!/usr/bin/env bash
# Runs as the one-shot minio-init service before the API and the worker start.
# Creates the application's bucket and its own MinIO user, allowed to read and
# write objects of that bucket only, so the API and the worker never hold the
# root keys. Every step is idempotent: the service runs on each start, and a
# changed S3_SECRET_KEY in .env replaces the old one.
set -euo pipefail

: "${MINIO_ROOT_USER:?}" "${MINIO_ROOT_PASSWORD:?}" "${S3_ACCESS_KEY:?}" "${S3_SECRET_KEY:?}" "${S3_BUCKET:?}"
if [[ "$S3_ACCESS_KEY" == "$MINIO_ROOT_USER" ]]; then
  echo "minio-init: S3_ACCESS_KEY must differ from MINIO_ROOT_USER: the application gets its own user" >&2
  exit 1
fi

export MC_CONFIG_DIR=/tmp/mc
for attempt in $(seq 1 60); do
  if mc alias set qadam "${MINIO_URL:-http://minio:9000}" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == 60 ]]; then
    echo "minio-init: MinIO did not answer at ${MINIO_URL:-http://minio:9000}" >&2
    exit 1
  fi
  sleep 1
done

mc mb --ignore-existing "qadam/$S3_BUCKET"

policy=/tmp/qadam-app-policy.json
cat >"$policy" <<JSON
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetBucketLocation", "s3:ListBucket", "s3:ListBucketMultipartUploads"],
      "Resource": ["arn:aws:s3:::$S3_BUCKET"]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"],
      "Resource": ["arn:aws:s3:::$S3_BUCKET/*"]
    }
  ]
}
JSON
mc admin policy create qadam qadam-app "$policy"
mc admin user add qadam "$S3_ACCESS_KEY" "$S3_SECRET_KEY"
# Some mc releases report attaching an attached policy as an error.
info=$(mc admin user info --json qadam "$S3_ACCESS_KEY")
policies=""
if [[ "$info" =~ \"policyName\":\"([^\"]*)\" ]]; then
  policies=${BASH_REMATCH[1]}
fi
if [[ ",$policies," != *",qadam-app,"* ]]; then
  mc admin policy attach qadam qadam-app --user "$S3_ACCESS_KEY"
fi
echo "minio-init: user $S3_ACCESS_KEY may use bucket $S3_BUCKET"
