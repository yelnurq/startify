# Архитектура Qadam

## Обзор

```
                ┌──────────────────────────── браузер (SPA, kk/ru/en) ─────────────────────────────┐
                │ React 19 · TanStack Router/Query · дизайн-система · openapi-fetch (типы из спеки) │
                └───────────────┬───────────────────────────────────────────────────────────────────┘
                                │ HTTPS (Caddy/Nginx)
                     ┌──────────▼──────────┐
                     │ web (Nginx)         │  статика SPA, /api → api, SSE без буферизации
                     └──────────┬──────────┘
                                │
   ┌────────────────────────────▼─────────────────────────────┐        ┌───────────────────────┐
   │ api (Go, chi, oapi-codegen strict server)                │        │ worker (Go, River)    │
   │ middleware: request-id → client-ip → recover → otel →    │        │ AI-оценки, PDF/PPTX,  │
   │ metrics → access-log → security headers → /api/v1:       │        │ e-mail, эмбеддинги,   │
   │ timeout → body limit → idempotency → (tenant, auth, csrf)│        │ дайджесты, вебхуки,   │
   │ модули: system, auth, tenant, project, incubator, …      │        │ housekeeping          │
   └───┬──────────────┬──────────────┬──────────────┬─────────┘        └──┬─────────┬──────────┘
       │              │              │              │                     │         │
 ┌─────▼─────┐  ┌─────▼────┐  ┌──────▼─────┐  ┌─────▼──────┐        ┌───────▼───┐ ┌───▼──────┐
 │PostgreSQL │  │ Valkey   │  │ MinIO (S3) │  │ Gotenberg  │        │ LLM:      │ │ ClamAV   │
 │+pgvector  │  │ limits,  │  │ файлы      │  │ HTML→PDF   │        │ Claude /  │ │ загрузки │
 │+RLS, River│  │ pub/sub  │  │            │  │            │        │ vLLM/Olla.│ │          │
 └───────────┘  └──────────┘  └────────────┘  └────────────┘        └───────────┘ └──────────┘
```

## Бэкенд: модульный монолит

`backend/internal/` разделён на платформу и доменные модули.

- `platform/` — инфраструктура без бизнес-логики: `config` (строгая загрузка env), `logger`
  (slog JSON с атрибутами из контекста), `db` (пул pgx, транзакции с областью RLS), `cache`
  (Valkey), `httpx` (RFC 9457, middleware, Idempotency-Key), `health`, `metrics`
  (Prometheus), `telemetry` (OpenTelemetry), `migrate` (goose + River), `apperr` (доменные ошибки
  со стабильными кодами), `testpg` (Postgres в testcontainers для интеграционных тестов).
- Доменные модули (`system`, далее `auth`, `tenant`, `user`, `project`, `incubator`, `evaluation`,
  `showcase`, `competition`, `mentorship`, `analytics`, `certificate`, `notification`, `telegram`,
  `ai`, `audit`, `files`, `webhooks`) устроены одинаково:
  - `service.go` — бизнес-правила и проверки прав; зависит от интерфейсов;
  - `store.go` — доступ к своим таблицам через собственный sqlc-пакет `<module>db`;
  - `handler.go` — реализация операций своего тега OpenAPI (`apigen.StrictServerInterface`);
  - фоновые задачи — River-воркеры модуля.
- `app/` собирает модули: `api.go` — роутер и композиция хендлеров, `worker.go` — регистрация воркеров.

Модули общаются через интерфейсы сервисов и доменные события, а не через чужие таблицы.

### Жизненный цикл запроса

1. `RequestID` (X-Request-ID, в логах), `ClientIP` (X-Forwarded-For только от доверенных прокси),
   `Recoverer` (panic → 500 problem), OpenTelemetry-спан с именем маршрута, метрика длительности
   по шаблону маршрута, access-лог, security-заголовки.
2. Для `/api/v1`: таймаут контекста (кроме SSE), лимит тела, Idempotency-Key для POST.
3. Сгенерированный роутер разбирает параметры, strict-хендлер декодирует тело в типы из спеки.
4. Хендлер вызывает сервис; сервис открывает `db.Tx` с областью тенанта; ошибки — `*apperr.Error`.
5. Ответ — типизированная структура из спеки или problem+json с локализуемым `code`.

### Данные и изоляция тенантов

- PostgreSQL 17: UUIDv7, `citext`, `pg_trgm`, `pgvector`.
- Каждая таблица с `tenant_id` — под RLS. `db.Tx` выставляет `app.tenant_id`/`app.system`
  транзакционно-локально; приложение подключается ролью `qadam_app`, на которую RLS действует
  всегда. Подробности — `docs/DECISIONS.md` (D-006).
- Миграции — goose (SQL, встроены в бинарник); очередь River хранит задачи в той же БД, поэтому
  задача ставится в одной транзакции с бизнес-изменением.

## Фронтенд

- `src/app` — роутер (file-based маршруты в `src/app/routes`), провайдеры, layout-ы.
- `src/design-system` — токены, темы, компоненты, иллюстрации (фаза 1).
- `src/features/<feature>` — экраны и хуки фич (`api.ts` с queryOptions).
- `src/shared` — API-клиент (`schema.gen.ts` из `openapi.yaml`, `unwrap`, `ApiError`), i18n, утилиты.
- `src/locales/{kk,ru,en}` — переводы по пространствам имён; язык грузится лениво.

## Развёртывание

Docker Compose поднимает весь стек одной командой (`make dev`): postgres, valkey, minio,
gotenberg, clamav, mailpit, одноразовый `migrate`, api, worker, web. Один образ бэкенда содержит
все бинарники (`/app/api`, `/app/worker`, `/app/migrate`). Prod-конфигурация, TLS (Caddy),
бэкапы и мониторинг — фаза 12 (`docs/OPERATIONS.md`).
