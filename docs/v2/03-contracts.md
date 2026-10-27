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
| 11 | `plugin_in_use` | Из мод-пака удаляется плагин, на который ссылается точка отката | 409 |
| 12 | `archive_in_use` | Архив защищён от удаления: закреплён, активен или является базой точки отката | 409 |

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
      {"name": "disk", "status": "warn", "message": "свободно меньше 512 MiB", "details": {"free": 402653184, "level": "warn"}}
    ]
  }
}
```

`status`: `ok`, `warn`, `error`. Если `status` = `error`, процесс завершается
кодом `10`. Это исключение из формы ошибки: `doctor` не добавляет объект
`error`, потому что диагностика уже в `data.checks`; HTTP 503 панель выставляет
по коду выхода. Имена проверок и их смысл — в `04-ops.md` §6.

`details` — необязательный объект у проверки. Для `disk` он содержит `free`
(свободные байты) и `level` маркера guard (`ok`, `warn`, `stop`). Остальные
проверки `details` не отдают, панель не полагается на их поля. Потребление
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

- `unit` — ровно `tes3mp.service`, `tes3mp-easy-backup.timer` или
  `tes3mp-easy-guard.timer`.
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

CLI: `backups list --type mods|configs [--limit 200] [--before <name>] [--sha256]`.

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

- `current` — имя из `backups/mods/current.txt`; для `configs` всегда `null`.
- `type=mods` отдаёт `mods-*` и `mods-init-*`, `type=configs` — `configs-*`.
  Снапшоты и чекпоинты эта команда не отдаёт: у них свои команды.
- `items` отсортированы по `mtime`, новые сверху; при равном `mtime` — по имени
  по убыванию.
- `limit` — 1..500, по умолчанию 200. `before` — имя архива, курсор
  «строго старше этого имени»; имя проверяется как безопасное.
- `next_before` — имя последнего элемента, если `len(items) == limit`, иначе
  `null`. Клиент передаёт его в `--before`, пока не получит `null`.
- `sha256` в списке нет по умолчанию: это дорого. Флаг `--sha256` добавляет
  поле `sha256` только к элементам текущей страницы.
- Битый `current.txt` — `archive_invalid`, а не молчаливый `null`.

### `snapshot create`

CLI: `snapshot create [--label <text>] [--reason manual|mods|base] [--dry-run]`.

```json
{
  "ok": true,
  "data": {
    "archive": "snapshot-2026-10-27_16-40-00.tar.gz",
    "size": 201326592,
    "sha256": "abc...",
    "reason": "manual",
    "label": "перед квестами",
    "mods_archive": "mods-2026-10-27_16-40-00.tar.gz",
    "pruned": [],
    "warnings": []
  }
}
```

- Снапшот всегда полный: `state/`, `mods/`, `configs/` (§6.2). `reason=mods`
  ставит `mods deploy`, `reason=manual` — панель и CLI,
  `reason=base` — ленивая база из `checkpoint create`, когда подходящего
  снапшота нет; вручную этот reason не нужен.
- `mods_archive` — архив из `backups/mods/`, из которого взяты моды. Если
  живые `mods/` расходятся с `current.txt`, ops сначала создаёт
  `mods-<timestamp>` из живого каталога и обновляет `current.txt`.
- `label` — необязательная строка 1..64 символа; без флага поле `null`.
- `pruned` и `warnings` — имена удалённых retention-архивов и предупреждения
  по диску. `--dry-run` печатает план и файла не создаёт.

### `snapshot list`

CLI: `snapshot list [--limit 200] [--before <name>]`.

```json
{
  "ok": true,
  "data": {
    "items": [
      {
        "name": "snapshot-2026-10-27_16-40-00.tar.gz",
        "size": 201326592,
        "mtime": "2026-10-27T16:40:00Z",
        "reason": "manual",
        "label": "перед квестами",
        "mods_sha256": "abc...",
        "state_sha256": "def...",
        "used_by": 12,
        "protected": true
      }
    ],
    "next_before": null
  }
}
```

- `used_by` — число живых чекпоинтов, чей `base.name` указывает на снапшот;
- `protected` — `true`, если снапшот нельзя удалить retention-ом: на него
  ссылается хотя бы один чекпоинт или он самый новый.

### `snapshot restore`

CLI: `snapshot restore (--latest|--archive <name>) [--no-restart]`.

```json
{
  "ok": true,
  "data": {
    "archive": "snapshot-2026-10-27_16-40-00.tar.gz",
    "mods_archive": "mods-2026-10-27_16-40-00.tar.gz",
    "base_switched": true,
    "predeploy_checkpoint": "checkpoint-predeploy-2026-10-27_17-02-11.tar.gz",
    "restarted": true
  }
}
```

- `base_switched` = `true`, если активный мод-пак отличался и был подменён;
- `mods_archive` — архив в `backups/mods/`, соответствующий модам снапшота; при
  необходимости ops создаёт его из слоя `mods/` и обновляет `current.txt`, иначе
  будущие чекпоинты не смогут ссылаться на моды;
- `predeploy_checkpoint` создаётся всегда до остановки сервера;
- `--latest` берёт самый новый снапшот любого `reason`.

### `checkpoint create`

CLI: `checkpoint create [--label <text>] [--if-running] [--dry-run]`.

```json
{
  "ok": true,
  "data": {
    "archive": "checkpoint-2026-10-27_16-45-00.tar.gz",
    "size": 131072,
    "sha256": "abc...",
    "label": null,
    "reason": "manual",
    "base": {
      "name": "snapshot-2026-10-27_16-40-00.tar.gz",
      "sha256": "abc...",
      "mods_sha256": "def..."
    },
    "players": 12,
    "skipped": false,
    "pruned": [],
    "warnings": []
  }
}
```

- `base` — активный снапшот: самый новый, чей `mods_sha256` совпадает с
  `current.txt`. Если подходящего нет, ops сначала создаёт `snapshot-*` с
  `reason=base` и ссылается на него.
- `players` — число файлов в `players/` на момент снятия.
- `--dry-run` печатает план (имя архива, база, `warnings`) и не создаёт ни
  чекпоинт, ни ленивый `snapshot-*` с `reason=base`.
- `--if-running` и остановленный сервер:
  `{"skipped": true, "reason": "not_running", "pruned": [], "warnings": []}`;
  поля `archive`, `size`, `sha256`, `base` отсутствуют.
- Чекпоинт снимается с живого сервера и не транзакционен: файл, который TES3MP
  пишет в этот момент, может попасть в архив частично. При 15-минутном
  интервале и типичных ~100 KB это приемлемый recovery point.

### `checkpoint list`

CLI: `checkpoint list [--limit 200] [--before <name>]`.

```json
{
  "ok": true,
  "data": {
    "items": [
      {
        "name": "checkpoint-2026-10-27_16-45-00.tar.gz",
        "size": 131072,
        "mtime": "2026-10-27T16:45:00Z",
        "label": null,
        "reason": "timer",
        "pinned": false,
        "players": 12,
        "base": {
          "name": "snapshot-2026-10-27_16-40-00.tar.gz",
          "sha256": "abc...",
          "available": true
        }
      }
    ],
    "next_before": null
  }
}
```

- `available=false` — базового снапшота нет (его удалил retention или
  `snapshot rm --force`): обычный `checkpoint restore` вернёт `not_found`, а с
  `--accept-mod-mismatch` мир восстанавливается на текущих модах
  (`base_deployed=false`, `base_missing=true`);
- `reason` — `timer`, `manual` или `predeploy`; панель показывает `predeploy`
  отдельной группой и не предлагает `--latest` для неё;
- `pinned` читается из sidecar-файла `<name>.pin`, а не из архива: архивы
  неизменяемы.

### `snapshot rm` / `checkpoint rm` / `backups rm`

CLI:

```text
snapshot rm <name>... [--force] [--dry-run]
checkpoint rm <name>... [--force] [--dry-run]
backups rm --type mods|configs <name>... [--force] [--dry-run]
```

```json
{
  "ok": true,
  "data": {
    "removed": ["checkpoint-2026-09-01_00-00-00.tar.gz"],
    "skipped": [],
    "freed": 131072,
    "warnings": []
  }
}
```

- имя проверяется как безопасное (`unsafe_path`, exit 8) и как архив своего
  типа (`invalid_argument`, exit 2); `checkpoint rm` принимает и
  `checkpoint-predeploy-*` (обе разновидности лежат в `backups/checkpoints`),
  отсутствующий файл — `not_found`, exit 3;
- без `--force` защищённые имена не удаляются: закреплённый чекпоинт, архив из
  `current.txt`, последний снапшот и база, на которую ссылаются чекпоинты, дают
  `archive_in_use`, exit 12, и отклоняется вся пачка — частичного удаления нет;
- с `--force` удаляется закреплённый чекпоинт вместе с `.pin` и база, на которую
  ссылаются точки отката (если это не последний снапшот): чекпоинты остаются,
  получают `available=false`, а `warnings` перечисляет их имена;
- архив из `current.txt` и последний снапшот не удаляются даже с `--force`: они
  попадают в `skipped` с причиной `current` или `last_snapshot`. Если после
  фильтра удалять нечего — `archive_in_use`, exit 12; если что-то удалено —
  exit 0;
- `--dry-run` возвращает `removed`/`freed` как план, файлы не трогает;
- lock берётся, удаление — `os.Remove` + `fsync` каталога; retention после `rm`
  не запускается.

### `checkpoint pin` / `checkpoint unpin`

CLI: `checkpoint pin <name>` / `checkpoint unpin <name>`.

```json
{
  "ok": true,
  "data": {
    "archive": "checkpoint-2026-10-27_16-45-00.tar.gz",
    "pinned": true
  }
}
```

`pin` создаёт `backups/checkpoints/<name>.pin` (JSON `{"pinned_at": "..."}`),
`unpin` удаляет его. Архив не переписывается. Закреплённые чекпоинты retention
не удаляет никогда.

### `checkpoint restore`

CLI: `checkpoint restore (--latest|--archive <name>) [--accept-mod-mismatch]
[--no-restart]`.

```json
{
  "ok": true,
  "data": {
    "archive": "checkpoint-2026-10-27_16-45-00.tar.gz",
    "base": {
      "name": "snapshot-2026-10-27_16-40-00.tar.gz",
      "sha256": "abc..."
    },
    "base_deployed": true,
    "base_missing": false,
    "mods_archive": "mods-2026-10-27_16-40-00.tar.gz",
    "missing_plugins": [],
    "predeploy_checkpoint": "checkpoint-predeploy-2026-10-27_17-02-11.tar.gz",
    "restarted": true
  }
}
```

- `base_deployed=false`, если активный мод-пак уже совпадает с базой; иначе
  моды базы деплоятся так же, как в `snapshot restore`;
- `base_missing=true` только с `--accept-mod-mismatch`, когда база удалена:
  `base` и `mods_archive` — `null`, активный мод-пак не меняется, а
  `missing_plugins` перечисляет плагины из `requiredDataFiles.json` чекпоинта,
  которых нет в текущем паке;
- без флага отсутствующая база — `not_found`, exit 3, данные не меняются;
- `configs/` не трогается никогда: конфиги принадлежат админу, а не точке
  отката;
- `--latest` берёт самый новый чекпоинт с `available=true` и не рассматривает
  `checkpoint-predeploy-*`: это внутренние точки отката, их восстанавливают
  только явным `--archive`;
- `--accept-mod-mismatch` не меняет `current.txt`: база следующих чекпоинтов
  выбирается по текущему паку, а если подходящего снапшота нет — первый
  `checkpoint create` создаст `snapshot-*` с `reason=base` (нужно ≥512 MiB
  свободно, иначе `no_space`).

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
    "archive": "mods-2026-10-27_16-40-00.tar.gz",
    "sha256": "abc...",
    "previous": "mods-2026-10-21_12-00-00.tar.gz",
    "changes": {
      "added": ["new-quests.esp"],
      "removed": [],
      "reordered": false,
      "crc_changed": ["balance.esp"]
    },
    "predeploy_checkpoint": "checkpoint-predeploy-2026-10-27_16-40-00.tar.gz",
    "snapshot": "snapshot-2026-10-27_16-40-00.tar.gz",
    "restarted": true
  }
}
```

- `previous` может быть `null`, если это первый деплой;
- `changes` — diff плагинов и скриптов относительно активного мод-пака;
  `reordered` = `true`, если изменился только порядок загрузки; `crc_changed` —
  плагины с тем же именем и другим CRC32;
- `predeploy_checkpoint` создаётся всегда. `snapshot` с `reason=mods`
  создаётся после успешной подмены: деплой меняет базу мира;
- если `changes.removed` не пуст и хотя бы один удаляемый плагин входит в базу
  живого чекпоинта, без `--allow-plugin-removal` команда завершается кодом 11
  `plugin_in_use` и ничего не меняет;
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
config set --file server|config|banlist (--values <json>|--key <k> --value <v>) [--dry-run]
```

```json
{
  "ok": true,
  "data": {
    "file": "config",
    "changed": [
      {"key": "loginTime", "value": 60},
      {"key": "difficulty", "value": -20}
    ],
    "restart_required": true,
    "configs_archive": "configs-2026-10-27_16-40-00.tar.gz"
  }
}
```

- `--values` — JSON-объект `{"<key>": <value>}`; все ключи проверяются до
  записи, при ошибке не меняется ни один (всё или ничего);
- `--key`/`--value` остаются для одной настройки и дают `changed` из одного
  элемента;
- `changed` отсортирован по `key`, `value` уже приведён к типу;
- `configs_archive` — архив истории конфигов (§6.6), созданный после записи.
  Он есть только для `--file config` и только вне `--dry-run`, иначе `null`;
- `restart_required` всегда `true`: сервер не перезапускается.

### `config restore`

CLI: `config restore (--latest|--archive <name>)`.

```json
{
  "ok": true,
  "data": {
    "archive": "configs-2026-10-27_16-40-00.tar.gz",
    "files": [
      "config.lua",
      "tes3mp-server-default.cfg",
      "banlist.json",
      "customScripts.lua"
    ],
    "predeploy_archive": "configs-2026-10-27_17-02-11.tar.gz",
    "restart_required": true
  }
}
```

- перед записью текущие конфиги архивируются (`predeploy_archive`), и каждый
  файл получает `.bak-<timestamp>`;
- `files` — ровно записи архива в фиксированном порядке;
- `customScripts.lua` восстанавливается из архива, но `mods deploy` и `doctor`
  пересоздают его из `mods/scripts/`; расхождение — `warn`, не `error`;
- команда сервер не перезапускает: `restart_required` всегда `true`, панель
  показывает баннер и кнопку рестарта.

## 5. Ops: lock и атомарность

- Все мутации берут эксклюзивный lock `.locks/ops.lock` через `flock`.
- `--lock-timeout` по умолчанию `30s`; при истечении — код `4`, `locked`.
- Читающие команды (`doctor`, `status`, `backups list`, `logs`) lock не берут.
- Внутренние операции, которым нужен уже взятый lock (чекпоинт внутри
  `mods deploy`, снапшот внутри `checkpoint create`), не вызывают публичную
  команду повторно. В коде это отдельные функции `createCheckpointLocked` и
  `createSnapshotLocked`, чтобы не было deadlock.
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

### 6.2. Snapshot

Снапшот — самодостаточная база: мир, моды и конфиги в одном архиве. Только
вложенная форма; плоские `players/`, `world/`, `plugins/`, `configs/` в корне
не принимаются.

```text
snapshot.tar.gz
├── manifest.json
├── state/
│   ├── players/
│   ├── world/
│   │   ├── cell/
│   │   ├── world/
│   │   ├── map/
│   │   ├── recordstore/
│   │   └── custom/
│   └── requiredDataFiles.json
├── mods/
│   ├── plugins/
│   │   ├── mod.esp
│   │   └── requiredDataFiles.json
│   └── scripts/
└── configs/
    ├── config.lua
    ├── tes3mp-server-default.cfg
    ├── banlist.json
    └── customScripts.lua
```

```json
{
  "format": "tes3mp-easy-snapshot",
  "version": 1,
  "created_at": "2026-10-27T16:40:00Z",
  "reason": "manual",
  "label": "",
  "panel_version": "0.1.15",
  "mods_sha256": "abc...",
  "state_sha256": "def...",
  "configs_sha256": "012..."
}
```

- `manifest.json` — первая запись архива: панель читает только её;
- `reason`: `manual`, `mods`, `base`; `label` — 0..64 символа, `""` без метки;
- digest слоя считается от канонического листинга, а не от байтов архива:

```text
digest(layer) = sha256( layer "\n" + Σ path "\0" size "\0" file_sha256 "\n" )
```

записи сортируются по `path` (байтовый порядок, `/` внутри пути, без ведущего
`/`), `size` — десятичное, `file_sha256` — 64 hex в нижнем регистре. Digest не
зависит от порядка обхода, mtime и степени сжатия.

Правила:

- `state/` — только вложенная форма, `players/` и `world/<name>/` ровно эти
  пути;
- `mods/` подчиняется §6.1 целиком, но лежит под префиксом `mods/`;
- набор плагинов, их CRC и порядок в `plugins/requiredDataFiles.json` — это
  порядок загрузки; расхождение с файлами слоя — `archive_invalid`;
- `configs/` — ровно четыре файла из списка, не больше и не меньше;
- `current.txt` внутри снапшота нет: активный мод-пак определяется по слою
  `mods/` и `mods_sha256`;
- при восстановлении ops заводит (или переиспользует) `mods-*` в
  `backups/mods/`, чтобы `current.txt` оставался валидным;
- снапшот неизменяем: он никогда не дописывается и не пересобирается.


### 6.3. Checkpoint

Чекпоинт — только прогресс мира: без модов и без конфигов.

```text
checkpoint.tar.gz
├── manifest.json
├── players/
├── world/
│   ├── cell/
│   ├── world/
│   ├── map/
│   ├── recordstore/
│   └── custom/
└── requiredDataFiles.json
```

```json
{
  "format": "tes3mp-easy-checkpoint",
  "version": 1,
  "created_at": "2026-10-27T16:45:00Z",
  "reason": "timer",
  "label": "",
  "base": {
    "name": "snapshot-2026-10-27_16-40-00.tar.gz",
    "sha256": "abc...",
    "mods_sha256": "def...",
    "state_sha256": "012..."
  },
  "players": 12,
  "world_sha256": "345..."
}
```

- `reason`: `timer`, `manual`, `predeploy`;
- `base` — снапшот, продолжением которого является чекпоинт. `sha256`
  проверяется при восстановлении, `mods_sha256` показывает, нужно ли деплоить
  моды, `state_sha256` — от какой версии мира снят чекпоинт (сам мир лежит в
  чекпоинте целиком, это не дельта);
- `world_sha256` — digest слоя `players/` + `world/` + `requiredDataFiles.json`
  по алгоритму §6.2;
- `players` — число файлов в `players/`;
- чекпоинт не содержит `mods/`, `configs/` и `current.txt`; восстановление без
  доступной базы — `not_found`, кроме явного `--accept-mod-mismatch` (§4);
- закрепление не меняет архив: оно живёт в sidecar-файле
  `backups/checkpoints/<name>.pin` с `{"pinned_at": "..."}`.

### 6.4. `requiredDataFiles.json`

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
- в `snapshot-*` JSON лежит в `mods/plugins/requiredDataFiles.json` и обязан
  совпадать с составом `mods/plugins/` (кроме трёх базовых ESM);
- в `checkpoint-*` JSON лежит в корне и описывает мод-пак базы: тот же набор,
  что в `base.mods_sha256`; расхождение — `archive_invalid`;
- отдельного state-архива с JSON больше нет, `current.txt` в снапшотах и
  чекпоинтах не используется.

### 6.5. `current.txt`

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

### 6.6. Configs

История конфигов: маленький архив, который создаётся при каждом изменении
`config.lua` и перед `config restore`.

```text
configs.tar.gz
├── manifest.json
├── config.lua
├── tes3mp-server-default.cfg
├── banlist.json
└── customScripts.lua
```

```json
{
  "format": "tes3mp-easy-configs",
  "version": 1,
  "created_at": "2026-10-27T16:40:00Z",
  "reason": "config_set",
  "label": "",
  "files": {
    "config.lua": "abc...",
    "tes3mp-server-default.cfg": "def...",
    "banlist.json": "012...",
    "customScripts.lua": "345..."
  }
}
```

- `reason`: `config_set`, `config_restore`, `predeploy`;
- `files` — sha256 каждого файла, ровно четыре ключа;
- архив не содержит `players/`, `world/`, `mods/`;
- снапшот (§6.2) содержит те же четыре файла в `configs/`, но не заменяет этот
  архив: история конфигов не должна стоить копии модов.

### 6.7. Лимиты архивов

| Параметр | Моды | Snapshot | Checkpoint | Configs |
|---|---|---|---|---|
| Максимальный размер сжатого архива | 512 MiB | 1 GiB | 256 MiB | 1 MiB |
| Максимальный размер после распаковки | 1 GiB | 2 GiB | 512 MiB | 4 MiB |
| Максимальное число записей tar | 50 000 | 100 000 | 50 000 | 16 |
| Максимальный размер upload | 512 MiB | — | — | — |
| Разрешённые типы записей | regular file, directory | то же | то же | то же |

Symlink, hardlink, device, FIFO, абсолютные пути, `..`, NUL и дубликаты
отклоняются с `archive_invalid` или `unsafe_path`. Gzip-мусор после tar и
повреждённый gzip — `archive_invalid`.

### 6.8. Имена архивов

Все имена — UTC, без пробелов:

| Тип | Шаблон | Кто создаёт |
|---|---|---|
| Моды | `mods-YYYY-MM-DD_HH-MM-SS.tar.gz` | `mods import` |
| Базовые моды | `mods-init-YYYY-MM-DD_HH-MM-SS.tar.gz` | `bootstrap.sh` |
| Snapshot | `snapshot-YYYY-MM-DD_HH-MM-SS.tar.gz` | `snapshot create`, `mods deploy` |
| Checkpoint | `checkpoint-YYYY-MM-DD_HH-MM-SS.tar.gz` | `tes3mp-easy-backup.timer`, `checkpoint create` |
| Checkpoint перед восстановлением | `checkpoint-predeploy-YYYY-MM-DD_HH-MM-SS.tar.gz` | `mods deploy`, `snapshot restore`, `checkpoint restore` |
| Configs | `configs-YYYY-MM-DD_HH-MM-SS.tar.gz` | `config set --file config`, `config restore` |

Если файл с таким именем уже существует, к имени перед `.tar.gz` добавляется
`-01`, `-02` и так далее. Имена вне шаблонов не принимаются как архивы этого
типа. Pin-файлы `<name>.pin` рядом с чекпоинтами архивами не являются и в
списки не попадают.

### 6.9. Retention

Retention применяется после каждого успешного `snapshot create`,
`checkpoint create`, `config set --file config`, `config restore`,
`mods import` и создания `checkpoint-predeploy-*`. Время — UTC. Ручное
удаление — `rm` из §4; retention после него не запускается.

Чекпоинты делятся на классы по возрасту; представитель бакета выбирается по
имени архива, без чтения содержимого:

| Класс | Возраст | Что хранится |
|---|---|---|
| Горячий | 0–7 дней | все `checkpoint-<timestamp>` (каждые 15 минут) |
| Дневной | 7–365 дней | самый новый чекпоинт каждой UTC-даты |
| Недельный | 1–3 года | самый новый чекпоинт каждой ISO-недели |
| Месячный | старше 3 лет | самый новый чекпоинт каждого месяца, **навсегда** |
| Закреплённый | любой | все, у кого есть `<name>.pin`, **навсегда** |
| Predeploy | любой | последние 2 `checkpoint-predeploy-*` |

Дневной представитель фиксируется, когда сутки закончились, и больше не
меняется: чекпоинты следующих суток его не вытесняют. «Последний чекпоинт дня»
означает последний по времени архив этой UTC-даты.

Прочие классы:

| Класс | Что хранится |
|---|---|
| Snapshot, автоматический | последние 2 с `reason=mods` или `reason=base` |
| Snapshot, ручной | последние 5 с `reason=manual` |
| Snapshot, защищённый | любой, на который ссылается хранимый чекпоинт |
| Configs | последние 20 `configs-*` |
| Моды | последние 5 `mods-<timestamp>` (кроме `mods-init-*`) |
| Базовые моды | последние 2 `mods-init-*` |

Автоматически не удаляются никогда:

- самый новый чекпоинт и самый новый снапшот;
- снапшот, на который ссылается хотя бы один хранимый чекпоинт;
- закреплённые чекпоинты (`<name>.pin`);
- архив, на который указывает `backups/mods/current.txt`, и сам `current.txt`;
- самая новая запись `configs-*`.

Снапшот-база живёт столько же, сколько ссылающиеся на него чекпоинты, включая
дневные, недельные и месячные: мод-пак, снятый год назад, остаётся
восстановимым. Бюджет `backups/snapshots` при этом конечен. Если он превышен,
защита снимается только с самой старой базы (при равенстве — с той, на которую
ссылается меньше чекпоинтов). Её чекпоинты не удаляются, получают
`available=false` и восстанавливаются только с `--accept-mod-mismatch`;
`warnings` перечисляет такие чекпоинты, `doctor` показывает `warn`.

Если места не хватает, включается pressure pruning в порядке:

1. `checkpoint-predeploy-*` сверх 2, самые старые;
2. автоматические снапшоты с `reason=base` сверх 1, не защищённые
   чекпоинтами;
3. незакреплённые чекпоинты вне горячего окна, самые старые: сначала не
   представители бакета, затем дневные, затем недельные;
4. самые старые снапшоты-базы, защита которых снимается последней; их
   чекпоинты остаются с `available=false`.

Ручные снапшоты, месячные представители и закреплённые чекпоинты pressure
pruning не удаляет. Если после этого бюджет всё ещё превышен, архив всё равно
создаётся (при наличии физического места), а `warnings` содержит причину.
`doctor` показывает `warn`, пока бюджет не вернётся в норму.

Бюджеты:

| Каталог | Бюджет |
|---|---|
| `backups/snapshots` | 1.5 GiB |
| `backups/checkpoints` | 256 MiB |
| `backups/configs` | 32 MiB |
| `backups/mods` | 512 MiB |
| Свободный резерв на файловой системе | не меньше 512 MiB |

Оценка объёма: 672 горячих чекпоинта за 7 дней плюс 358 дневных за год плюс
недельные и месячные ≈ 1100 архивов × ~150 КБ ≈ 165 МБ. Бюджет
`backups/checkpoints` не достигается даже за три года, поэтому дневную историю
можно хранить бессрочно.

Guard (`04-ops.md` §6) останавливает сервер, если свободного места меньше
256 MiB, и предупреждает при 512 MiB. `doctor` даёт `warn` при свободных
256–512 MiB и `error` меньше 256 MiB. Превышение бюджета каталога — `warn`, а
не повод удалять закреплённые точки.

## 7. Публичный HTTP API

Базовый URL — `https://<panel>/api/v1`. Пароль сервера здесь не отдаётся
никогда. Публичный список мира и скачивание пакета включаются флагом
`public_state_download` в конфиге панели; по умолчанию флаг выключен. Тот же
флаг управляет публичным списком чекпоинтов (`/backups` без `type` и
`/backups?type=checkpoints`) и маршрутом `/backups/checkpoints/<name>`: при
выключенном флаге они отвечают `404`, а `/backups?type=mods` и архивы модов
остаются доступными.

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

### `GET /api/v1/checkpoints`

```json
{
  "items": [
    {
      "name": "checkpoint-2026-10-27_16-45-00.tar.gz",
      "created_at": "2026-10-27T16:45:00Z",
      "label": null,
      "size": 131072,
      "sha256": "abc...",
      "players": 12,
      "base": {
        "name": "snapshot-2026-10-27_16-40-00.tar.gz",
        "mods_sha256": "def..."
      },
      "url": "/api/v1/checkpoints/checkpoint-2026-10-27_16-45-00.tar.gz/download"
    }
  ],
  "next_before": null
}
```

- по умолчанию 50 новейших; `?before=<name>&limit=1..200` — как
  `checkpoint list`;
- `label` — `null`, если метки нет; `base.mods_sha256` нужен клиенту, чтобы
  сравнить со своим мод-паком;
- `Cache-Control: no-store`;
- вместе с `.../download` доступен только при `public_state_download = true`,
  иначе `404` с кодом `disabled`.

### `GET /api/v1/checkpoints/<name>/download`

Собирает переносимый пакет мира на лету, без временного файла:

```text
tes3mp-server-<checkpoint>.tar.gz
├── manifest.json
├── world/
│   ├── players/
│   ├── world/{cell,world,map,recordstore,custom}/
│   └── requiredDataFiles.json
├── mods/
│   ├── plugins/
│   └── scripts/
├── config.lua.template
└── README.txt
```

```json
{
  "format": "tes3mp-easy-world-bundle",
  "version": 1,
  "created_at": "2026-10-27T16:45:00Z",
  "checkpoint": "checkpoint-2026-10-27_16-45-00.tar.gz",
  "base": "snapshot-2026-10-27_16-40-00.tar.gz",
  "players": 12
}
```

- `config.lua.template` — `config.lua` из базы с пустым `password`, пустыми
  списками бана и без админских ключей; пароли и баны наружу не уходят;
- `README.txt` — как поднять сервер из пакета без панели;
- `200 OK`, `Content-Type: application/gzip`, `Content-Length`,
  `Content-Disposition: attachment; filename="<bundle>"`;
- `ETag` — sha256 от пары (чекпоинт, база);
- `404 not_found` — чекпоинта нет; `409 base_missing` — нет базового снапшота;
  `404 disabled` — флаг выключен;
- лимит скачиваний общий: 60 в минуту на IP.

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
`internal`, `disabled`, `base_missing`. HTTP-статусы: `404`, `404`, `429`,
`503`, `500`, `404`, `409`.

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
| `status`, `logs`, `backups list`, `snapshot list`, `checkpoint list`, `config get` | 5 с |
| `start`, `stop`, `restart`, `staff set`, `config set`, `config restore`, `checkpoint pin`, `checkpoint unpin`, `snapshot rm`, `checkpoint rm`, `backups rm` | 2 мин |
| `mods import`, `mods deploy`, `snapshot create`, `snapshot restore`, `checkpoint create`, `checkpoint restore` | 30 мин |

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

- Единственные форматы — §6.1–§6.6: `mods-*`, `mods-init-*`, `snapshot-*`,
  `checkpoint-*`, `configs-*`.
- State-архивов больше нет: `state-*`, `state-manual-*` и `state-predeploy-*`
  не принимает ни одна команда (`archive_invalid`).
- `current.txt` только `<sha256> <filename>`; строка только с хешем —
  `archive_invalid`.
- Имена вне шаблонов §6.8 не являются валидными архивами.
- Старые URL `/list-backups/...` и `/download/...` не поддерживаются.
- Старые клиентские конфиги не мигрируются.
- При обновлении `bootstrap.sh` переносит `backups/state/` в
  `backups/legacy-state/`, не меняя файлы: данные не удаляются, но и не
  восстанавливаются этой версией. `doctor` показывает `warn` с этим каталогом,
  пока администратор не уберёт его в холодный архив.
- Автоматической конвертации state → checkpoint нет: старый state-архив не
  содержит ссылки на базу, поэтому такая конвертация была бы догадкой.

## 11. Изменение контракта

1. Обновить этот документ.
2. Обновить фикстуры в `testdata/` (или аналоге).
3. Обновить тесты ops/panel/client.
4. Обновить `07-testing.md`, если появляется новый класс проверок.
5. Только после этого менять реализацию.
