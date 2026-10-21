# Qadam

**Qadam** (каз. «шаг») — AI-платформа инновационной и проектной деятельности университета.
Она ведёт студента от идеи к структурированному проекту, публичной витрине, Demo Day и выходу
на рынок, а руководству вуза даёт измеримые результаты в реальном времени.

```
Студент → AI-ментор (инкубатор) → Стартап-проект → AI-оценка → Витрина проектов
        → Конкурс / Demo Day → Гранты, акселераторы, инвесторы → Коммерциализация
```

Первый заказчик — АО «КазУТБ им. К. Кулажанова». Платформа мультитенантная: другие вузы
Казахстана подключаются как отдельные тенанты. Интерфейс — на казахском, русском и английском.

> Статус: идёт разработка по фазам (см. [docs/PROGRESS.md](docs/PROGRESS.md)).
> Полное техническое задание — [MASTER_PROMPT.md](MASTER_PROMPT.md).

## Быстрый старт

Нужны Docker (с Compose v2) и GNU make. Go и Node.js для запуска не нужны.

```bash
git clone https://github.com/yelnurq/startify.git && cd startify
cp .env.example .env
make dev
```

После сборки:

| Что | Адрес |
|---|---|
| Веб-приложение | http://localhost:3000 |
| Дизайн-система | http://localhost:3000/design |
| API (метаданные) | http://localhost:8080/api/v1/meta |
| Перехват e-mail (Mailpit) | http://localhost:8025 |
| Консоль MinIO | http://localhost:9901 |

Порты настраиваются в `.env`. Остановка — `make down` (данные сохраняются в Docker-томах).

## Разработка

Для разработки с горячей перезагрузкой нужны Go 1.26+ и Node.js 22+:

```bash
make infra        # PostgreSQL, Valkey, MinIO, Gotenberg, Mailpit в Docker
make migrate      # применить миграции
make run-api      # API на :8080
make run-worker   # фоновые задачи
make run-web      # Vite на http://localhost:5173 (проксирует /api на :8080)
```

| Команда | Что делает |
|---|---|
| `make gen` | Генерирует Go-сервер и модели (oapi-codegen), SQL-код (sqlc) и TS-типы из `backend/api/openapi.yaml` |
| `make lint` | golangci-lint, `tsc`, ESLint, Prettier, проверка полноты переводов |
| `make test` | Юнит- и интеграционные тесты (Postgres в testcontainers, нужен Docker) |
| `make e2e` | Playwright против запущенного стека |
| `make help` | Все команды |

Без `make` (например, в Windows без WSL) команды из `Makefile` можно запускать напрямую.

## Структура

```
backend/    Go: cmd/{api,worker,migrate}, api/openapi.yaml, db/{migrations,queries}, internal/<module>
frontend/   React + TypeScript: src/{app,design-system,features,shared,locales}, e2e/
deploy/     docker-compose.yml, Dockerfile-ы, Nginx, init-скрипты PostgreSQL
docs/       ARCHITECTURE, DECISIONS, BACKLOG, PROGRESS, дизайн экранов
```

## Документация

- [Архитектура](docs/ARCHITECTURE.md)
- [Принятые решения](docs/DECISIONS.md)
- [Ход работ по фазам](docs/PROGRESS.md)
- [Отложенные задачи](docs/BACKLOG.md)
- [Правила проекта для разработчиков и AI-агентов](CLAUDE.md)
