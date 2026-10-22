# 02 — Общая архитектура

**Статус:** draft.

**Обновлено:** 2026-10-22.

Документ описывает целевую схему и заменяет старую архитектуру с `nginx` +
`export`. Нормативные форматы и коды — в `03-contracts.md`.

## 1. Схема

```text
                                  VPS
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  Игрок ──UDP 25565──────────────► tes3mp ──пишет──► players/ world/     │
│                                      ▲                                  │
│                                      │ читает configs/ mods/            │
│                                      │                                  │
│  Клиент игрока ──HTTPS 443──┐        │                                  │
│  Браузер админа ──HTTPS 443─┼──► panel ──exec──► ops ──► files          │
│  Браузер игрока ──HTTPS 443─┘        │             │                    │
│                                      │             └──► docker compose  │
│                                      │                                  │
│  backup (тот же образ) ──exec──► ops ──пишет──► backups/               │
│                                                                         │
│  /tes3mp-easy/{configs,players,world,mods,backups,panel-data}           │
└─────────────────────────────────────────────────────────────────────────┘
```

Ключевая идея: панель — единственная точка входа для админа и клиента, но
единственный **административный писатель** — `ops`. Панель не трогает
`players/`, `world/`, `mods/`, `configs/` и `backups/` напрямую.

## 2. Компоненты

| Компонент | Где | Роль | Что пишет |
|---|---|---|---|
| `tes3mp` | VPS, Docker | Игровой сервер, UDP 25565 | `players/`, `world/` во время игры |
| `panel` | VPS, Docker, :80/:443 | HTTPS-панель, публичный API, админка | `panel-data/` |
| `backup` | VPS, Docker | Планировщик state-бэкапов | через `ops` в `backups/` |
| `ops` | Внутри образа `panel`/`backup`, CLI | Все административные операции | `configs/`, `players/`, `world/`, `mods/`, `backups/`, `.locks/` |
| `tes3mp-easy-client` | ПК игрока | Мастер установки, моды, запуск | свою рабочую папку |
| Docker Compose | VPS | Запуск трёх сервисов | — |

`panel` и `backup` используют один Docker-образ: в нём лежат
`tes3mp-easy-panel` и `tes3mp-easy-ops`. Разница только в команде запуска.
Это меньше движущихся частей, чем отдельный образ для бэкапов.

## 3. Контейнеры и монтирования

### `tes3mp`

- порт `25565/udp`;
- монтирует `configs/`, `players/`, `world/`, `mods/`, `container-data/`;
- запускается от того пользователя, от которого сейчас работает;
- не получает Docker socket.

### `panel`

- порты `80/tcp` (ACME + redirect) и `443/tcp`;
- монтирует `/tes3mp-easy` (rw) — чтобы читать бэкапы и передавать пути ops;
- монтирует `/var/run/docker.sock` (rw) — ops управляет compose;
- монтирует `panel-data/` (rw) — пароль, TLS, загрузки;
- наружу не отдаёт Docker socket и не выполняет произвольные команды.

### `backup`

- тот же образ, команда `tes3mp-easy-ops backup loop --interval 5m --if-running`;
- монтирует `/tes3mp-easy` (rw) — читает state, пишет бэкапы;
- монтирует `/var/run/docker.sock` (ro) — только чтобы узнать, запущен ли
  `tes3mp`;
- не слушает порты и не имеет HTTP-интерфейса.

Почему отдельный `backup`, а не планировщик в панели: если панель упала или её
перезапускают, бэкапы должны продолжать идти. Это не «лишний сервис», это
маленький процесс с одной задачей.

## 4. Layout на VPS

```text
/tes3mp-easy/
├── docker-compose.yml
├── .env
├── configs/
│   ├── tes3mp-server-default.cfg
│   ├── config.lua
│   ├── banlist.json
│   └── customScripts.lua        # генерируется ops, read-only
├── players/
├── world/
│   ├── cell/
│   ├── world/
│   ├── map/
│   ├── recordstore/
│   └── custom/
├── mods/
│   ├── plugins/
│   └── scripts/
├── backups/
│   ├── mods/
│   │   ├── current.txt
│   │   └── *.tar.gz
│   └── state/
│       └── *.tar.gz
├── panel-data/
│   ├── auth.json
│   ├── initial-password.txt
│   ├── tls/
│   └── uploads/                # временные загрузки модов
├── .staging/                   # распаковка и подмена, чистится ops
├── .locks/
│   └── ops.lock
└── container-data/              # данные самого TES3MP, как сейчас
```

`panel-data/uploads` — единственное место, куда панель пишет пользовательские
файлы. После успешного `ops mods import` панель удаляет временный файл.

## 5. Кто чем владеет

| Путь | Владелец | Панель | Ops | TES3MP |
|---|---|---|---|---|
| `configs/` | ops | read | write | read |
| `players/` | tes3mp + ops | read | write при restore/staff | write |
| `world/` | tes3mp + ops | read | write при restore | write |
| `mods/` | ops | read | write | read |
| `backups/` | ops | read | write | — |
| `panel-data/` | panel | write | — | — |
| `panel-data/uploads/` | panel | write | read | — |
| `.locks/` | ops | — | write | — |

«Панель читает» означает: для отображения списков и скачивания файлов. Любая
мутация идёт через ops.

## 6. Потоки

### Админ входит в панель

```text
Браузер ──HTTPS──► panel
  GET /admin/login
  POST /admin/login (пароль)
  panel проверяет Argon2id-хеш в panel-data/auth.json
  panel ставит HttpOnly/Secure/SameSite cookie + CSRF-токен
```

### Статус и управление сервером

```text
Браузер ──POST /admin/server/restart──► panel
  panel ──exec──► ops restart --json
  ops ──docker compose restart tes3mp──► Docker
  ops возвращает JSON
  panel показывает результат и обновляет статус
```

### Загрузка и деплой модов

```text
Браузер ──multipart upload──► panel
  panel пишет panel-data/uploads/<uuid>.tar.gz
  panel ──exec──► ops mods import --file ... --json
  ops проверяет архив, CRC, пути; кладёт в backups/mods/import-*.tar.gz
  panel показывает список и предлагает Deploy

Браузер ──POST /admin/mods/deploy──► panel
  panel ──exec──► ops mods deploy --archive ... --json
  ops делает state-бэкап, останавливает tes3mp, распаковывает в staging,
  меняет mods/, генерирует customScripts.lua и current.txt, запускает tes3mp
```

### State-бэкап и восстановление

```text
backup ──каждые 5 минут──► ops backup loop (внутри: createStateBackupLocked)
  ops: lock → проверка диска → tar.gz во временный файл → rename

Браузер ──POST /admin/state/deploy──► panel ──exec──► ops state deploy --archive ...
  ops: lock → проверка архива → stop → staging → подмена players/ и world/ →
       start → при ошибке вернуть старые каталоги
```

### Клиент игрока

```text
Клиент ──GET /api/v1/server──────────► panel  (имя, адрес, статус, mods_sha256)
Клиент ──GET /api/v1/mods/current────► panel  (имя архива, sha256, размер, url)
Клиент ──GET /api/v1/mods/current/archive──► panel ──read──► backups/mods/*.tar.gz
Клиент ──UDP 25565──────────────────► tes3mp
```

Пароль сервера в публичном API не отдаётся. Админ передаёт его игроку
отдельно; клиент спрашивает пароль в мастере.

## 7. Границы безопасности

- **Публично:** главная страница, список и скачивание бэкапов, текущий архив
  модов, информация о сервере (без пароля).
- **Только с паролем:** всё под `/admin/*` и `/api/v1/admin/*`.
- **Docker socket — это root.** Его получают только `panel` и `backup`, и
  только потому, что ops управляет compose. Панель не превращает HTTP-запрос в
  произвольную команду: обработчики вызывают фиксированный набор ops-команд.
- **Никакого `sh -c`.** Ops запускает процессы через `exec.Command` с
  аргументами-массивом. Пути и имена проверяются до передачи.
- **Path traversal.** Все имена архивов и игроков проверяются: только
  `filepath.Base`, без `/`, `\`, `..` и NUL. Распаковка tar запрещает
  абсолютные пути, `..`, symlink и hardlink.
- **TLS.** Панель не работает по голому HTTP для админки. Целевой режим —
  сертификат Let's Encrypt по IP; self-signed — аварийный режим с явным
  предупреждением.
- **CSRF и сессии.** Все изменяющие запросы — с CSRF-токеном. Cookie:
  `HttpOnly`, `Secure`, `SameSite=Strict`.
- **Rate limit.** Логин и публичные скачивания ограничены по IP.
- **Загрузки.** Ограничение размера, проверка gzip/tar до распаковки,
  ограничение на количество и суммарный размер файлов.

## 8. Что происходит при сбоях

| Сбой | Поведение |
|---|---|
| Панель упала | `tes3mp` и `backup` продолжают работать; бэкапы идут |
| `backup` упал | Панель показывает предупреждение; кнопка «создать бэкап» работает через ops |
| Docker недоступен | Панель отдаёт 503 и понятную ошибку, не делает вид, что всё хорошо |
| Кривой архив модов | `ops mods import` отклоняет до записи в `mods/` |
| Ошибка при деплое state | Ops возвращает старые `players/` и `world/`, запускает сервер |
| Мало места | Ops проверяет диск до упаковки/распаковки и возвращает `no_space` |
| Одновременные операции | `.locks/ops.lock`: вторая мутация ждёт или получает `locked` |
| Питание/падение в момент записи | Временный файл + rename; недописанный архив не становится `current` |

## 9. Порядок миграции

1. **Контракты и тесты.** Зафиксировать JSON-формат ops, коды выхода, layout
   архивов; собрать фикстуры и parity-тесты против текущих bash-скриптов.
2. **Ops, только чтение.** `doctor`, `status`, `backups list`; панель ещё нет.
3. **Ops, мутации.** `mods import/deploy`, `state deploy`, `staff set`,
   `config get/set`, `backups create`; тесты с temp-root и fake Docker.
4. **Панель MVP.** Авторизация, статус, логи, бэкапы, моды, state, конфиги,
   staff; панель вызывает только ops.
5. **Клиент MVP.** Мастер, Morrowind, TES3MP, моды, шрифты, запуск; может
   разрабатываться параллельно с панелью на моках API.
6. **Вырезание старого.** Убрать `nginx`, `export`, старые bash-скрипты
   администрирования; оставить bootstrap и аварийные команды.
7. **Перенос `package.sh`.** Go-пакейджер с parity-тестами; shell-версия
   остаётся референсом до полного совпадения.

На время миграции старые скрипты могут лежать в `legacy/`, но панель и клиент
не должны от них зависеть.

## 10. Non-goals архитектуры

- мультитенантность и несколько серверов на панель;
- HA, репликация, внешняя БД;
- микросервисы;
- plugin API для панели;
- хранение игровых данных в облаке;
- управление сервером из клиента;
- произвольный доступ к файлам через панель.
