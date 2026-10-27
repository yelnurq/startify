# Сертификат вуза для HTTPS

Нужен, только если сертификат выдаёт университет (`CADDY_TLS=/certs/fullchain.pem /certs/privkey.pem`
в `.env`). Положите сюда:

- `fullchain.pem` — сертификат сервера вместе с промежуточными сертификатами;
- `privkey.pem` — закрытый ключ (права `600`).

Файлы `*.pem` и `*.key` не попадают в git (`.gitignore`). Подробнее — `docs/OPERATIONS.md`.
