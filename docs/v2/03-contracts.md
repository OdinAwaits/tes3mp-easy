# 03 — Контракты

Этот документ — единственный источник истины для JSON-форматов, кодов ошибок,
форматов архивов, retention и публичного HTTP API. `04-ops.md`, `05-panel.md` и
`06-client.md` описывают поведение, но не переопределяют схемы отсюда.

## 1. Правила версионирования

- Публичный HTTP API версионируется путём: `/api/v1/...`.
- JSON-envelope ops не имеет отдельного поля версии; стабильность держится на
  добавлении полей, а не на их переименовании.
- Добавлять новые поля и новые значения `status` можно без смены версии.
- Удалять, переименовывать и менять тип существующих полей нельзя без
  `/api/v2` или нового major-релиза ops.
- Любое изменение контракта требует обновления фикстур и тестов в том же PR.
- Поддерживаются только форматы этой версии. State-архивы без
  `configs/config.lua` и архивы с `tes3mp-server-default.cfg` или
  `banlist.json` отклоняются как `archive_invalid`. Старый `current.txt` и
  старые имена snapshot/checkpoint не принимаются.

## 2. Ops: JSON-envelope

При `--json` в stdout пишется ровно один JSON-объект и ничего больше. Логи,
прогресс и предупреждения идут в stderr (systemd/journald).

Успех:

```json
{
  "ok": true,
  "data": {
    "unit": "tes3mp.service",
    "state": "running"
  }
}
```

Ошибка:

```json
{
  "ok": false,
  "error": {
    "code": "archive_invalid",
    "message": "requiredDataFiles.json: CRC mismatch for test.esp",
    "details": {
      "file": "test.esp"
    }
  }
}
```

- `error.code` — стабильный машинный код; `error.message` — человекочитаемый
  текст, его можно менять.
- `error.details` опционален; панель может показать его рядом с полем.
- При `--json` код выхода всё равно значим: панель строит логику по коду, а не
  по тексту.

## 3. Ops: коды выхода и ошибки

| Код | `error.code` | Значение | HTTP в панели |
|---|---|---|---|
| 0 | — | Успех | 200 |
| 1 | `internal` | Внутренняя ошибка | 500 |
| 2 | `invalid_argument` | Неверные аргументы или валидация | 400 |
| 3 | `not_found` | Объект не найден | 404 |
| 4 | `locked` | Другая мутация уже выполняется | 409 |
| 5 | `no_space` | Недостаточно места | 507 |
| 6 | `service_unavailable` | systemd, journald или нужный unit недоступен | 503 |
| 7 | `archive_invalid` | Некорректный архив или формат `current.txt` | 422 |
| 8 | `unsafe_path` | Небезопасное имя или путь | 400 |
| 9 | `config_invalid` | Ошибка конфига | 422 |
| 10 | `checks_failed` | `doctor` нашёл ошибки | 503 |
| 11 | `plugin_in_use` | Зарезервирован; текущие команды его не возвращают | 409 |
| 12 | `archive_in_use` | Зарезервирован; текущие команды его не возвращают | 409 |

`doctor` возвращает `0`, если нет проверок со статусом `error` (предупреждения
допустимы), и `10`, если есть хотя бы одна ошибка. Код `2` зарезервирован за
неверными аргументами и для `doctor` не используется.

`service_unavailable` — единственный код для недоступного systemd, journald или
нужного unit; других кодов недоступности нет.

## 4. Ops: формы `data` по командам

### `doctor`

```json
{
  "ok": true,
  "data": {
    "status": "warn",
    "checks": [
      {"name": "root", "status": "ok"},
      {"name": "disk", "status": "warn", "message": "free space 384.0 MiB is below the 1 GiB warning threshold"}
    ]
  }
}
```

`status`: `ok`, `warn`, `error`. Если `status` = `error`, процесс завершается
кодом `10`. Это исключение из формы ошибки: `doctor` не добавляет объект
`error`, потому что диагностика уже в `data.checks`; HTTP 503 панель выставляет
по коду выхода. Имена проверок и их смысл — в `04-ops.md` §6.

`details` — необязательный объект у проверки. Текущие проверки его не отдают:
`disk` сообщает только `message` с порогом и свободным объёмом. Потребление
RAM/CPU — не проверка `doctor`, а метрики панели (`05-panel.md` §13).

### `status`

```json
{
  "ok": true,
  "data": {
    "unit": "tes3mp.service",
    "state": "running",
    "health": "healthy",
    "started_at": "2026-10-22T12:00:00Z"
  }
}
```

- `unit` — ровно `tes3mp.service` или `tes3mp-easy-backup.timer`.
- `state`: `running`, `stopped`, `starting`, `unhealthy`, `unknown`.
- `health`: `healthy`, `unhealthy`, `unknown`.
- `started_at` — RFC3339 или `null`; для таймера это время последнего запуска,
  если systemd его отдаёт.
- Поля `backend` и любые другие backend-специфичные поля отсутствуют.

### `start` / `stop` / `restart`

```json
{
  "ok": true,
  "data": {
    "unit": "tes3mp.service",
    "action": "restart"
  }
}
```

`action` повторяет глагол. `unit` информационный: unit фиксирован в rootctl.
При `--dry-run` добавляется `"dry_run": true`, rootctl не вызывается.

### `logs`

```json
{
  "ok": true,
  "data": {
    "lines": ["2026-10-22 12:00:00 | server started"]
  }
}
```

Всегда массив строк, без вложенных объектов. Источник — journald, unit
`tes3mp.service`; панель показывает массив в `<pre>`.

### `backups list`

CLI: `backups list --type mods|state [--limit 200] [--before <name>] [--sha256]`.

```json
{
  "ok": true,
  "data": {
    "type": "mods",
    "current": "mods-2026-10-22_12-00-00.tar.gz",
    "items": [
      {
        "name": "mods-2026-10-22_12-00-00.tar.gz",
        "size": 123456,
        "mtime": "2026-10-22T12:00:00Z",
        "current": true
      }
    ],
    "next_before": null
  }
}
```

- `current` — имя из `backups/mods/current.txt` (`null`, если файла нет).
  Для `type=state` поле всегда `null`.
- `type=mods` отдаёт `mods-*` и `mods-init-*`.
- `type=state` отдаёт `state-*`, `state-manual-*` и `state-predeploy-*`.
  Каждый state-item дополнительно содержит `mods_archive` — имя из его
  `current.txt`, и `mods_available` — существует ли этот пакет в
  `backups/mods/`. Если `current.txt` повреждён, `mods_archive` отсутствует,
  а `mods_available=false`.
- `items` отсортированы по `mtime`, новые сверху; при равном `mtime` — по имени
  по убыванию.
- `limit` — 1..500, по умолчанию 200. `before` — имя архива, курсор
  «строго старше этого имени»; имя проверяется как безопасное.
- `next_before` — имя последнего элемента, если `len(items) == limit`, иначе
  `null`. Клиент передаёт его в `--before`, пока не получит `null`.
- `sha256` в списке нет по умолчанию: это дорого. Флаг `--sha256` добавляет
  поле `sha256` только к элементам текущей страницы.
- Битый `current.txt` — `archive_invalid`, а не молчаливый `null`.

### `backups create`

CLI: `backups create --type state [--manual] [--if-running] [--dry-run]`.

```json
{
  "ok": true,
  "data": {
    "archive": "state-2026-10-27_16-40-00.tar.gz",
    "size": 201326592,
    "sha256": "abc...",
    "skipped": false,
    "pruned": [],
    "warnings": []
  }
}
```

- Создаёт state-архив: `players/`, `world/`, `configs/config.lua` и
  `current.txt`. `tes3mp-server-default.cfg`, `banlist.json` и
  `customScripts.lua` не входят. Если `config.lua` отсутствует или не является
  обычным файлом, команда завершается `archive_invalid` (exit 7) и не оставляет
  архива.
- `current.txt` — первая запись архива, чтобы retention читал ссылку на моды
  без полного сканирования.
- После создания выполняется только state-retention: state-архивы по возрасту
  и бюджету, без чтения архивов и без удаления модов.
- Если свободного места меньше 1 GiB, перед созданием выполняется полный
  retention, который защищает моды, указанные в `current.txt` любого state.
- `--if-running` и остановленный сервер:
  `{"skipped": true, "reason": "not_running", "pruned": [], "warnings": []}`;
  поля `archive`, `size`, `sha256` отсутствуют.
- `--dry-run` печатает план и файла не создаёт.

### `backups prune`

CLI: `backups prune [--dry-run]`.

```json
{
  "ok": true,
  "data": {
    "removed": ["mods-2026-08-01_12-00-00.tar.gz"],
    "warnings": []
  }
}
```

- Полный retention: state-архивы по политике §6.8 и mods-архивы по фиксированным
  keep-счётчикам.
- Перед удалением mods читается `current.txt` каждого state; все указанные
  пакеты защищены. Текущий `backups/mods/current.txt` защищён всегда.
- `--dry-run` возвращает план в `removed`, файлы не трогает.
- Команда берёт ops lock; ошибки отдельных удалений попадают в `warnings`, а
  остальные файлы продолжают удаляться.

### `state deploy`

CLI: `state deploy (--latest|--archive <name>) [--no-restart] [--dry-run]`.

```json
{
  "ok": true,
  "data": {
    "archive": "state-2026-10-27_16-40-00.tar.gz",
    "mods_archive": "mods-2026-10-27_16-40-00.tar.gz",
    "predeploy_backup": "state-predeploy-2026-10-27_17-02-11.tar.gz",
    "restarted": true
  }
}
```

- State-архив обязан ссылаться на текущий пакет модов: `current.txt` состояния
  должен совпадать с `backups/mods/current.txt`, а sha256 — с файлом. Иначе
  `archive_invalid`; моды автоматически не подменяются.
- Восстанавливаются `players/`, `world/` и `configs/config.lua`.
  `tes3mp-server-default.cfg`, `banlist.json`, `customScripts.lua` и остальные
  файлы `configs/` не трогаются.
- State-архивы без `configs/config.lua` или с `tes3mp-server-default.cfg` либо
  `banlist.json` отклоняются как `archive_invalid`; обратной совместимости нет.
- Перед изменением всегда создаётся `state-predeploy-*` с текущими данными.
- Если сервер работал, он останавливается до подмены и запускается после, кроме
  `--no-restart`.

### `state import`

CLI: `state import --file <path> [--name <archive>] [--dry-run]`.

```json
{
  "ok": true,
  "data": {
    "archive": "state-manual-2026-10-22_12-00-00.tar.gz",
    "sha256": "abc...",
    "mods_archive": "mods-2026-10-22_12-00-00.tar.gz",
    "players": 12,
    "world_files": 340,
    "pruned": [],
    "warnings": []
  }
}
```

- Файл должен быть обычным state `.tar.gz` и находиться на той же файловой
  системе, что `backups/state`; при `EXDEV` используется атомарное копирование.
- `--name` сохраняет точное имя скачанного state. Без него берётся basename
  файла. Оба варианта обязаны соответствовать §6.6.
- State проверяется полностью, включая `configs/config.lua` и совпадение
  `current.txt` с активным `backups/mods/current.txt`. Поэтому перед импортом
  нужный пакет модов уже должен быть задеплоен.
- Импорт только кладёт архив в `backups/state`; восстановление выполняет
  отдельный `state deploy`. Импортированный архив защищён от retention этого
  запуска.
- Коллизия имени — `invalid_argument`; ops ничего не перезаписывает.

### `mods import`

```json
{
  "ok": true,
  "data": {
    "archive": "mods-2026-10-22_12-00-00.tar.gz",
    "sha256": "abc...",
    "plugins": 12,
    "scripts": 2,
    "pruned": [],
    "warnings": []
  }
}
```

`mods import` не меняет `current.txt`. Импорт — это «положить архив в
`backups/mods/`», а не «включить моды». Retention модов применяется сразу
после успешного импорта и защищает только что импортированный архив.

Необязательный `--name` сохраняет точное имя пакета. Он нужен для
восстановления state: `current.txt` внутри state ссылается на конкретное имя
модов. При совпадении имени ops возвращает `invalid_argument` и не
перезаписывает файл.

### `mods deploy`

```json
{
  "ok": true,
  "data": {
    "archive": "mods-2026-10-27_16-40-00.tar.gz",
    "sha256": "abc...",
    "previous": "mods-2026-10-21_12-00-00.tar.gz",
    "state_backup": "state-predeploy-2026-10-27_16-40-00.tar.gz",
    "restarted": true
  }
}
```

- `previous` может быть `null`, если это первый деплой.
- `state_backup` — state-predeploy-архив, снятый до подмены модов.
- После успешной подмены выполняется полный retention: state-архивы по политике
  и mods-архивы по keep-счётчикам, при этом защищены пакеты, на которые
  ссылается любой state.
- `restarted` равен `true`, только если сервер работал до операции и был
  запущен после неё; при `--no-restart` или изначально остановленном сервере —
  `false`.

### `staff set`

```json
{
  "ok": true,
  "data": {
    "player": "John Doe",
    "rank": 2,
    "restart_required": true
  }
}
```

### `config get`

```json
{
  "ok": true,
  "data": {
    "file": "server",
    "values": {
      "hostname": "Morrowind with friends",
      "port": 25565,
      "password": "",
      "maximumPlayers": 8
    },
    "read_only": []
  }
}
```

Для `config` таблицы и списки возвращаются как `read_only`, а не как значения
для редактирования.

### `config set`

CLI:

```text
config set --file server|config|banlist --key <k> --value <v> [--dry-run]
```

```json
{
  "ok": true,
  "data": {
    "file": "config",
    "key": "loginTime",
    "value": 60,
    "restart_required": true
  }
}
```

- `--key`/`--value` задают одну настройку; `value` уже приведён к типу;
- `restart_required` всегда `true`: сервер не перезапускается.

## 5. Ops: lock и атомарность

- Все мутации берут эксклюзивный lock `.locks/ops.lock` через `flock`.
- `--lock-timeout` по умолчанию `30s`; при истечении — код `4`, `locked`.
- Читающие команды (`doctor`, `status`, `backups list`, `logs`) lock не берут.
- Внутренние операции, которым нужен уже взятый lock (state-predeploy внутри
  `mods deploy` и `state deploy`), не вызывают публичную команду повторно.
  В коде это `createStateBackupLocked`, чтобы не было deadlock.
- Любая запись — temp-файл в том же каталоге, `fsync`, `rename`.
- `--dry-run` берёт shared lock и ничего не меняет.

## 6. Форматы архивов

### 6.1. Моды

`mods-*` и `mods-init-*` — неизменяемые пакеты:

```text
plugins/
├── mod.esp
└── requiredDataFiles.json
scripts/
└── test.lua
```

- В корне только `plugins/` и `scripts/`.
- `requiredDataFiles.json` лежит внутри `plugins/`, не в корне.
- Плагины: `.esp`, `.esm`, `.omwaddon`, `.omwscripts`, `.omwgame`.
- Базовые `Morrowind.esm`, `Tribunal.esm`, `Bloodmoon.esm` в архив не входят.
- Скрипты — только `.lua` верхнего уровня.
- CRC32 — 8 hex-цифр в верхнем регистре; пустой массив означает проверку
  только наличия файла.
- `mods-init-*` — базовый архив bootstrap: три ESM с пустыми CRC и пустой
  `scripts/`.

Пакет модов неизменяем. `mods deploy` не пересоздаёт его, а выбирает готовый
архив и обновляет `backups/mods/current.txt`.

### 6.2. State

State-архив — снимок мира и конфигов, который ссылается на отдельный пакет
модов:

```text
current.txt
configs/
└── config.lua
requiredDataFiles.json
players/
world/
├── cell/
├── world/
├── map/
├── recordstore/
└── custom/
```

- `current.txt` записывается первой записью tar и ссылается на пакет модов.
- `configs/config.lua` обязателен. `tes3mp-server-default.cfg` с паролем
  сервера и `banlist.json` в state не входят.
- `customScripts.lua` в state не входит: он генерируется из `mods/scripts/`.
- `requiredDataFiles.json` в корне описывает состав пакета модов.
- `players/` — файлы верхнего уровня; вложенные каталоги запрещены.
- `world/` содержит ровно пять фиксированных подкаталогов; вложенность внутри
  них запрещена.
- Symlink, hardlink, device, FIFO и неизвестные top-level записи отклоняются
  с `archive_invalid`.
- Архивы без `configs/config.lua`, с `tes3mp-server-default.cfg` или
  `banlist.json` отклоняются: старый формат не поддерживается.

### 6.3. `requiredDataFiles.json`

Поля и проверки:

- JSON-массив объектов `{"<filename>": ["0xCRC32", ...]}`;
- CRC32 — 8 hex-цифр с префиксом `0x`;
- пустой массив означает «проверять только наличие файла»;
- имена — базовые имена файлов, без путей;
- дубликаты ключей запрещены;
- для модов каждый плагин из JSON должен быть в `plugins/`, и наоборот (кроме
  самого `requiredDataFiles.json`); три базовых ESM не входят в пакет.

### 6.4. `current.txt`

Единственный формат:

```text
<sha256> <filename>\n
```

- `sha256` — 64 hex-символа в нижнем регистре;
- `filename` — базовое имя пакета модов;
- ровно один пробел между полями, перевод строки в конце;
- другие форматы, CRLF, лишние пробелы и пустые строки отклоняются;
- sha256 должен совпадать с содержимым файла.

`backups/mods/current.txt` — активный пакет модов.
`current.txt` внутри state — пакет, с которым снят мир; `state deploy` требует,
чтобы активный пакет совпадал с ним.

### 6.5. Configs

`configs/` в state содержит ровно один файл:

```text
configs/
└── config.lua
```

`tes3mp-server-default.cfg` и `banlist.json` — серверные файлы, а не состояние
мира, и в публичный архив не попадают. `customScripts.lua` генерируется из
`mods/scripts/` и также не входит в state. `config set` пишет файл атомарно и
оставляет рядом `.bak-<timestamp>`; `.bak` не входит в state.

### 6.6. Лимиты архивов

| Параметр | State | Mods |
|---|---|---|
| Максимальный размер сжатого архива | 512 MiB | 512 MiB |
| Максимальный размер после распаковки | 1 GiB | 1 GiB |
| Максимальное число записей tar | 50 000 | 50 000 |
| Максимальный размер upload | — | 512 MiB |
| Разрешённые типы записей | regular file, directory | то же |

Gzip-мусор после tar, повреждённый gzip, абсолютные пути, `..`, NUL и
дубликаты отклоняются с `archive_invalid` или `unsafe_path`.

### 6.7. Имена архивов

Все имена — UTC, без пробелов:

| Тип | Шаблон | Кто создаёт |
|---|---|---|
| State | `state-YYYY-MM-DD_HH-MM-SS.tar.gz` | 15-минутный таймер |
| Ручной state | `state-manual-YYYY-MM-DD_HH-MM-SS.tar.gz` | панель, CLI |
| State перед изменением | `state-predeploy-YYYY-MM-DD_HH-MM-SS.tar.gz` | `mods deploy`, `state deploy` |
| Моды | `mods-YYYY-MM-DD_HH-MM-SS.tar.gz` | `mods import` |
| Базовые моды | `mods-init-YYYY-MM-DD_HH-MM-SS.tar.gz` | `bootstrap.sh` |

При коллизии добавляется `-01`, `-02` и так далее перед `.tar.gz`.

### 6.8. Retention

Время — UTC. Есть два режима.

**State-only** выполняется после каждого state-архива (15 минут, ручное
создание, predeploy):

- state-архивы делятся по возрасту:
  - 0–7 дней — все;
  - 7–365 дней — самый новый за UTC-дату;
  - 1–3 года — самый новый за ISO-неделю;
  - старше 3 лет — самый новый за месяц, навсегда;
- ручные state — не менее 5 новейших;
- predeploy — не менее 2 новейших;
- самый новый state и защищённые вызовом state не удаляются;
- mods-архивы не читаются и не удаляются.

**Full** выполняется на `mods import`, успешном `mods deploy`,
`backups prune` и при нехватке места:

- state-политика выше;
- mods: 5 новейших `mods-*` и 2 новейших `mods-init-*`;
- `backups/mods/current.txt` защищён;
- перед удалением читается `current.txt` каждого state; все указанные пакеты
  защищены. Если `current.txt` состояния не читается, все mods защищаются;
- удалённый state больше не защищает свой пакет: он может быть удалён при
  следующем full retention. Это безопасное направление: пакет живёт дольше
  состояния.

## 7. Публичный HTTP API

Публикуются статус сервера, моды и состояния. State-архив содержит только
`config.lua`; `tes3mp-server-default.cfg` с паролем сервера и `banlist.json` в
него не входят. Перед отдачей state-архив проверяется на текущий layout, старые
архивы с этими файлами получают `404`.

### `GET /api/v1/server`

```json
{
  "name": "Morrowind with friends",
  "address": "203.0.113.10",
  "port": 25565,
  "status": "running",
  "mods_sha256": "abc...",
  "panel_version": "0.2.4"
}
```

`name` — `hostname` из `tes3mp-server-default.cfg`; `address` — публичный адрес
панели; `port` — игровой порт; `status` — `running`, `stopped`, `starting`,
`unhealthy`, `unknown` или `service_unavailable`; `mods_sha256` — sha текущего
пакета или `null`; `panel_version` — версия серверного релиза
(`server-v<version>`), а не клиента.

### `GET /api/v1/mods/current`

```json
{
  "name": "mods-2026-10-27_16-40-00.tar.gz",
  "sha256": "abc...",
  "size": 123456,
  "url": "/api/v1/mods/current/archive"
}
```

Если `backups/mods/current.txt` отсутствует или битый — 404 `no_mods`.

### `GET /api/v1/mods/current/archive`

Отдаёт текущий пакет модов как `application/gzip` с `Content-Disposition` и
`ETag: "<sha256>"`. Поддерживает `If-None-Match`. Лимит скачиваний общий с
публичной страницей бэкапов; при превышении — 429 `rate_limited`.

### `GET /backups`

Публичная HTML-страница: состояния и архивы модов показаны одновременно в двух
колонках. Для каждого состояния видна его дата и `mods_archive`, на который оно
ссылается.

Пагинация независимая для каждой колонки: `limit` (1..500, по умолчанию 200),
`state_before` и `mods_before` — имена-курсоры из `next_before` соответствующего
списка.

### `GET /backups/mods/<name>` и `GET /backups/state/<name>`

Скачивание архива без авторизации. Имя проверяется по шаблону своего типа;
symlink, каталоги и отсутствующие файлы — `404`. Перед отдачей state-архив
проверяется на layout §6.2: старые архивы с `tes3mp-server-default.cfg` или
`banlist.json` не отдаются. Оба маршрута используют общий rate limit
скачиваний.

### `GET /backups/state/<name>/bundle`

Скачивание состояния вместе с пакетом модов, на который оно ссылается, одним
ZIP-файлом (`application/zip`, имя `<state>-with-mods.zip`). Имя пакета берётся
из `current.txt` внутри state, а не из query. В ZIP лежат исходный state и
исходный mods без перепаковки. Если state не проходит проверку layout, пакет
модов отсутствует или является symlink/каталогом — `404`. Маршрут использует
общий rate limit скачиваний.

### `GET /healthz`

```json
{
  "status": "ok"
}
```

Без авторизации, без обращения к ops и без раскрытия состояния сервера.

### Ошибки публичного API

```json
{
  "error": {
    "code": "no_mods",
    "message": "Current mods archive not found"
  }
}
```

Коды: `not_found`, `no_mods`, `rate_limited`, `service_unavailable`,
`internal`.

## 8. Вызов ops из панели

- Аргументы передаются массивом, без `sh -c`.
- Всегда используется `--root <root> --json`.
- Панель запускает ops от пользователя `tes3mp` через узкое правило sudoers;
  сама панель не имеет Unix-прав писать в игровые данные.
- `stdout` парсится как один JSON-объект; `stderr` уходит в journald.
- Код выхода маппится по таблице §3.
- Таймауты:

| Операция | Таймаут |
|---|---|
| `status`, `logs`, `backups list`, `config get` | 5 с |
| `start`, `stop`, `restart`, `staff set`, `config set`, `backups prune` | 2 мин |
| `mods import`, `mods deploy`, `state import`, `backups create`, `state deploy` | 30 мин |

- Одновременно может идти только одна мутация. Второй запрос ждёт lock до 30 с;
  если не дождался — `409 locked` и сообщение «операция уже выполняется».
- Панель не пытается повторять мутацию автоматически после таймаута: она
  показывает ошибку и предлагает проверить статус.

## 9. Клиент: локальная установка

- Morrowind определяется по каталогу, содержащему `Data Files` с
  `Morrowind.esm`, `Tribunal.esm`, `Bloodmoon.esm`. Имена сначала сравниваются
  точно, затем без учёта регистра. Оригинальная установка не изменяется.
- TES3MP фиксирован на версии `0.8.1` Win64. Клиент принимает только архив,
  SHA256 которого совпадает с зашитым в сборку.
- После успешной установки в `tes3mp/` лежат `tes3mp.exe` и
  `.tes3mp-version`; маркер содержит версию и завершающий LF (`0.8.1` + LF).
  Без совпадающего маркера установка считается устаревшей.
- Архив кешируется как `cache/tes3mp-0.8.1-win64.zip`. Временные
  `tes3mp.staging-*` и `tes3mp.old` не являются частью установки и удаляются
  после успешной замены; при сбое клиент возвращает прежний каталог.
- Клиент не скачивает моды с произвольных URL: только архивы `mods-*` /
  `mods-init-*` из API (§7). `name` проверяется по §6.7, `sha256` — 64
  lowercase hex, `size` — 1..512 MiB. `url` должен вести на тот же хост панели
  и ровно на `/api/v1/mods/current/archive`; query, fragment, credentials и
  редиректы на другой хост запрещены.
- Архив модов кешируется как `cache/<name>`, проверяется по `size` и `sha256`,
  затем проходит общий `internal/archive` (layout, CRC, лимиты, pathguard).
  Плагины копируются в `servers/<id>/data/Data Files`; серверные `scripts/` не
  устанавливаются. Базовые ESM из `requiredDataFiles.json` сверяются с
  Morrowind по CRC, если CRC указаны.
- `openmw.cfg` профиля и `Data Files` заменяются вместе; старые версии лежат в
  `*.old` до успешной записи `servers/<id>/.mods-sha256`. При любой ошибке
  клиент возвращает прежние файлы. `.mods-sha256` содержит lowercase SHA256 и
  завершающий LF; совпадение с API позволяет пропустить скачивание.
- Набор шрифтов выбирается из каталога сборки: `noto` (по умолчанию), `inter`,
  `plex` — по три TTF-файла sans/serif/mono с зашитыми HTTPS URL и SHA256.
  Идентификатор хранится в `font_set` (`config.json`); пустое или неизвестное
  значение трактуется как `noto`, поэтому старые конфиги не мигрируются.
  Клиент кладёт файлы выбранного набора в `Fonts/` каталога конфигурации
  OpenMW, генерирует `Fonts/openmw_font.xml` с Latin + Cyrillic и заменяет
  только строки `fallback=Fonts_Font_0..2`; остальные строки `openmw.cfg`
  сохраняются, а прежние family-имена набора заменяются, а не дублируются.
  После успешной установки файлы других наборов из каталога удаляются; файлы,
  которых нет в каталоге, не трогаются. Ошибка загрузки шрифта —
  предупреждение, запуск не блокируется: активный `openmw.cfg` получает
  fallback-строки только при полностью проверенном наборе. Кнопки Skip нет:
  набор выбирается шагом мастера, а установка запускается после Finish или
  кнопкой **Install** на **Installation → Fonts**. Пока идёт загрузка, строка
  показывает этап, имя файла, байты и прогресс-бар; **Cancel** останавливает
  её, **Retry** повторяет после ошибки или отмены.
- Локализация — глобальная необязательная установка, а не настройка профиля.
  Диалог **Game language** (Installation → Game language и «⋯ → Game language…»)
  показывает каталог пакетов (сейчас только Russian) и вариант «только текст /
  текст + озвучка»; выбор озвучки доступен только при `Pack.HasVoices` и
  хранится в `voiceover` (`config.json`; отсутствие поля означает озвучку).
  Шага локализации в мастере нет: без пакета игра идёт на оригинальном языке,
  а строка Installation остаётся нейтральной. Размеры вариантов приходят из
  `TextSize`/`VoicesSize` метаданных пакета. Архивы скачиваются с GitHub
  Releases проекта и проверяются по зашитым SHA256.
  Пакет собирается в `workDir/localization/main` и `voices`, не попадая в
  Morrowind. При запуске клиент добавляет
  `data="<workDir>/localization/main/Data Files"` после базового `data=` и до
  модов сервера, удаляя прежнюю управляемую строку. Для `russian` активный
  `openmw.cfg` получает `encoding=win1251`; если пакет не установлен, encoding
  берётся из самого `Morrowind.esm` (CP1251 определяется по маркерам в ESM).
  Один overlay используется всеми профилями; `game_language` и legacy
  `localization` в `config.json` не поддерживаются и отбрасываются при
  сохранении.
- `settings.cfg` правится только через **Installation → UI and video** и шаг 4
  мастера: `internal/openmwcfg` меняет `[GUI] ttf resolution`, `font size`,
  `scaling factor` и `[Video] framerate limit`. Файл принадлежит OpenMW и
  появляется после первого запуска; до этого состояние «файла ещё нет» не
  является ошибкой, а запись создаёт только эти секции и ключи. Остальные
  строки, комментарии, порядок секций, BOM и переводы строк (CRLF/LF)
  сохраняются байт в байт; дубликаты управляемого ключа схлопываются в один.
  Запись атомарная (`.part` + rename), перед ней создаётся
  `<settings.cfg>.bak-<timestamp>`. Диапазоны: TTF 16–48, шрифт 8–32, масштаб
  0.5–4, FPS 0–360 (0 — без ограничения); значения вне диапазона отклоняются
  как `T3E-GAME-03`, файл не меняется. Симлинк вместо `settings.cfg`
  отклоняется.
- Uninstall — единственное место, которое удаляет пользовательские данные.
  Область задаётся тремя флагами: `GameFiles` (весь `workDir`: TES3MP, серверы,
  моды, шрифты, локализация), `Cache` (скачанные архивы) и `Settings`
  (`config.json` с профилями). Удаляются только эти клиентские каталоги;
  установка Morrowind, её `Data Files` и `settings.cfg` не затрагиваются.
  Каталог, который содержит Morrowind, домашний каталог или каталог настроек,
  отклоняется целиком (`T3E-APP-99`), как и симлинк или не-каталог; отсутствие
  каталога не считается ошибкой. Полное удаление вместе с настройками
  закрывает лаунчер, потому что конфигурации больше нет.
- Перед запуском клиент проверяет профиль и установленные моды, создаёт
  `prefix` и ссылку `C:\Morrowind`, копирует профиль в активный `openmw.cfg` с
  добавлением глобального overlay (если он установлен) и `.bak-*`. Windows
  запускает `tes3mp.exe`; Linux — `proton run` с `SteamAppId=22320`,
  `STEAM_COMPAT_DATA_PATH` и `STEAM_COMPAT_CLIENT_INSTALL_PATH`. Ожидание
  процесса идёт в фоне, выход показывается пользователю. `openmw-cs.exe`
  запускается из advanced-раздела, а при отсутствии клиент даёт ссылку на
  OpenMW.

## 10. Совместимость

- Поддерживаются `state-*`, `state-manual-*`, `state-predeploy-*`, `mods-*` и
  `mods-init-*`.
- `snapshot-*` и `checkpoint-*` этой версией не создаются и не принимаются как
  архивы состояния.
- `current.txt` только `<sha256> <filename>`; строка только с хешем —
  `archive_invalid`.
- State-архивы без `configs/config.lua` и архивы с
  `tes3mp-server-default.cfg` или `banlist.json` не принимаются.
- Имена вне шаблонов §6.7 не являются валидными архивами.
- Старые URL `/list-backups/...` и `/download/...` не поддерживаются.
- Старые клиентские конфиги не мигрируются.
- Автоматической конвертации старых snapshot/checkpoint нет. Их можно хранить
  в холодном архиве, но эта версия их не восстанавливает.

## 11. Изменение контракта

1. Обновить этот документ.
2. Обновить фикстуры в `testdata/` (или аналоге).
3. Обновить тесты ops/panel/client.
4. Обновить `07-testing.md`, если появляется новый класс проверок.
5. Только после этого менять реализацию.
