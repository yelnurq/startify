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
make seed        # демо-данные КазУТБ: структура, аккаунты, 25 проектов с AI-оценками, витрина, база знаний
```

После сборки:

| Что | Адрес |
|---|---|
| Веб-приложение | http://localhost:3000 |
| Витрина проектов | http://localhost:3000/showcase (национальная — `?scope=national`) |
| Дизайн-система | http://localhost:3000/design |
| API (метаданные) | http://localhost:8080/api/v1/meta |
| Перехват e-mail (Mailpit) | http://localhost:8025 |
| Консоль MinIO | http://localhost:9901 |

Порты настраиваются в `.env`. Остановка — `make down` (данные сохраняются в Docker-томах).

### Демо-аккаунты

Пароль у всех — `Qadam-Demo-2026!`. Письма (коды входа, ссылки подтверждения и сброса пароля,
приглашения) приходят в Mailpit: http://localhost:8025.

| Роль | Почта |
|---|---|
| Администратор вуза и innovation manager | `admin@qadam.test` |
| Innovation manager | `innovation@qadam.test` |
| Руководство (проректор) | `rector@qadam.test` |
| Студентка, заполненный публичный профиль | `aigerim@qadam.test` |
| Студенты | `nurlan@`, `dana@`, `timur@`, `madina@qadam.test` и ещё 55 вида `aruzhan.muratova@qadam.test` |
| Научный руководитель и ментор | `gulnar@qadam.test` |
| Менторы | `bakhyt.mentor@`, `irina.mentor@`, `yerlan.mentor@`, `aliya.mentor@`, `timur.mentor@`, `saule.mentor@`, `oleg.mentor@qadam.test` |
| Жюри | `askhat@qadam.test`, `jury.venture@`, `jury.akimat@`, `jury.industry@`, `jury.techpark@`, `jury.alumni@qadam.test` |

Вуз определяется по адресу: `localhost:3000` и `kazutb.localhost:3000` — КазУТБ,
`sandbox.localhost:3000` — второй, пустой вуз для проверки изоляции (`sandbox-admin@qadam.test`).
В продакшене — `<вуз>.qadam.kz` или собственный домен вуза. Вход через SSO вуза (OIDC)
настраивается в таблице `tenant_oidc_providers` (интерфейс — фаза 10).

### Языковые модели

Из коробки AI-функции работают на встроенном детерминированном mock-провайдере — без ключей и
без доступа в интернет. Провайдер выбирается переменными в `.env` (без изменения кода):

| Вариант | Переменные |
|---|---|
| Claude (Anthropic API) | `AI_DEFAULT=anthropic`, `ANTHROPIC_API_KEY=…` |
| Локальная модель в сети вуза (Ollama, vLLM, LM Studio) | `AI_DEFAULT=openai`, `OPENAI_BASE_URL=http://<сервер>:11434/v1`, `OPENAI_MODEL=qwen3:32b`, `OPENAI_EMBEDDING_MODEL=bge-m3` |
| OpenAI или другой внешний OpenAI-совместимый сервис | как выше; адрес в интернете считается внешним сам (`OPENAI_LOCAL=auto`): персональные данные маскируются, пользователи видят предупреждение |
| Разные провайдеры по функциям с резервными | `AI_ROUTES=mentor=anthropic:claude-sonnet-5-5,openai;embedding=openai` |

Локальным сервер считается только при `OPENAI_LOCAL=auto` и адресе `localhost`, частном IP или имени
сервиса Docker (`ollama`); `OPENAI_LOCAL=true` объявляет локальным любой адрес, `false` — внешним
даже частный (например, прокси к api.openai.com). О внешнем сервере API и worker предупреждают в
журнале при старте.

Модели Claude по умолчанию: ментор — `claude-sonnet-5-5`, оценка и генерация — `claude-opus-5-5`,
сводки — `claude-haiku-5-5`. У Anthropic нет эмбеддингов: при `AI_DEFAULT=anthropic` база знаний
индексируется OpenAI-совместимым сервером с `OPENAI_EMBEDDING_MODEL`, иначе mock-эмбеддингами
(маршрут эмбеддингов на провайдера без них отклоняется). Фрагменты базы знаний с косинусной
близостью к вопросу ниже `AI_EMBEDDING_MIN_SIMILARITY` (0,3) ментору не передаются. Администратор
вуза переопределяет маршруты, включает режим «только локальные модели», задаёт дневные лимиты и
проверяет подключение на странице «AI» (`/app/admin/ai`). Подробно — `docs/ARCHITECTURE.md`
(«AI-слой») и решения D-030–D-034.

### Качество AI

`make ai-eval` прогоняет промпт AI-оценки на 22 «золотых» проектах — пустых, расплывчатых,
наполовину и полностью готовых демо-проектах, сильном проекте с одной слабостью (убыточная
юнит-экономика, один участник, противоречивый рынок, нет конкурентов, нет подтверждений),
попытках подменить инструкции, на казахском и английском — и проверяет, что модель оценивает их
как строгий эксперт: итог и уровень в ожидаемых пределах, red flags найдены, сильные проекты выше
слабых, тексты на нужном языке. Вызовы идут через AI-шлюз, как у worker. Модель выбирается теми же
переменными, что и в платформе, например:

```bash
QADAM_AI_DEFAULT=anthropic QADAM_AI_ANTHROPIC_API_KEY=... make ai-eval
```

Без переменных проверяется встроенный mock дважды: как есть и с JSON, обёрнутым в блок кода
Markdown, как отвечают многие локальные модели (так же прогоняется в CI).

## Разработка

Для разработки с горячей перезагрузкой нужны Go 1.26+ и Node.js 22+:

```bash
make infra        # PostgreSQL, Valkey, MinIO, Gotenberg, Mailpit в Docker
make migrate      # применить миграции
cd backend && go run ./cmd/seed && cd ..   # демо-данные
make run-api      # API на :8080
make run-worker   # фоновые задачи
make run-web      # Vite на http://localhost:5173 (проксирует /api на :8080)
```

| Команда | Что делает |
|---|---|
| `make gen` | Генерирует Go-сервер и модели (oapi-codegen), SQL-код (sqlc) и TS-типы из `backend/api/openapi.yaml` |
| `make lint` | golangci-lint, `tsc`, ESLint, Prettier, проверка полноты переводов |
| `make test` | Юнит- и интеграционные тесты (Postgres в testcontainers, нужен Docker) |
| `make seed` | Демо-данные в запущенный стек (идемпотентно) |
| `make ai-eval` | Проверка промпта AI-оценки на золотых проектах |
| `make e2e` | Playwright против запущенного стека |
| `make help` | Все команды |

Без `make` (например, в Windows без WSL) команды из `Makefile` можно запускать напрямую.

## Структура

```
backend/    Go: cmd/{api,worker,migrate,seed}, api/openapi.yaml, db/{migrations,queries}, internal/<module>
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
