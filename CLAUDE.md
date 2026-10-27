# CLAUDE.md — правила проекта Qadam

Qadam — мультитенантная AI-платформа проектной и инновационной деятельности вуза
(первый заказчик — КазУТБ им. К. Кулажанова). Полное ТЗ: `MASTER_PROMPT.md`.
Ход работ по фазам: `docs/PROGRESS.md`. Архитектура: `docs/ARCHITECTURE.md`.
Принятые решения: `docs/DECISIONS.md`. Отложенное: `docs/BACKLOG.md`.

## Стек

- **Backend**: Go 1.26, модульный монолит (`backend/internal/<module>`), chi v5,
  OpenAPI-first (`backend/api/openapi.yaml`, OpenAPI 3.1 → oapi-codegen strict server),
  PostgreSQL 17 + pgvector + pg_trgm, pgx v5 + sqlc (пакет на модуль), goose,
  River (очередь на Postgres), Valkey, MinIO (S3), Gotenberg, ClamAV.
- **Frontend**: React 19 + TypeScript 5.9 strict, Vite 8, TanStack Router (file-based,
  `src/app/routes`) и Query, Tailwind v4 + собственная дизайн-система на Radix,
  i18next (kk / ru / en), openapi-fetch с типами из `openapi.yaml`.
- **Infra**: Docker Compose (`deploy/docker-compose.yml` — базовый, наружу только web;
  `deploy/docker-compose.dev.yml` — порты служб на 127.0.0.1 и Mailpit, только для разработки;
  `deploy/docker-compose.prod.yml` — Caddy с HTTPS перед web, для сервера), Nginx, GitHub Actions.

## Команды

```bash
cp .env.example .env
make dev            # весь стек в Docker (базовый + dev-файл) → http://localhost:3000
make infra          # только инфраструктура, затем make migrate / run-api / run-worker / run-web
make gen            # oapi-codegen + sqlc (Docker) + openapi-typescript
make lint           # golangci-lint, tsc, eslint, prettier, i18n:check
make test           # go test (+ интеграционные на testcontainers) и vitest
make e2e            # Playwright против запущенного стека
make compose-check  # docker compose config: базовый файл, с dev-файлом и с prod-файлом
# сервер (docs/OPERATIONS.md):
make prod-init      # .env продакшна: адрес, HTTPS, почта, сгенерированные секреты
make prod-up        # базовый файл + deploy/docker-compose.prod.yml (Caddy, HTTPS)
make prod-bootstrap ADMIN_EMAIL=… ADMIN_NAME="…"   # КазУТБ и первый администратор
make prod-backup    # резервная копия базы и файлов в backups/
```

На Windows без `make`: команды из Makefile запускаются напрямую (compose — всегда с
`-f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml`); race detector
требует cgo, поэтому локально `go test ./...` (в CI — `-race`).

## Порядок работы над фичей

1. Сначала спецификация: `backend/api/openapi.yaml` и миграция `backend/db/migrations/NNNNN_*.sql`.
2. `make gen` — сгенерированный код (`internal/apigen`, `internal/*/…db`, `frontend/src/shared/api/schema.gen.ts`) руками не правится.
3. Реализация: сервис модуля → HTTP-хендлер (реализует методы `apigen.StrictServerInterface`) → регистрация в `internal/app/api.go`.
4. Тесты вместе с кодом. Баг исправляется вместе с тестом, который его ловит.
5. UI: экраны описываются в `docs/design/screens.md` до вёрстки; после вёрстки — скриншоты Playwright (desktop/mobile; тема только светлая, D-052) и визуальная проверка.

## Конвенции API

- Все пути под `/api/v1`. Ошибки — RFC 9457 (`application/problem+json`) со стабильным
  `code` вида `module.reason` (`project.not_found`); фронтенд переводит `errors.<code>`.
- Сервисы возвращают `*apperr.Error` (`internal/platform/apperr`); 400 — некорректный запрос,
  422 — ошибки полей (`apperr.Validation`), 409/412 — конфликты и `If-Match`.
- Курсорная пагинация: `?cursor=&limit=` → `{ items, next_cursor }`.
- Создающие запросы принимают `Idempotency-Key`; общие редактируемые ресурсы отдают `ETag`, обновление с устаревшим `If-Match` — 412 `precondition.failed` (D-028).
- operationId — camelCase; тег OpenAPI = модуль.

## База данных и изоляция тенантов

- Первичные ключи — UUIDv7 (`uuid.NewV7()` в Go, `uuid_generate_v7()` в SQL).
- Каждая бизнес-таблица: `tenant_id`, `created_at`, `updated_at` (триггер `set_updated_at`), при необходимости `deleted_at`.
- Каждая таблица с `tenant_id` получает RLS-политику
  `USING (tenant_id = app_current_tenant() OR app_is_system())` и тот же `WITH CHECK`
  (`app_enable_tenant_rls`); расширенное чтение — только политиками по командам (`SELECT` шире,
  запись строгая, D-083). Тест каталога в `internal/platform/migrate` проверяет каждую таблицу.
- Доступ к данным только внутри `db.Tx(ctx, …)` / `db.ReadTx`; запросы через `db.Conn(ctx)`.
  Контекст несёт область: `db.WithTenant` (обычный запрос) или `db.WithSystem` (кросс-тенантные операции — редко и осознанно,
  только в worker и seed).
- API подключается ролью `qadam_app` (без владения таблицами и BYPASSRLS); worker и seed — ролью `qadam_worker`
  (член `qadam_app`): только она входит в системную область (D-082). Миграции — владельцем.
- Модуль читает только свои таблицы (свои sqlc-запросы в `db/queries/<module>`); к чужим данным — через сервис другого модуля.

## Доступ, безопасность, побочные эффекты

- Метод сервиса начинается с проверки: `authz.Require(ctx)` / `authz.RequireRole(ctx, …)` и при
  необходимости `tenancy.Require(ctx)`. Вуз определяется только по адресу запроса, никогда по данным клиента.
- Ввод проверяется через `internal/platform/validate`: коды полей совпадают с ключами
  `common:validation.*` фронтенда. Новый код ошибки (`module.reason`) сразу добавляется в
  `common:errors` на трёх языках.
- Операции, которые можно перебирать (вход, коды, письма, создание контента), ограничиваются
  `internal/platform/ratelimit`.
- Значимые действия пишутся в аудит (`audit.Logger.Record`) в той же транзакции.
- Письма — только `mail.Mailer.Send` внутри транзакции (задача River с зашифрованными данными);
  загрузки — только `files.Service.Store` (антивирус, хэш, S3) и отдача через `/api/v1/files/{id}/content`.
- Демо-данные — пакет `internal/seed` (`make seed`), идемпотентно; тесты используют его же.

## Frontend

- Ни одной строки текста в JSX: только `t('…')` (ESLint `i18next/no-literal-string`). Ключи проверяются типами по `locales/ru`, полнота — `npm run i18n:check`. Казахский перевод — полноценный.
- API-типы только сгенерированные (`schema.gen.ts`); вызовы через `api` + `unwrap` из `shared/api/client.ts`.
- Серверное состояние — TanStack Query, локальное UI — Zustand (минимально).
- Цвета, отступы, радиусы, тени — только токены дизайн-системы, никаких «магических» значений.
  Компоненты — из `@/design-system` (витрина и правила использования — `/design`); новые
  компоненты добавляются туда же и показываются на витрине.
- Текст и кнопки primary — токены `primary` / `primary-text` (ступень 700, контраст ≥ 4,5 : 1);
  `fg-faint` — только иконки и декор. Обрезанный текст — только через `TruncatedText` (подсказка).
- Любая сетка, которая на телефоне становится одной колонкой, начинается с `grid-cols-1`
  (e2e проверяет отсутствие горизонтальной прокрутки).
- Визуальная проверка: `npm run screenshots` (desktop/mobile) — смотреть глазами
  после каждого нового экрана; e2e `design-system.spec.ts` прогоняет axe (WCAG 2.2 AA).

## Качество и стиль

- Conventional Commits, маленькие осмысленные коммиты. Ветка работ — `feat/qadam-platform`.
- Нет `TODO` без задачи в `docs/BACKLOG.md`, нет закомментированного кода, нет секретов в репозитории (`.env.example` актуален).
- Неочевидный выбор — вариант, который проще поддерживать on-prem в вузе, с записью в `docs/DECISIONS.md`.
- Комментарии в коде — на английском; документация и тексты интерфейса — kk / ru / en.
