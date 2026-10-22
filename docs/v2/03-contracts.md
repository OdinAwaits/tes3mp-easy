# 03 — Контракты

**Статус:** draft, нормативный.

**Обновлено:** 2026-10-22.

Этот документ — единственный источник истины для JSON-форматов, кодов ошибок,
форматов архивов и публичного HTTP API. `04-ops.md`, `05-panel.md` и
`06-client.md` описывают поведение, но не переопределяют схемы отсюда.

## 1. Правила версионирования

- Публичный HTTP API версионируется путём: `/api/v1/...`.
- JSON-envelope ops не имеет отдельного поля версии; совместимость держится на
  добавлении полей, а не на их переименовании.
- Добавлять новые поля и новые значения `status` можно без смены версии.
- Удалять, переименовывать и менять тип существующих полей нельзя без
  `/api/v2` или нового major-релиза ops.
- Любое изменение контракта требует обновления фикстур и тестов в том же PR.

## 2. Ops: JSON-envelope

При `--json` в stdout пишется ровно один JSON-объект и ничего больше. Логи,
прогресс и предупреждения идут в stderr.

Успех:

```json
{
  "ok": true,
  "data": {
    "service": "tes3mp",
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
| 6 | `docker_unavailable` | Docker/compose недоступен | 503 |
| 7 | `archive_invalid` | Некорректный архив | 422 |
| 8 | `unsafe_path` | Небезопасное имя или путь | 400 |
| 9 | `config_invalid` | Ошибка конфига | 422 |
| 10 | `checks_failed` | `doctor` нашёл ошибки | 503 |

`doctor` возвращает `0`, если нет проверок со статусом `error` (предупреждения
допустимы), и `10`, если есть хотя бы одна ошибка. Код `2` зарезервирован за
неверными аргументами и для `doctor` не используется.

## 4. Ops: формы `data` по командам

### `doctor`

```json
{
  "ok": true,
  "data": {
    "status": "warn",
    "checks": [
      {"name": "root", "status": "ok"},
      {"name": "disk", "status": "warn", "message": "меньше 5 ГБ свободно"}
    ]
  }
}
```

`status`: `ok`, `warn`, `error`. Если `status` = `error`, процесс завершается
кодом `10`.

### `status`

```json
{
  "ok": true,
  "data": {
    "service": "tes3mp",
    "state": "running",
    "health": "healthy",
    "started_at": "2026-10-22T12:00:00Z",
    "compose_project": "tes3mp"
  }
}
```

`state`: `running`, `stopped`, `starting`, `unhealthy`, `unknown`.
`health`: `healthy`, `unhealthy`, `unknown`.
`started_at` может быть `null`.

### `logs`

```json
{
  "ok": true,
  "data": {
    "lines": ["2026-10-22 12:00:00 | server started"]
  }
}
```

Всегда массив строк, без вложенных объектов. Панель показывает его в `<pre>`.

### `backups list`

```json
{
  "ok": true,
  "data": {
    "type": "mods",
    "current": "import-2026-10-22_12-00-00.tar.gz",
    "items": [
      {
        "name": "import-2026-10-22_12-00-00.tar.gz",
        "size": 123456,
        "mtime": "2026-10-22T12:00:00Z",
        "current": true
      }
    ]
  }
}
```

- `current` — `null` для state или если `current.txt` отсутствует/битый.
- `items` отсортированы по `mtime`, новые сверху.
- `sha256` в списке нет по умолчанию: это дорого. Флаг `--sha256` добавляет
  поле `sha256` к каждому элементу.

### `backups create`

```json
{
  "ok": true,
  "data": {
    "archive": "manual-2026-10-22_12-00-00.tar.gz",
    "size": 123456,
    "sha256": "abc...",
    "skipped": false
  }
}
```

При `--if-running` и остановленном сервере:

```json
{
  "ok": true,
  "data": {
    "skipped": true,
    "reason": "not_running"
  }
}
```

### `mods import`

```json
{
  "ok": true,
  "data": {
    "archive": "import-2026-10-22_12-00-00.tar.gz",
    "sha256": "abc...",
    "plugins": 12,
    "scripts": 2
  }
}
```

`mods import` не меняет `current.txt`. Импорт — это «положить архив в
`backups/mods/`», а не «включить моды».

### `mods deploy`

```json
{
  "ok": true,
  "data": {
    "archive": "import-2026-10-22_12-00-00.tar.gz",
    "sha256": "abc...",
    "previous": "import-2026-10-21_12-00-00.tar.gz",
    "state_backup": "export-2026-10-22_12-00-00.tar.gz",
    "restarted": true
  }
}
```

`previous` может быть `null`, если это первый деплой. `state_backup` — имя
созданного перед деплоем state-бэкапа; `null`, если бэкап не создавался.

### `state deploy`

```json
{
  "ok": true,
  "data": {
    "archive": "export-2026-10-22_12-00-00.tar.gz",
    "mods_archive": "import-2026-10-22_12-00-00.tar.gz",
    "restarted": true
  }
}
```

`mods_archive` — архив модов, с которым был сделан state. Может быть `null`,
если в state-архиве нет `current.txt` (старый архив).

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
      "maximumPlayers": 16
    },
    "read_only": []
  }
}
```

Для `config` таблицы и списки возвращаются как `read_only`, а не как значения
для редактирования.

### `config set`

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

## 5. Ops: lock и атомарность

- Все мутации берут эксклюзивный lock `.locks/ops.lock` через `flock`.
- `--lock-timeout` по умолчанию `30s`; при истечении — код `4`, `locked`.
- Читающие команды (`doctor`, `status`, `backups list`, `logs`) lock не берут.
- Внутренние операции, которым нужен уже взятый lock (например, state-бэкап
  внутри `mods deploy`), не вызывают публичную команду повторно. В коде это
  отдельная функция `createStateBackupLocked`, чтобы не было deadlock.
- Любая запись — temp-файл в том же каталоге, `fsync`, `rename`.
- `--dry-run` берёт shared lock и ничего не меняет.

## 6. Форматы архивов

### 6.1. Моды

```text
mods.tar.gz
├── plugins/
│   ├── mod.esp
│   ├── another.omwaddon
│   └── requiredDataFiles.json
└── scripts/
    └── test.lua
```

Правила:

- верхний уровень — только `plugins/` и `scripts/`;
- `requiredDataFiles.json` лежит **внутри `plugins/`**, не в корне;
- плагины: `esp`, `esm`, `omwaddon`, `omwscripts`, `omwgame` (регистр не важен);
- `scripts/` — только `.lua` верхнего уровня, без вложенных каталогов;
- имена скриптов: буквы, цифры, `_`, `-`; без пробелов и точек, чтобы
  `require("custom.<name>")` оставался предсказуемым;
- оригинальные `Morrowind.esm`, `Tribunal.esm`, `Bloodmoon.esm` в архив не
  попадают; если они есть в `plugins/`, импорт отклоняется с
  `archive_invalid`;
- дубликаты имён и неоднозначные регистры отклоняются.

### 6.2. State

```text
state.tar.gz
├── players/
├── cell/
├── world/
├── map/
├── recordstore/
├── custom/
├── requiredDataFiles.json
└── current.txt
```

Правила:

- `players/` и пять каталогов мира лежат в корне архива, а не внутри
  `world/`; при восстановлении `cell/`, `world/`, `map/`, `recordstore/`,
  `custom/` монтируются в `world/<name>/`;
- `requiredDataFiles.json` и `current.txt` — метаданные мод-пака, с которым
  сделан state;
- `requiredDataFiles.json` может отсутствовать в очень старом архиве; тогда
  `state deploy` предупреждает, но не падает;
- `current.txt` может отсутствовать; тогда `mods_archive` = `null`.

### 6.3. `requiredDataFiles.json`

Формат совместим с TES3MP и текущим `package.sh`:

```json
[
  {"Morrowind.esm": []},
  {"Tribunal.esm": []},
  {"Bloodmoon.esm": []},
  {"mod.esp": ["0x1A2B3C4D"]}
]
```

- массив объектов с одним ключом;
- порядок сохраняется: сначала базовые ESM, затем моды;
- CRC32 — IEEE, 8 hex-цифр в верхнем регистре с префиксом `0x`;
- пустой массив означает «проверять только наличие файла»;
- имена — базовые имена файлов, без путей;
- дубликаты ключей запрещены.

### 6.4. `current.txt`

Новый формат:

```text
<sha256> <filename>\n
```

- `sha256` — 64 hex-символа в нижнем регистре;
- `filename` — базовое имя архива модов, заканчивающееся на `.tar.gz`;
- ровно один пробел между полями, перевод строки в конце.

Старый формат, который надо принимать:

```text
<sha256>\n
```

Правила чтения:

- первый токен обязателен и должен быть 64 hex;
- если второго токена нет, это legacy-формат;
- для `mods` legacy-формат разрешён: `mods deploy --current` и `doctor` ищут
  архив по `sha256` среди `backups/mods/*.tar.gz`; если не нашли — `not_found`;
- для `state deploy` legacy-формат тоже разрешён: ops ищет архив модов по
  `sha256`; если не нашёл — `archive_invalid` с понятным сообщением;
- новые записи всегда в формате `<sha256> <filename>`.

`state deploy` не подменяет моды. Если `current.txt` указывает на другой
архив, чем текущий `backups/mods/current.txt`, ops останавливается с
`archive_invalid`: сначала админ деплоит нужные моды, потом восстанавливает
state. Это защищает от несовместимых CRC.

### 6.5. Лимиты архивов

| Параметр | Значение |
|---|---|
| Максимальный размер сжатого архива | 2 GiB |
| Максимальное число записей tar | 100 000 |
| Максимальный суммарный размер после распаковки | 8 GiB |
| Разрешённые типы записей | regular file, directory |

Symlink, hardlink, device, FIFO, абсолютные пути и `..` отклоняются с
`archive_invalid` или `unsafe_path`.

## 7. Публичный HTTP API

Базовый URL — `https://<panel>/api/v1`. Пароль сервера здесь не отдаётся
никогда.

### `GET /api/v1/server`

```json
{
  "name": "Morrowind with friends",
  "address": "203.0.113.10",
  "port": 25565,
  "status": "running",
  "mods_sha256": "abc...",
  "panel_version": "0.1.0"
}
```

- `status` — те же значения, что у `ops status`;
- `mods_sha256` — `null`, если текущего архива модов нет;
- `address` — публичный адрес из конфигурации панели, не обязательно IP
  клиента;
- `Cache-Control: no-store`.

### `GET /api/v1/mods/current`

```json
{
  "name": "import-2026-10-22_12-00-00.tar.gz",
  "sha256": "abc...",
  "size": 123456,
  "url": "/api/v1/mods/current/archive"
}
```

Если текущего архива нет — `404`:

```json
{
  "error": {
    "code": "no_mods",
    "message": "Текущий архив модов не найден"
  }
}
```

### `GET /api/v1/mods/current/archive`

- `200 OK`;
- `Content-Type: application/gzip`;
- `Content-Length`;
- `Content-Disposition: attachment; filename="<name>"`;
- `ETag: "<sha256>"`;
- `Cache-Control: public, max-age=0, must-revalidate`;
- поддержка `If-None-Match` → `304 Not Modified`;
- при отсутствии архива — `404` с `no_mods`.

### `GET /healthz`

`200 OK`, тело `ok\n`, без проверки Docker. Это liveness, а не readiness.

### Ошибки публичного API

```json
{
  "error": {
    "code": "not_found",
    "message": "Не найдено"
  }
}
```

Коды: `not_found`, `no_mods`, `rate_limited`, `internal`. HTTP-статусы:
`404`, `404`, `429`, `500`.

## 8. Вызов ops из панели

- Аргументы передаются массивом, без `sh -c`.
- Всегда используется `--root <root> --json`.
- `stdout` парсится как один JSON-объект; `stderr` уходит в `panel.log`.
- Код выхода маппится по таблице §3.
- Таймауты:

| Операция | Таймаут |
|---|---|
| `status`, `logs`, `backups list`, `config get` | 5 с |
| `start`, `stop`, `restart`, `staff set`, `config set` | 2 мин |
| `mods import`, `mods deploy`, `state deploy`, `backups create` | 10 мин |

- Одновременно может идти только одна мутация. Второй запрос получает
  `409 locked` и сообщение «операция уже выполняется».
- Панель не пытается повторять мутацию автоматически после таймаута: она
  показывает ошибку и предлагает проверить статус.

## 9. Совместимость со старым

- Старые state-архивы читаются: плоские `cell/`, `world/`, `map/`,
  `recordstore/`, `custom/`, `players/`, `requiredDataFiles.json` в корне,
  `current.txt` только с `sha256`.
- Старые mods-архивы читаются: `plugins/requiredDataFiles.json` уже внутри
  `plugins/`.
- Старые имена `export-*`, `import-*` и `manual-*` сохраняются.
- Старые URL `/list-backups/...` и `/download/...` не поддерживаются.
- Старые клиентские конфиги не мигрируются.
- Отличие от `deploy_state.sh`: новый `state deploy` не удаляет текущее
  состояние до успешной распаковки, а использует staging + rename.

## 10. Изменение контракта

1. Обновить этот документ.
2. Обновить фикстуры в `testdata/` (или аналоге).
3. Обновить тесты ops/panel/client.
4. Обновить `07-testing.md`, если появляется новый класс проверок.
5. Только после этого менять реализацию.
