# Установка Qadam на сервер

Инструкция для того, кто разворачивает платформу в вузе: от чистого сервера до работающего сайта
по HTTPS с первым администратором. Занимает около 30 минут, из них 10–20 — первая сборка.

## Коротко

```bash
git clone -b feat/qadam-platform <адрес репозитория> /opt/qadam && cd /opt/qadam
make prod-init                                   # адрес, HTTPS, почта; пароли создаются сами
make prod-up                                     # сборка и запуск за HTTPS
make prod-bootstrap ADMIN_EMAIL=admin@kaztbu.edu.kz ADMIN_NAME="Имя Фамилия"
```

Дальше — откройте адрес платформы и войдите под первым администратором с паролем, который
напечатала последняя команда.

## 1. Что нужно

| Что | Требование |
|---|---|
| Сервер | Ubuntu 24.04 LTS (подойдёт 22.04 или Debian 12), 4 vCPU, **8 ГБ памяти** (антивирус ClamAV занимает 1,5–3 ГБ), 60 ГБ SSD |
| Программы | Docker Engine 26+ с Compose **2.24 или новее**, git, make, openssl |
| Адрес | DNS-имя, указывающее на сервер (например, `qadam.kaztbu.edu.kz`, A-запись на его IP), — или просто IP-адрес для пробного запуска |
| Сеть | Входящие порты **80 и 443** (и SSH). Исходящий доступ в интернет: Docker Hub, `proxy.golang.org`, `registry.npmjs.org` для сборки, Let's Encrypt для сертификата, обновления баз ClamAV |
| Почта | SMTP-сервер вуза и ящик-отправитель на домене вуза (`qadam@kaztbu.edu.kz`): без почты продакшн не запускается — по ней приходят коды входа, подтверждение адреса и сброс пароля |
| AI (по желанию) | Ключ Anthropic или локальная модель в сети вуза. Без них AI-функции работают на встроенных заготовленных ответах |

Платформа устанавливается на один сервер; база данных, файлы и очереди — в Docker-томах на нём же.

## 2. Подготовка сервера

```bash
sudo apt update && sudo apt -y upgrade
sudo apt -y install git make openssl
curl -fsSL https://get.docker.com | sudo sh          # Docker Engine и Compose
sudo usermod -aG docker "$USER" && newgrp docker      # docker без sudo
docker compose version                                # должно быть 2.24 или новее

# Межсетевой экран: только SSH, HTTP и HTTPS
sudo ufw allow OpenSSH && sudo ufw allow 80/tcp && sudo ufw allow 443/tcp && sudo ufw allow 443/udp
sudo ufw --force enable
```

Если на сервере уже работает веб-сервер (Apache, nginx) на портах 80/443, остановите его или
используйте другой сервер.

## 3. Код

Репозиторий закрытый: на сервере нужен доступ к нему — ключ развёртывания GitHub (Settings →
Deploy keys, только чтение) или персональный токен.

```bash
sudo mkdir -p /opt/qadam && sudo chown "$USER" /opt/qadam
git clone -b feat/qadam-platform git@github.com:yelnurq/startify.git /opt/qadam
cd /opt/qadam
```

Рабочая ветка — `feat/qadam-platform`. Если её слили в `main`, клонируйте без `-b`.

## 4. Настройка

```bash
make prod-init
```

Скрипт (`deploy/scripts/prod-init.sh`) спросит три вещи и запишет `.env` с правами `600`:

1. **Адрес платформы** — DNS-имя или IP-адрес сервера.
2. **Откуда взять сертификат HTTPS:**
   - *Let's Encrypt* — если имя видно из интернета и порты 80/443 открыты. Сертификат выпускается
     и продлевается сам; нужен e-mail для уведомлений.
   - *свой центр сертификации Caddy* — если адрес известен только в сети вуза или это IP.
     Браузеры покажут предупреждение, пока корневой сертификат Caddy не добавят в доверенные
     (его можно взять командой `docker compose … exec caddy cat /data/caddy/pki/authorities/local/root.crt`).
   - *сертификат университета* — положите `fullchain.pem` и `privkey.pem` в `deploy/certs/`.
3. **Почта:** SMTP-сервер, порт, шифрование, логин, пароль и адрес отправителя. Примеры:

   | Почта вуза | Сервер | Порт | Шифрование |
   |---|---|---|---|
   | Microsoft 365 | `smtp.office365.com` | 587 | `starttls` |
   | Google Workspace | `smtp.gmail.com` (пароль приложения) | 587 | `starttls` |
   | Свой сервер (Postfix, Exchange) | имя сервера | 587 или 465 | `starttls` или `tls` |

Все пароли баз данных, хранилища файлов, Valkey и ключ подписи сессий скрипт создаёт сам
(случайные, по 64 символа). **Сохраните копию `.env` в надёжном месте**: без неё базу и файлы из
резервной копии не открыть. В git файл не попадает.

Позже `.env` можно править вручную, например чтобы подключить языковую модель:

| Вариант | Переменные в `.env` |
|---|---|
| Claude (Anthropic API) | `AI_DEFAULT=anthropic`, `ANTHROPIC_API_KEY=…` |
| Модель в сети вуза (Ollama, vLLM) | `AI_DEFAULT=openai`, `OPENAI_BASE_URL=http://<сервер>:11434/v1`, `OPENAI_MODEL=…`, `OPENAI_EMBEDDING_MODEL=…` |

После правки `.env` — `make prod-up` (перезапустит то, что изменилось).

## 5. Запуск

```bash
make prod-up
```

Первая сборка образов занимает 10–20 минут. Затем стартуют PostgreSQL, Valkey, MinIO, ClamAV,
Gotenberg, применяются миграции базы данных, поднимаются API, фоновые задачи, веб-приложение и
Caddy с HTTPS. Команда ждёт, пока сервисы станут здоровыми, и печатает адрес.

ClamAV при первом запуске скачивает антивирусные базы (несколько минут): до этого загрузка файлов
отклоняется — так и задумано.

## 6. Первый администратор

```bash
make prod-bootstrap ADMIN_EMAIL=admin@kaztbu.edu.kz ADMIN_NAME="Ерлан Сейтказиев"
```

Команда создаёт КазУТБ на адресе платформы (название, печать, фото кампуса, согласие на обработку
персональных данных и политика интеллектуальной собственности) и первого администратора вуза.
Она **печатает пароль один раз** — войдите и смените его (меню профиля → «Безопасность»).

Дополнительные настройки — через `BOOTSTRAP_FLAGS`:

```bash
# Регистрация только с почтой университета
make prod-bootstrap ADMIN_EMAIL=… ADMIN_NAME="…" BOOTSTRAP_FLAGS="-email-domains kaztbu.edu.kz"
# Без самостоятельной регистрации: студенты приходят импортом или через Active Directory
make prod-bootstrap ADMIN_EMAIL=… ADMIN_NAME="…" BOOTSTRAP_FLAGS="-self-signup=false"
```

Команду можно запускать повторно: она обновит настройки вуза и **выдаст администратору новый
пароль** — так восстанавливается забытый пароль первого администратора.

Дальше администратор работает в интерфейсе: импортирует студентов и структуру
(«Администрирование → Импорт данных»), назначает роли, настраивает AI и рубрику оценки, а вход
через учётные записи университета — по [инструкции для Active Directory](guides/active-directory.md)
(серверу Qadam нужен доступ к контроллеру домена на порт 636 или 389).

### Демо-данные на сервере для презентации

```bash
make prod-demo
```

Заполняет КазУТБ демонстрационными данными: 25 проектов, оценки, витрина, ~90 демо-аккаунтов.
Команда спросит пароль, который получат **все** демо-аккаунты (общеизвестный пароль из README в
продакшне не принимается). Демо-проекты видны на публичной витрине — используйте это только на
сервере для показа, не на пилоте с настоящими студентами.

## 7. Проверка

```bash
make prod-ps                                   # все сервисы running/healthy, разовые — exited (0)
curl -s https://qadam.kaztbu.edu.kz/api/v1/meta  # версия и коммит
```

Откройте адрес платформы: главная КазУТБ, вход, кабинет администратора.

## 8. Обновление

```bash
cd /opt/qadam
make prod-backup                               # сначала резервная копия
make prod-update                               # git pull и перезапуск; миграции применяются сами
```

## 9. Резервные копии

`make prod-backup` (скрипт `deploy/scripts/backup.sh`) сохраняет в `backups/`:

- `qadam-<дата>.dump` — база данных (`pg_dump`, все вузы, проекты, оценки, журнал аудита);
- `files-<дата>.tar.gz` — загруженные файлы (печать вуза, аватары, обложки, материалы, документы
  базы знаний).

Копии старше 14 дней удаляются (`BACKUP_KEEP_DAYS`). Ежедневно в 03:15:

```bash
crontab -e
15 3 * * * cd /opt/qadam && deploy/scripts/backup.sh >> backups/backup.log 2>&1
```

Копируйте `backups/` и `.env` на другой сервер или в хранилище вуза — копия на том же диске не
спасёт при его отказе.

Восстановление **заменяет** текущие данные:

```bash
deploy/scripts/restore.sh backups/qadam-20261027-031500.dump backups/files-20261027-031500.tar.gz
```

На новом сервере сначала положите в `/opt/qadam` тот же `.env`, выполните `make prod-up`, затем
восстановление.

## 10. Журналы и неполадки

```bash
make prod-logs                                 # Caddy, API, фоновые задачи, веб
docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml logs api --since 1h
```

| Что видно | Причина | Что сделать |
|---|---|---|
| Браузер: сертификат недействителен, в журнале Caddy ошибки ACME | DNS-имя ещё не указывает на сервер или порты 80/443 закрыты | Проверьте A-запись и межсетевой экран; Caddy повторит сам |
| API не стартует: `QADAM_SMTP_HOST is required` / `QADAM_SMTP_FROM is required` | Не заданы SMTP-сервер или отправитель | Заполните `SMTP_*` в `.env`, `make prod-up` |
| API не стартует: `… looks like a development value` | В `.env` пароль из примера для разработки | Создайте `.env` через `make prod-init` |
| Загрузка файла: «Проверка файлов временно недоступна» | ClamAV ещё скачивает базы | Подождите несколько минут (`make prod-ps`) |
| «Университет по этому адресу не найден» (404) | Адрес в браузере не совпадает с адресом из `.env` | Откройте адрес из `PUBLIC_URL`; для другого имени — `make prod-bootstrap … BOOTSTRAP_FLAGS="-domain новое.имя"` и `PUBLIC_URL`/`QADAM_DOMAIN` в `.env` |
| `make prod-up`: порт 80 или 443 занят | На сервере работает другой веб-сервер | Остановите его (`sudo systemctl disable --now nginx apache2`) |
| Сервисы перезапускаются, `docker stats` — память на пределе | Меньше 8 ГБ памяти | Добавьте памяти; на время — swap |

## 11. Безопасность

- Наружу открыт только Caddy (80/443): база данных, Valkey, MinIO, API и служебные сервисы
  доступны лишь внутри Docker-сети. Не публикуйте их портами.
- `.env` — права `600`, только у администратора сервера; в git его нет.
- Обновляйте систему (`sudo apt upgrade`) и платформу (`make prod-update`), вход на сервер — по
  SSH-ключам.
- Журналы контейнеров ограничены (5 файлов по 20 МБ на сервис); токены из ссылок в письмах в
  журналы не пишутся.

## Без make

```bash
COMPOSE="docker compose --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml"
deploy/scripts/prod-init.sh
$COMPOSE up -d --build && $COMPOSE up -d --wait caddy api worker web
$COMPOSE run --rm --no-deps worker /app/seed bootstrap -admin-email admin@kaztbu.edu.kz -admin-name "Имя Фамилия"
deploy/scripts/backup.sh
```
