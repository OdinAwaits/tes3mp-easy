# 04 — Серверный операционный слой (ops)

**Статус:** draft.

**Обновлено:** 2026-10-22.

Документ описывает команды и алгоритмы ops. Нормативные JSON-формы, коды
выхода и форматы архивов — в `03-contracts.md`; при расхождении прав 03.
Ops заменяет ad-hoc bash-скрипты из `server/scripts/`.

## 1. Зачем отдельный CLI

Ops — это один бинарник `tes3mp-easy-ops`, который умеет всё, что админ делал
руками: смотреть статус, запускать и останавливать сервер, делать и
восстанавливать бэкапы, импортировать и деплоить моды, менять конфиги и
staff rank.

Почему CLI, а не библиотека внутри панели:

- **Тестируемость.** Ops можно запустить с `--root` на временной папке и
  fake-Docker, без HTTP и без настоящего сервера.
- **Аварийный доступ.** Если панель не поднимается, админ заходит по SSH и
  делает `docker compose run --rm panel tes3mp-easy-ops status`.
- **Один писатель.** И панель, и `backup`, и человек используют один и тот же
  код и один lock.
- **Контракт.** JSON и коды выхода стабильны; панель не зависит от внутренних
  Go-типов.
- **Границы.** Ops не знает про HTTP, пароли и UI. Панель не знает, как
  устроен tar.

Позже, если подпроцессы станут неудобны, панель сможет импортировать пакеты
ops напрямую. Контракт при этом останется.

## 2. Где лежит

```text
cmd/
  tes3mp-easy-ops/       # main
  tes3mp-easy-panel/     # main, отдельный документ
internal/
  ops/                   # команды и сценарии
  archive/               # tar.gz, requiredDataFiles.json, current.txt
  docker/                # обёртка над docker compose
  config/                # чтение/запись cfg, lua, banlist
  lock/                  # flock
  atomicfile/            # temp + rename
  pathguard/             # проверка имён и путей
  doctor/                # проверки окружения
```

Пакеты не должны тянуть HTTP-сервер, шаблоны или клиентские зависимости.

## 3. CLI-контракт

### Глобальные флаги

| Флаг | По умолчанию | Смысл |
|---|---|---|
| `--root` | `/tes3mp-easy` | Корень данных и compose-файла |
| `--compose-file` | `$root/docker-compose.yml` | Compose-файл |
| `--project` | `tes3mp` | Имя compose-проекта |
| `--json` | выкл | Только JSON в stdout, логи в stderr |
| `--dry-run` | выкл | Проверить и показать план, ничего не менять |
| `--lock-timeout` | `30s` | Сколько ждать lock |
| `--command-timeout` | `2m` | Таймаут одной docker-команды; долгие операции могут задавать больший |

`--root` и `--compose-file` нужны для тестов и для нестандартных установок.
В контейнере значения по умолчанию совпадают с текущим layout.

### Команды

| Команда | Что делает |
|---|---|
| `doctor [--json]` | Проверяет root, права, compose, Docker, диск, конфиги, `current.txt` |
| `status [--service tes3mp или backup] [--json]` | Состояние сервиса |
| `start` / `stop` / `restart` | Управляет `tes3mp` через compose |
| `logs --tail N` | Отдаёт последние N строк логов `tes3mp` |
| `backups list --type mods\|state` | Список архивов, размер, mtime, current |
| `backups create --type state [--if-running]` | Создаёт state-бэкап |
| `mods import --file <path>` | Проверяет и импортирует архив модов |
| `mods deploy (--current\|--latest\|--archive <name>)` | Деплоит моды |
| `state deploy (--latest\|--archive <name>)` | Восстанавливает state |
| `staff set --player <name> --rank 0..3` | Меняет `settings.staffRank` |
| `config get --file server\|config\|banlist` | Отдаёт структурированные настройки |
| `config set --file ... --key ... --value ...` | Меняет одну настройку |
| `backup loop --interval 5m [--if-running]` | Бесконечный планировщик для сервиса `backup` |

Команды `mods deploy` и `state deploy` по умолчанию останавливают и запускают
сервер. Флаг `--no-restart` оставляет сервер остановленным после операции;
он не отменяет остановку перед подменой файлов.

## 4. JSON и коды выхода

Нормативно — `03-contracts.md` §2–§3. Кратко:

- `--json` → ровно один объект `{"ok":...}` в stdout, логи в stderr;
- без `--json` — человекочитаемый вывод;
- коды `0`–`10`, включая `10 checks_failed` для `doctor`;
- панель строит логику по `error.code`, а не по `message`.

## 5. Lock и атомарность

Все мутации берут эксклюзивный lock:

```text
/tes3mp-easy/.locks/ops.lock
```

- Реализация: `flock`/`syscall.Flock` на файле; lock живёт, пока жив процесс.
- `--lock-timeout 30s`: по истечении — exit 4, `locked`.
- Читающие команды (`status`, `backups list`, `doctor`) lock не берут: архивы
  пишутся через temp + rename и не видны частично.
- `--dry-run` берёт shared lock и ничего не пишет.
- Внутренние операции не вызывают публичную команду повторно: `mods deploy`
  делает state-бэкап через `createStateBackupLocked`, а `backup loop` — через
  ту же внутреннюю функцию. Иначе `flock` в том же процессе даст deadlock.

Любая запись файла — это temp-файл в том же каталоге, `fsync`, `rename`.
Никаких «записали прямо в `config.lua`» и «удалили state, потом распаковали».

## 6. Команды

### `doctor`

Проверяет:

- `--root` существует и доступен на запись;
- есть `configs/`, `players/`, `world/`, `mods/`, `backups/`;
- `docker compose version` работает, socket доступен;
- свободное место больше порога (например, 2 ГБ + 2× размер state);
- `backups/mods/current.txt` указывает на существующий архив, sha256 совпадает;
- `requiredDataFiles.json` читается;
- конфиги парсятся, `customScripts.lua` соответствует `mods/scripts/`;
- нет осиротевших `.part`, `.old-*`, `.staging/` и старых lock-файлов.

Результат — список проверок:

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

`doctor` возвращает 0, если нет `error` (предупреждения допустимы), и 10
(`checks_failed`), если есть хотя бы одна ошибка. Код 2 зарезервирован за
неверными аргументами.

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

Состояния: `running`, `stopped`, `starting`, `unhealthy`, `unknown`.
Источник — `docker compose ps --format json`. Если Docker недоступен — exit 6.

Для панели дополнительно можно запросить `ops status --service backup`, но
основной статус — `tes3mp`.

### `start` / `stop` / `restart`

```text
docker compose -f <compose> -p <project> up -d tes3mp
docker compose -f <compose> -p <project> stop tes3mp
docker compose -f <compose> -p <project> restart tes3mp
```

`start` идемпотентен. `stop` ждёт завершения. `restart` не пересоздаёт
контейнер без необходимости. Ошибки Docker → exit 6.

### `logs`

`docker compose logs --tail N --no-color tes3mp`. Без `--json` — просто текст.
С `--json` — всегда `{"lines": ["...", "..."]}`; панель показывает массив в
`<pre>`.

### `backups list`

Читает `backups/mods/*.tar.gz` или `backups/state/*.tar.gz`, сортирует по
mtime (новые сверху). По умолчанию не считает sha256 — это дорого на больших
архивах. Флаг `--sha256` добавляет хеш.

```json
{
  "ok": true,
  "data": {
    "type": "mods",
    "current": "import-2026-10-22_12-00-00.tar.gz",
    "items": [
      {"name": "import-2026-10-22_12-00-00.tar.gz", "size": 123456, "mtime": "...", "current": true}
    ]
  }
}
```

### `backups create`

1. Берёт lock.
2. Проверяет диск (2× размер state).
3. Создаёт staging-копию state: `players/`, `world/{cell,world,map,recordstore,custom}/`,
   `requiredDataFiles.json`, `current.txt`.
4. Пишет `backups/state/export-YYYY-MM-DD_HH-MM-SS.tar.gz.part`.
5. `fsync` + `rename` в `.tar.gz`.
6. Возвращает имя, размер, sha256.

`--if-running`: если `tes3mp` не `running`, вернуть `{"skipped": true}` и exit 0.
Это режим для `backup loop`, чтобы не плодить одинаковые архивы на стоящем
сервере.

Имя: `export-...` для планировщика, `manual-...` для кнопки в панели. Оба
варианта валидны для `state deploy`.

### `mods import`

Вход — путь к `.tar.gz`, который панель сохранила в `panel-data/uploads/`.
Ops не доверяет ни имени, ни содержимому.

Проверки:

- gzip и tar читаются, размер в пределах лимита;
- нет абсолютных путей, `..`, symlink, hardlink, устройств;
- верхний уровень — только `plugins/` и `scripts/`;
- `plugins/requiredDataFiles.json` существует и парсится как массив объектов;
- `requiredDataFiles.json` в корне архива — ошибка `archive_invalid`;
- каждый плагин из JSON есть в `plugins/`, расширение из белого списка;
- оригинальные `Morrowind.esm`/`Tribunal.esm`/`Bloodmoon.esm` отсутствуют в
  `plugins/`;
- CRC32 (IEEE) каждого плагина совпадает с `0x...` в JSON;
- `scripts/` содержит только `.lua` (без вложенных каталогов);
- нет дубликатов имён.

При успехе архив переименовывается в
`backups/mods/import-YYYY-MM-DD_HH-MM-SS.tar.gz`; если upload и `backups/`
на разных файловых системах, используется copy + fsync + rename, а не
частичная запись. `current.txt` **не** меняется. При ошибке исходный upload
не удаляется: панель сама решает, когда чистить `panel-data/uploads`.

```json
{
  "ok": true,
  "data": {
    "archive": "import-2026-10-22_12-00-00.tar.gz",
    "sha256": "...",
    "plugins": 12,
    "scripts": 2
  }
}
```

### `mods deploy`

Аргумент: `--current` (взять из `current.txt`), `--latest` (самый новый
`import-*`/`init-*`) или `--archive <name>`.

Порядок:

1. Взять lock.
2. Найти и проверить архив.
3. Проверить диск: state-бэкап + распаковка + запас.
4. Сделать state-бэкап текущего состояния через `createStateBackupLocked`
   (внутренняя функция, не повторный вызов `backups create`), чтобы был откат.
5. Остановить `tes3mp`.
6. Распаковать `plugins/` и `scripts/` в
   `/tes3mp-easy/.staging/mods-<id>/`.
7. Проверить распакованное ещё раз (пути, файлы, CRC).
8. Подменить `mods/plugins` и `mods/scripts` через rename; старые каталоги
   оставить как `mods/plugins.old-<id>` до успеха.
9. Сгенерировать `configs/customScripts.lua` из `mods/scripts/*.lua`
   (отсортировано, `require("custom.<name>")`).
10. Записать `backups/mods/current.txt` в формате `<sha256> <filename>`.
11. Запустить `tes3mp` (если не `--no-restart`).
12. Удалить `.old-*` и staging.

Если шаг 8–11 упал: вернуть старые `mods/`, старый `current.txt`, старый
`customScripts.lua`, запустить сервер, вернуть ошибку. Это не «автоматический
откат вообще всего», а завершение одной транзакции деплоя.

`--no-restart` оставляет сервер остановленным. Если сервер был остановлен до
операции, ops оставляет его остановленным; если работал — запускает после
успешного деплоя. В MVP панель всегда деплоит на работающем сервере, а
`--no-restart` нужен для аварийного режима.

### `state deploy`

Порядок:

1. Взять lock.
2. Проверить архив: tar читается, нет опасных путей, есть `players/` и
   ожидаемые каталоги `world/`.
3. Проверить диск.
4. Остановить `tes3mp`.
5. Распаковать в `/tes3mp-easy/.staging/state-<id>/`, разложив плоские
   `cell/`, `world/`, `map/`, `recordstore/`, `custom/` из архива в
   `world/<name>/`; рядом — `players/`, `requiredDataFiles.json`,
   `current.txt`.
6. Переименовать текущие `players` → `players.old-<id>`, `world` →
   `world.old-<id>`.
7. Переименовать staging-каталоги на их место.
8. Запустить `tes3mp`.
9. При успехе удалить `.old-*`; при ошибке вернуть старые каталоги и запустить
   сервер.

Важно: текущая реализация `deploy_state.sh` сначала удаляет state, а потом
распаковывает. Здесь порядок обратный: сначала полностью подготовили новое,
потом заменили. Если архив битый или места не хватило, текущее состояние
остаётся на месте.

`requiredDataFiles.json` и `current.txt` из архива должны соответствовать
модам, с которыми делался бэкап. Ops не подменяет моды:

- если `current.txt` указывает на другой архив, чем текущий
  `backups/mods/current.txt`, — `archive_invalid` и подмены нет;
- если `current.txt` в legacy-формате (только `sha256`), ops ищет архив модов
  по хэшу; не нашёл — `archive_invalid`;
- если `current.txt` отсутствует (очень старый state), восстановление идёт с
  предупреждением, `mods_archive` = `null`;
- `--force` в MVP нет: лучше остановиться, чем смешать несовместимые CRC.

### `staff set`

```text
tes3mp-easy-ops staff set --player "John Doe" --rank 2
```

- Имя проверяется: непустое, без `/`, `\`, `..`, NUL, не `.`/`..`.
- Файл: `players/<name>.json`.
- `settings.staffRank` = 0..3 (0 none, 1 moderator, 2 admin, 3 owner).
- Перед записью делается `players/<name>.json.bak-<timestamp>`.
- Запись атомарная; при ошибке JSON не портится.
- Возвращает `{"player": "...", "rank": 2, "restart_required": true}`.
- Сервер не перезапускает: панель показывает баннер «нужен рестарт».

Полный state-бэкап перед сменой ранга не нужен: это один маленький JSON, и
рядом остаётся `.bak`. Это осознанное отличие от старого скрипта, который
паковал `players/` и `world/`.

### `config get` / `config set`

Файлы:

| `--file` | Путь | Формат |
|---|---|---|
| `server` | `configs/tes3mp-server-default.cfg` | INI |
| `config` | `configs/config.lua` | `config.key = value` |
| `banlist` | `configs/banlist.json` | JSON |

`config get` отдаёт структурированный объект известных ключей, а не сырой
текст. Для `config` поддерживаются простые значения: string, number, boolean.
Таблицы и списки в MVP read-only.

`config set`:

- ключ должен быть в белом списке;
- тип и диапазон проверяются (порт 1024–65535, `maxClientsPerIP` 1–10,
  `difficulty` -100..100 и т.п.);
- строка `config.key = value` заменяется построчно, комментарии сохраняются;
- перед записью — `.bak-<timestamp>`;
- запись атомарная;
- `customScripts.lua` через `config set` менять нельзя — он генерируется.

Ошибка валидации → exit 9, `config_invalid`. Панель показывает её рядом с
полем, а не общую «500».

### `backup loop`

```text
tes3mp-easy-ops backup loop --interval 5m --if-running
```

- Тикер каждые `--interval` (минимум 1m).
- На каждом тике вызывает `createStateBackupLocked` с семантикой
  `--if-running`, а не публичную команду через subprocess.
- Lock занят → пишет в stderr и пропускает тик, не падает.
- `SIGTERM`/`SIGINT` — корректный выход.
- Retention по умолчанию выключен: архивы не удаляются. `doctor` предупреждает
  о диске; позже добавим `--retention-days`.

## 7. Архивы и генерация

Форматы `mods` и `state`, `requiredDataFiles.json`, `current.txt` и лимиты —
только в `03-contracts.md` §6. Здесь остаётся то, что делает ops поверх этих
форматов.


### `customScripts.lua`

Генерируется из `mods/scripts/*.lua`:

```lua
require("custom.test")
require("custom.another")
```

Порядок — лексикографический. Файл read-only для панели.

### Детерминированность

Go-пакейджер пишет tar с фиксированным порядком, `mtime` из исходных файлов,
uid/gid 0, без лишних заголовков. Байт-в-байт совпадение с GNU tar не цель;
parity-тест сравнивает **содержимое** и метаданные после распаковки.

## 8. Docker-адаптер

```go
type Runner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

- Production: `exec.CommandContext(ctx, "docker", args...)`, без `sh -c`.
- Compose-вызовы: `docker compose -f <file> -p <project> ...`.
- `status`: `compose ps --format json`; парсим и JSON-массив, и JSON-lines
  (разные версии Compose).
- `logs`: `compose logs --tail N --no-color`.
- Таймаут из `--command-timeout`.
- В тестах — fake runner, который пишет вызовы и возвращает заготовленный
  вывод/ошибки. Никакой реальный Docker для unit-тестов не нужен.

## 9. Тестирование

| Уровень | Что проверяет | Docker |
|---|---|---|
| Unit | tar, CRC, `requiredDataFiles.json`, `current.txt`, config, pathguard, lock | нет |
| Contract | запуск бинарника с `--root` и `--json`, коды выхода, envelope | fake |
| Integration | compose lifecycle, mods/state deploy на temp-root | да, отдельный проект |
| Parity | ops vs текущие `package.sh`/`deploy_*.sh` на фикстурах | нет |
| Failure | битый архив, path traversal, lock, нет места, Docker error, rollback | fake |
| Acceptance | сценарий админа на чистой VM | да |

Ключевые фикстуры:

- пустой state;
- state с игроками и всеми пятью world-каталогами;
- моды с CRC и без;
- архив с `../evil`, абсолютным путём, symlink;
- `current.txt`, указывающий на отсутствующий файл;
- конфиги с комментариями и нестандартным порядком ключей.

Temp-root: каждый тест создаёт `t.TempDir()`, кладёт туда `configs/`,
`players/`, `world/`, `mods/`, `backups/` и compose-заглушку. Это позволяет
проверять почти всё без VPS.

## 10. Миграция со старых скриптов

| Сейчас | Куда переходит |
|---|---|
| `server/scripts/deploy_mods.sh` | `ops mods deploy` |
| `server/scripts/import_mods.sh` | `ops mods import` |
| `server/scripts/deploy_state.sh` | `ops state deploy` |
| `server/scripts/set-staff-rank.sh` | `ops staff set` |
| `server/scripts/list-backups.sh` | `ops backups list` |
| `server/scripts/package.sh` | `internal/archive` (target), shell-адаптер на переходный период |
| `server/docker/export_server.sh` | `ops backup loop` + панель |
| `nginx` | панель |
| `server/scripts/install.sh` | bootstrap + compose |

Что чиним по дороге:

- `deploy_state.sh` не удаляет state до успешной распаковки;
- нет хардкода `/tes3mp-easy` — есть `--root`;
- нет `jq`/`rhash` — JSON и CRC32 на Go;
- нет хрупкого `docker ps --filter` — `compose ps`;
- нет `set -e`-ловушек с частичными изменениями;
- все ошибки — структурированные, с кодом.

## 11. Non-goals ops

- HTTP-сервер, auth, HTML;
- планировщик с UI;
- управление клиентом;
- несколько серверов/проектов в одном запуске;
- произвольные shell-команды;
- удаление бэкапов по расписанию (пока);
- миграция старых клиентских конфигов.
