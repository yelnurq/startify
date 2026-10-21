# Ход работ

Отчёт по каждой фазе: что сделано, как проверено, что отложено и какие есть риски.

## Фаза 0. Фундамент — готово

**Сделано**
- Монорепозиторий `backend/` · `frontend/` · `deploy/` · `docs/`, `Makefile` (dev, down, logs,
  infra, migrate, run-*, gen, lint, test, e2e, build), `.env.example`, `CLAUDE.md`.
- Бэкенд (Go 1.26): конфиг из `QADAM_*` со строгой валидацией (неизвестная переменная — ошибка
  старта), slog JSON с атрибутами из контекста, пул pgx с транзакциями, задающими область RLS
  (тенант / system, savepoint'ы для вложенных), Valkey, RFC 9457 problem details с локализуемыми
  кодами, middleware (request-id, IP за доверенными прокси, recover, access-лог, security-заголовки,
  лимит тела, таймаут), Idempotency-Key на Postgres, `/healthz` `/readyz` `/metrics`,
  OpenTelemetry (OTLP по env), graceful shutdown.
- OpenAPI-first: `backend/api/openapi.yaml` (3.1) → oapi-codegen strict server (Go) и
  openapi-typescript (TS). Первый эндпоинт `GET /api/v1/meta`.
- База: goose-миграция фундамента (pgcrypto, citext, pg_trgm, pgvector, `uuid_generate_v7()`,
  функции RLS, роль `qadam_app`, `idempotency_keys`) + миграции River. sqlc — пакет на модуль.
- Worker на River с периодической очисткой просроченных ключей идемпотентности.
- Фронтенд: React 19, TS strict, Vite 8, TanStack Router (file-based, code splitting) и Query,
  Tailwind v4, i18next с ленивой загрузкой kk/ru/en и типизированными ключами, ESLint (strict
  type-checked + запрет литералов в JSX), Prettier, Vitest + MSW, Playwright.
- Docker Compose: postgres (pgvector), valkey, minio, gotenberg, clamav, mailpit, migrate, api,
  worker, web (Nginx: SPA + прокси `/api`, CSP, кэширование ассетов). Healthcheck'и у всех
  сервисов, где это возможно.
- CI (GitHub Actions): проверка актуальности сгенерированного кода → golangci-lint → тесты с
  `-race` → фронтенд (tsc, eslint, prettier, i18n, vitest, build) → e2e против compose → сборка образов.

**Проверено**
- `go test ./...`: юнит-тесты и интеграционные на testcontainers — изоляция тенантов через RLS
  (чтение, запись, обновление чужих строк), savepoint'ы и восстановление области, UUIDv7,
  хранилище идемпотентности, откат и повторное применение всех миграций, контрактный тест
  ответов API против `openapi.yaml`. Юнит-тесты с `-race` — в Linux-контейнере.
- golangci-lint (строгий набор из ~45 линтеров): 0 замечаний.
- Фронтенд: `tsc -b`, ESLint, Prettier, `i18n:check` (29 ключей × 3 языка), 11 тестов Vitest.
- `docker compose up` (эквивалент `make dev`): все сервисы healthy; через Nginx отдаются SPA,
  `/api/v1/meta` и problem+json для неизвестных путей.
- Playwright: 6 e2e-тестов (desktop 1440×900 и Pixel 7) против запущенного стека.
- Makefile проверен GNU make в Linux-контейнере (`make help`, dry-run целей).

**Отложено**
- Реальный прогон GitHub Actions — после push в удалённый репозиторий (локально выполнены те же шаги).
- Helm-чарт — фаза 12 (B-002).

**Риски**
- Репозиторий лежит в OneDrive: синхронизация `node_modules` и Docker-томов замедляет работу.
  Рекомендуется клонировать проект в каталог вне OneDrive.
- Образ MinIO берётся из сборки Chainguard (D-008); при смене политики Chainguard хранилище
  заменяется любым S3-совместимым сервером без изменений кода.

## Фаза 1. Дизайн-система — готово

**Сделано**
- Токены Tailwind v4: шкалы 50–950 в OKLCH (primary из #00A3C4, gold #F5B800, success, warning,
  danger, info, cool neutrals), семантические токены светлой и тёмной темы, радиусы 8/12/16/20,
  три уровня теней (в тёмной теме — границы и свечение), движение 150–250 мс с учётом reduced motion.
- Темы: светлая по умолчанию, тёмная полноценная, «как в системе»; применение до отрисовки
  (`theme-init.js`, без inline-скриптов), плавное переключение через View Transitions.
- Шрифты: Inter + Geologica, self-hosted. Скрипт `fonts:check` доказал, что Unbounded и Manrope
  из ТЗ не содержат казахских букв (D-016); проверка добавлена в CI.
- ~40 компонентов на Radix: Button (8 вариантов, загрузка, asChild), IconButton с подсказками и
  горячими клавишами, Field (подпись/подсказка/ошибка/счётчик через aria), Input, Textarea,
  Select, Checkbox, Radio, Switch, Slider, Badge, Tag, Avatar/AvatarGroup с присутствием, Card,
  Callout, Tooltip, Popover, DropdownMenu, Dialog (нижний лист на телефоне), ConfirmDialog, Sheet,
  Tabs и SegmentedControl с анимированным индикатором, Accordion, Breadcrumbs, Stepper и
  StageTrack инкубатора, Table, Progress/ProgressRing, Skeleton, Spinner, StatTile с
  анимированными числами и спарклайном, ScoreGauge, EmptyState, TruncatedText, тосты (sonner)
  с «Отменить», командная палитра ⌘K с реестром команд, переключатели темы и языка, логотип.
- 10 собственных SVG-иллюстраций пустых состояний в едином стиле; орнамент «қошқар мүйіз» (D-020).
- Layout-ы: PublicLayout, AuthLayout, AppShell (сворачиваемый сайдбар, ⌘K), WorkspaceLayout
  (три колонки инкубатора). Живые демо: `/design/shell`, `/design/workspace`, `/design/auth`.
- Витрина `/design`: 13 разделов с живым расчётом контраста, оглавлением со scroll-spy.
- `docs/design/screens.md` — структура, состояния и данные всех 12 ключевых экранов.

**Проверено**
- 27 юнит-тестов (цвет и контраст, Field/Button/Avatar), `tsc` strict, ESLint (вкл. контроль строк
  интерфейса, D-019), Prettier, `i18n:check` — 481 ключ × 3 языка, `fonts:check`.
- 26 e2e (desktop 1440 и Pixel 7): нет горизонтальной прокрутки на 5 экранах; axe (WCAG 2.2 AA) без
  serious/critical нарушений в светлой и тёмной теме; тема запоминается; ⌘K; казахский язык.
  Прогнаны и против Vite, и против production-сборки в Nginx; нарушений CSP нет.
- Визуально: 68 скриншотов (`npm run screenshots`), по ним исправлены: орнамент (читался как ряд
  букв), выравнивание контейнеров, стопки аватаров, обрезка без подсказок, переполнение сеток
  на телефоне, контраст подписей, имена ссылок (WCAG 2.5.3).
  Скриншоты: `docs/design/screenshots/phase-1/`.

**Отложено**
- Скриншот-тесты с эталонами в CI — B-003 (фаза 11). Компактная мобильная шапка рабочего
  пространства — B-004 (фаза 4). Сворачивание крошек — B-005.

**Риски**
- Новые шрифты и иконки нужно проверять тем же скриптом глифов; без него легко вернуть шрифт без казахских букв.
