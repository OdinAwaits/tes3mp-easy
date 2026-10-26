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
- Поддерживаются только форматы этой версии: старые форматы архивов, старые
  имена файлов и старый `current.txt` не принимаются. Это явный отказ, а не
  режим совместимости.

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
      {"name": "disk", "status": "warn", "message": "свободно меньше 1 ГиБ"}
    ]
  }
}
```

`status`: `ok`, `warn`, `error`. Если `status` = `error`, процесс завершается
кодом `10`. Это исключение из формы ошибки: `doctor` не добавляет объект
`error`, потому что диагностика уже в `data.checks`; HTTP 503 панель выставляет
по коду выхода. Имена проверок и их смысл — в `04-ops.md` §6.

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

- `current` — имя из `backups/mods/current.txt`; для state всегда `null`.
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

CLI: `backups create --type state [--manual] [--if-running]`.

```json
{
  "ok": true,
  "data": {
    "archive": "state-2026-10-22_12-00-00.tar.gz",
    "size": 123456,
    "sha256": "abc...",
    "skipped": false,
    "pruned": ["state-2026-10-15_12-00-00.tar.gz"],
    "warnings": []
  }
}
```

- Без `--manual` имя `state-*`; с `--manual` — `state-manual-*`.
- `pruned` — имена удалённых retention-архивов, может быть пустым.
- `warnings` — предупреждения retention/диска, может быть пустым. Если архив
  создан, но бюджет превышен и автоматически удалять больше нечего,
  `warnings` содержит причину, а команда завершается успешно.
- `skipped` = `false` для созданного архива; при `skipped=true` поля `archive`,
  `size` и `sha256` отсутствуют, а `reason` равен `not_running`.

При `--if-running` и остановленном сервере:

```json
{
  "ok": true,
  "data": {
    "skipped": true,
    "reason": "not_running",
    "pruned": [],
    "warnings": []
  }
}
```

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
после успешного импорта.

### `mods deploy`

```json
{
  "ok": true,
  "data": {
    "archive": "mods-2026-10-22_12-00-00.tar.gz",
    "sha256": "abc...",
    "previous": "mods-2026-10-21_12-00-00.tar.gz",
    "state_backup": "state-predeploy-2026-10-22_12-00-00.tar.gz",
    "restarted": true
  }
}
```

`previous` может быть `null`, если это первый деплой. `state_backup` — имя
созданного перед деплоем state-архива; он создаётся всегда. `restarted`
равен `true`, только если сервер работал до операции и был запущен после неё;
если сервер был остановлен или указан `--no-restart`, поле равно `false`.

### `state deploy`

```json
{
  "ok": true,
  "data": {
    "archive": "state-manual-2026-10-22_12-00-00.tar.gz",
    "mods_archive": "mods-2026-10-22_12-00-00.tar.gz",
    "predeploy_backup": "state-predeploy-2026-10-22_12-00-00.tar.gz",
    "restarted": true
  }
}
```

`mods_archive` — архив модов из `current.txt` внутри state-архива. Он не может
быть `null`: `current.txt` обязателен. Перед восстановлением ops всегда делает
`state-predeploy-*`. `restarted` равен `true`, только если сервер работал до
операции и был запущен после неё; при `--no-restart` или изначально
остановленном сервере — `false`. `--latest` выбирает самый новый `state-*` или
`state-manual-*`, но не `state-predeploy-*`; последние восстанавливаются только
явным `--archive`.

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
- дубликаты имён и неоднозначные регистры отклоняются;
- `mods-init-*` — базовый архив без пользовательских плагинов и скриптов; он
  содержит `plugins/requiredDataFiles.json` с тремя базовыми ESM и пустой
  `scripts/`.

### 6.2. State

Только вложенная форма. Плоские `cell/`, `world/`, `map/`, `recordstore/`,
`custom/` в корне архива не принимаются.

```text
state.tar.gz
├── players/
├── world/
│   ├── cell/
│   ├── world/
│   ├── map/
│   ├── recordstore/
│   └── custom/
├── requiredDataFiles.json
└── current.txt
```

Обязательны все перечисленные каталоги и оба файла. Пустой `players/` — это
каталог без файлов, а не отсутствующий каталог. Любое отклонение —
`archive_invalid` с указанием отсутствующей записи.

Правила:

- `players/` и `world/<name>/` — ровно эти пути, без дополнительной вложенности;
- `requiredDataFiles.json` и `current.txt` — метаданные мод-пака, с которым
  сделан state;
- `current.txt` обязателен и должен быть в каноническом формате §6.4;
- `current.txt` должен указывать на существующий архив в `backups/mods/`, иначе
  `state deploy` возвращает `archive_invalid`;
- ops не подменяет моды: если `current.txt` из state не совпадает с текущим
  `backups/mods/current.txt`, сначала нужно задеплоить нужный мод-пак;
- `state deploy` не удаляет текущие `players/` и `world/` до успешной
  распаковки в staging.

### 6.3. `requiredDataFiles.json`

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
- дубликаты ключей запрещены;
- для модов каждый плагин из JSON должен быть в `plugins/`, а каждый плагин в
  `plugins/` — в JSON (кроме самого `requiredDataFiles.json`); три базовых ESM
  (`Morrowind.esm`, `Tribunal.esm`, `Bloodmoon.esm`) в архив не входят и
  проверяются клиентом по установленному Morrowind;
- для state JSON описывает мод-пак, но сами плагины лежат в `backups/mods/`,
  а не в state-архиве.

### 6.4. `current.txt`

Единственный формат:

```text
<sha256> <filename>\n
```

- `sha256` — 64 hex-символа в нижнем регистре;
- `filename` — базовое имя архива модов, `mods-*.tar.gz` или
  `mods-init-*.tar.gz`;
- ровно один пробел между полями, перевод строки в конце;
- другие форматы, включая строку только с `sha256`, CRLF, лишние пробелы и
  пустые строки, отклоняются с `archive_invalid`;
- `sha256` должен совпадать с содержимым файла; несовпадение —
  `archive_invalid`;
- новые записи всегда создаются только в этом формате.

### 6.5. Лимиты архивов

| Параметр | Моды | State |
|---|---|---|
| Максимальный размер сжатого архива | 512 MiB | 512 MiB |
| Максимальный размер после распаковки | 1 GiB | 1 GiB |
| Максимальное число записей tar | 50 000 | 50 000 |
| Максимальный размер upload | 512 MiB | — |
| Разрешённые типы записей | regular file, directory | regular file, directory |

Symlink, hardlink, device, FIFO, абсолютные пути, `..`, NUL и дубликаты
отклоняются с `archive_invalid` или `unsafe_path`. Gzip-мусор после tar и
повреждённый gzip — `archive_invalid`.

### 6.6. Имена архивов

Все имена — UTC, без пробелов:

| Тип | Шаблон | Кто создаёт |
|---|---|---|
| Моды | `mods-YYYY-MM-DD_HH-MM-SS.tar.gz` | `mods import` |
| Базовые моды | `mods-init-YYYY-MM-DD_HH-MM-SS.tar.gz` | `bootstrap.sh` |
| State по таймеру | `state-YYYY-MM-DD_HH-MM-SS.tar.gz` | `tes3mp-easy-backup.timer` |
| State вручную | `state-manual-YYYY-MM-DD_HH-MM-SS.tar.gz` | панель / `--manual` |
| State перед деплоем | `state-predeploy-YYYY-MM-DD_HH-MM-SS.tar.gz` | `mods deploy` / `state deploy` |

Если файл с таким именем уже существует, к имени перед `.tar.gz`
добавляется `-01`, `-02` и так далее. Имена вне шаблонов не принимаются как
state/mods-архивы.

### 6.7. Retention

Retention применяется после каждого успешного `backups create`, `mods import`
и создания `state-predeploy-*`. Время — UTC.

| Класс | Возраст | Что хранится |
|---|---|---|
| Горячий | 0–7 дней | все автоматические `state-<timestamp>` (каждые 5 минут) |
| Дневной | 7–365 дней | последний автоматический `state-<timestamp>` каждого дня |
| Недельный | 1–3 года | последний автоматический `state-<timestamp>` каждой ISO-недели |
| Месячный | старше 3 лет | последний автоматический `state-<timestamp>` каждого месяца, **навсегда** |
| Ручной | любой | последние 5 `state-manual-*` |
| Перед деплоем | любой | последние 2 `state-predeploy-*` |
| Моды | любой | последние 5 `mods-<timestamp>` (кроме `mods-init-*`) |
| Базовые моды | любой | последние 2 `mods-init-*` |

«Последний в bucket» — самый новый архив внутри дня/недели/месяца. При обычном
retention в каждом bucket остаётся этот представитель; при pressure pruning
дневные и недельные представители могут удаляться (самые старые первыми), но
месячный представитель не удаляется автоматически никогда.

Всегда защищены:

- самый новый state-архив любого класса;
- архив, на который указывает `backups/mods/current.txt`, и сам `current.txt`;
- последний месячный архив каждого месяца.

Если места не хватает, включается pressure pruning в порядке:

1. автоматические `state-<timestamp>` старше 24 часов, не являющиеся последними
   за день;
2. дневные представители (самые старые первыми);
3. недельные представители (самые старые первыми).

Месячные не удаляются даже при давлении. Если после этого бюджет всё ещё
превышен, архив всё равно создаётся (если есть физическое место), а
`warnings` содержит причину. `doctor` показывает `warn`, пока бюджет не
вернётся в норму.

Бюджеты:

| Каталог | Бюджет |
|---|---|
| `backups/state` | 1 GiB |
| `backups/mods` | 512 MiB |
| Свободный резерв на файловой системе | не меньше 512 MiB |

`doctor` предупреждает, если свободно меньше 1 GiB, и считает ошибкой, если
свободно меньше 512 MiB. Превышение бюджета каталога — `warn`, а не повод
удалять месячные архивы.

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

- `name` — непустое отображаемое имя; панель подставляет `TES3MP`, если
  `hostname` в конфиге пуст;
- `status` — те же значения, что у `ops status`;
- `mods_sha256` — `null`, если текущего архива модов нет;
- `address` — публичный адрес из конфигурации панели, не обязательно IP
  клиента;
- `Cache-Control: no-store`.

### `GET /api/v1/mods/current`

```json
{
  "name": "mods-2026-10-22_12-00-00.tar.gz",
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

`no_mods` также возвращается, если `current.txt` отсутствует или не проходит
проверку. В поддерживаемой установке `bootstrap` всегда создаёт `current.txt`,
поэтому клиент трактует такой ответ как ошибку установки, а не как валидный
сервер без модов.

### `GET /api/v1/mods/current/archive`

- `200 OK`;
- `Content-Type: application/gzip`;
- `Content-Length`;
- `Content-Disposition: attachment; filename="<name>"`;
- `ETag: "<sha256>"`;
- `Cache-Control: public, max-age=0, must-revalidate`;
- поддержка `If-None-Match` → `304 Not Modified`;
- при отсутствии архива или невалидном `current.txt` — `404` с `no_mods`.

### `GET /healthz`

`200 OK`, тело `ok\n`, без проверки других сервисов. Это liveness, а не
readiness.

### Ошибки публичного API

```json
{
  "error": {
    "code": "not_found",
    "message": "Не найдено"
  }
}
```

Коды: `not_found`, `no_mods`, `rate_limited`, `service_unavailable`,
`internal`. HTTP-статусы: `404`, `404`, `429`, `503`, `500`.

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
| `start`, `stop`, `restart`, `staff set`, `config set` | 2 мин |
| `mods import`, `mods deploy`, `state deploy`, `backups create` | 10 мин |

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
  `mods-init-*` из API (§7). `name` проверяется по §6.6, `sha256` — 64
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
  её, **Retry** повторяет после ошибки или отмены. `skip_fonts` остаётся
  legacy-полем.
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

## 10. Старые форматы не принимаются

- State-архив только вложенный: `players/`, `world/<пять каталогов>/`,
  `requiredDataFiles.json`, `current.txt`.
- `current.txt` только `<sha256> <filename>`; строка только с хешем —
  `archive_invalid`.
- Имена вне шаблонов §6.6 не являются валидными state/mods-архивами.
- Старые URL `/list-backups/...` и `/download/...` не поддерживаются.
- Старые клиентские конфиги не мигрируются.
- Если при обновлении найден старый формат, `doctor` возвращает `error`, а
  `state deploy`/`mods deploy` — `archive_invalid`. Автоматической конвертации
  нет; нужен новый архив, снятый этой версией.

## 11. Изменение контракта

1. Обновить этот документ.
2. Обновить фикстуры в `testdata/` (или аналоге).
3. Обновить тесты ops/panel/client.
4. Обновить `07-testing.md`, если появляется новый класс проверок.
5. Только после этого менять реализацию.
