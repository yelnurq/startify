#!/usr/bin/env bash
# Writes the .env of a production installation: the address, HTTPS, mail and
# freshly generated secrets. Asks for what is not given as an option.
#
#   deploy/scripts/prod-init.sh                       # interactive
#   deploy/scripts/prod-init.sh --domain qadam.kaztbu.edu.kz --tls admin@kaztbu.edu.kz \
#     --smtp-host smtp.office365.com --smtp-user qadam@kaztbu.edu.kz --smtp-from qadam@kaztbu.edu.kz
#
# Options:
#   --domain HOST        the address students open (a name, or the server's IP)
#   --tls VALUE          an e-mail for Let's Encrypt | internal | files
#   --smtp-host HOST     the university's mail server
#   --smtp-port PORT     default 587
#   --smtp-tls MODE      starttls (default) | tls | none
#   --smtp-user USER     login on the mail server (empty: no login)
#   --smtp-from ADDRESS  sender of the letters, an address of the university's domain
#   --https-port PORT    default 443 (another port only for a trial behind a busy 443)
#   --http-port PORT     default 80
#   --output FILE        default .env in the repository root
#   --force              replace an existing file
# The SMTP password is read from SMTP_PASSWORD or asked for, never taken as an option.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
output="$root/.env"
domain="" tls="" smtp_host="" smtp_port="587" smtp_tls="starttls" smtp_user="" smtp_from=""
https_port="443" http_port="80" force=0

while [ $# -gt 0 ]; do
  case "$1" in
    --domain) domain="$2"; shift 2 ;;
    --tls) tls="$2"; shift 2 ;;
    --smtp-host) smtp_host="$2"; shift 2 ;;
    --smtp-port) smtp_port="$2"; shift 2 ;;
    --smtp-tls) smtp_tls="$2"; shift 2 ;;
    --smtp-user) smtp_user="$2"; shift 2 ;;
    --smtp-from) smtp_from="$2"; shift 2 ;;
    --https-port) https_port="$2"; shift 2 ;;
    --http-port) http_port="$2"; shift 2 ;;
    --output) output="$2"; shift 2 ;;
    --force) force=1; shift ;;
    -h|--help) sed -n '2,21p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1 (see --help)" >&2; exit 2 ;;
  esac
done

if [ -e "$output" ] && [ "$force" -ne 1 ]; then
  echo "$output already exists. It holds the passwords of the running installation:" >&2
  echo "keep it, or pass --force to replace it (the databases would no longer open)." >&2
  exit 1
fi

ask() { # ask VAR "question" [default]
  local var="$1" question="$2" default="${3:-}" answer
  if [ -n "${!var}" ]; then return; fi
  if [ ! -t 0 ]; then
    if [ -n "$default" ]; then printf -v "$var" '%s' "$default"; return; fi
    echo "Missing --${var//_/-} (no terminal to ask)." >&2; exit 2
  fi
  if [ -n "$default" ]; then read -r -p "$question [$default]: " answer; answer="${answer:-$default}"
  else read -r -p "$question: " answer; fi
  printf -v "$var" '%s' "$answer"
}

ask domain "Адрес платформы (например, qadam.kaztbu.edu.kz или IP-адрес сервера)"
domain="${domain#https://}"; domain="${domain#http://}"; domain="${domain%%/*}"
if [ -z "$domain" ]; then echo "The address is required." >&2; exit 2; fi

if [ -z "$tls" ] && [ -t 0 ]; then
  echo "Сертификат HTTPS:"
  echo "  1) Let's Encrypt — у адреса публичное DNS-имя, порты 80 и 443 открыты из интернета"
  echo "  2) свой центр сертификации Caddy — адрес известен только в сети вуза или это IP"
  echo "  3) сертификат университета — файлы в deploy/certs/"
  read -r -p "Выберите 1, 2 или 3 [1]: " choice
  case "${choice:-1}" in
    1) read -r -p "E-mail для уведомлений Let's Encrypt: " tls ;;
    2) tls="internal" ;;
    3) tls="files" ;;
    *) echo "Choose 1, 2 or 3." >&2; exit 2 ;;
  esac
fi
case "$tls" in
  internal) caddy_tls="internal" ;;
  files) caddy_tls="/certs/fullchain.pem /certs/privkey.pem" ;;
  *@*) caddy_tls="$tls" ;;
  *) echo "--tls must be an e-mail (Let's Encrypt), internal or files." >&2; exit 2 ;;
esac

echo "Почта: коды входа, подтверждение адреса и сброс пароля уходят письмами через почтовый сервер вуза."
ask smtp_host "SMTP-сервер (например, smtp.office365.com или smtp.gmail.com)"
ask smtp_port "Порт SMTP" "587"
ask smtp_tls "Шифрование SMTP: starttls, tls или none" "starttls"
if [ -z "$smtp_user" ] && [ -t 0 ]; then read -r -p "Логин SMTP (пусто — без входа): " smtp_user; fi
smtp_password="${SMTP_PASSWORD:-}"
if [ -n "$smtp_user" ] && [ -z "$smtp_password" ] && [ -t 0 ]; then
  read -r -s -p "Пароль SMTP (не отображается): " smtp_password; echo
fi
case "$smtp_password" in *"'"*) echo "The SMTP password contains an apostrophe ('), which .env cannot hold literally; choose another password." >&2; exit 2 ;; esac
ask smtp_from "Адрес отправителя (на домене вуза, например qadam@kaztbu.edu.kz)" "$smtp_user"
case "$smtp_tls" in starttls|tls|none) ;; *) echo "--smtp-tls must be starttls, tls or none." >&2; exit 2 ;; esac

secret() { # 64 hex characters: no "dev" inside, nothing to quote
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex 32
  else head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'; fi
}

public_url="https://$domain"
if [ "$https_port" != "443" ]; then public_url="$public_url:$https_port"; fi

umask 077
cat > "$output" <<EOF
# Production installation of Qadam, written by deploy/scripts/prod-init.sh on $(date -u +%Y-%m-%d).
# It holds every password of this installation: keep a copy in a safe place
# (the backups do not include it) and never commit it.

QADAM_ENV=production
# The release: the tag of the built images and the version /api/v1/meta reports.
VERSION=0.7.0
PUBLIC_URL=$public_url
QADAM_DOMAIN=$domain
CADDY_TLS=$caddy_tls
HTTP_PORT=$http_port
HTTPS_PORT=$https_port
DEFAULT_LOCALE=kk
LOG_LEVEL=info

# The university is served on the platform's own address.
TENANT_BASE_DOMAIN=$domain
TENANT_DEFAULT=kazutb

# Generated secrets.
POSTGRES_PASSWORD=$(secret)
APP_DB_PASSWORD=$(secret)
WORKER_DB_PASSWORD=$(secret)
VALKEY_PASSWORD=$(secret)
MINIO_ROOT_USER=qadam-root
MINIO_ROOT_PASSWORD=$(secret)
S3_ACCESS_KEY=qadam-app
S3_SECRET_KEY=$(secret)
AUTH_SECRET_KEY=$(secret)

# Uploads are refused while the antivirus is not ready.
FILES_SCAN=required

# Mail.
SMTP_HOST=$smtp_host
SMTP_PORT=$smtp_port
SMTP_TLS=$smtp_tls
SMTP_USERNAME=$smtp_user
# Single quotes: Compose takes the value literally ($ and spaces stay as they are).
SMTP_PASSWORD='$smtp_password'
SMTP_FROM=$smtp_from
SMTP_FROM_NAME=Qadam

# Language models. "mock" works without any model (prepared answers); for
# Claude set AI_DEFAULT=anthropic and ANTHROPIC_API_KEY; for a model inside
# the university network (Ollama, vLLM) set AI_DEFAULT=openai, OPENAI_BASE_URL,
# OPENAI_MODEL and OPENAI_EMBEDDING_MODEL. See docs/OPERATIONS.md.
AI_DEFAULT=mock
AI_ROUTES=
ANTHROPIC_API_KEY=
OPENAI_BASE_URL=
OPENAI_API_KEY=
OPENAI_MODEL=
OPENAI_EMBEDDING_MODEL=
OPENAI_LOCAL=auto
EOF

echo
echo "Written $output (permissions 600)."
if [ -z "$smtp_host" ] || [ -z "$smtp_from" ]; then
  echo "Mail is not set yet: fill SMTP_HOST and SMTP_FROM in $output — production does not start without them."
fi
if [ "$tls" = "files" ]; then
  echo "Put the certificate into deploy/certs/fullchain.pem and the key into deploy/certs/privkey.pem."
fi
echo "Next: make prod-up, then make prod-bootstrap (docs/OPERATIONS.md)."
