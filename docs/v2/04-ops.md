# 04 — Серверный операционный слой (ops)

Документ описывает команды и алгоритмы ops. Нормативные JSON-формы, коды
выхода, форматы архивов и retention — в `03-contracts.md`; при расхождении
прав 03. Ops — единственный административный писатель.

## 1. Зачем отдельный CLI

Ops — это один бинарник `tes3mp-easy-ops`, который умеет всё, что админ делал
руками: смотреть статус, запускать и останавливать сервер, делать и
восстанавливать бэкапы, импортировать и деплоить моды, менять конфиги и
staff rank.

Почему CLI, а не библиотека внутри панели:

- **Тестируемость.** Ops можно запустить с `--root` на временной папке и
  fake systemd/journalctl, без HTTP и без настоящего сервера.
- **Аварийный доступ.** Если панель не поднимается, админ заходит по SSH и
  делает `tes3mp-easy-ops --root /tes3mp-easy --json status`.
- **Один писатель.** Панель, таймер и человек используют один и тот же код и
  один lock.
- **Контракт.** JSON и коды выхода стабильны; панель не зависит от внутренних
  Go-типов.
- **Границы.** Ops не знает про HTTP, пароли и UI. Панель не знает, как
  устроен tar.

## 2. Где лежит

```text
cmd/
  tes3mp-easy-ops/       # main
  tes3mp-easy-panel/     # main, отдельный документ
  tes3mp-easy-rootctl/   # root-helper, фиксированные start/stop/restart
internal/
  ops/                   # команды и сценарии
  archive/               # tar.gz, requiredDataFiles.json, current.txt
  systemd/               # обёртка над systemctl/journalctl/rootctl
  retention/             # buckets, бюджеты, pruning
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
| `--root` | `/tes3mp-easy` | Корень данных |
| `--json` | выкл | Только JSON в stdout, логи в stderr/journald |
| `--dry-run` | выкл | Проверить и показать план, ничего не менять |
| `--lock-timeout` | `30s` | Сколько ждать lock |
| `--command-timeout` | `2m` | Таймаут одной systemctl/journalctl-команды |

`--root` нужен для тестов и нестандартных установок. В production он совпадает
с `/tes3mp-easy`.

### Команды

| Команда | Что делает |
|---|---|
| `doctor [--json]` | Проверяет root, права, systemd, journald, runtime, память, диск, конфиги, `current.txt`, retention-бюджеты |
| `status [--unit tes3mp.service\|tes3mp-easy-backup.timer\|tes3mp-easy-guard.timer] [--json]` | Состояние unit |
| `start` / `stop` / `restart` | Управляет `tes3mp.service` через rootctl |
| `logs --tail N` | Отдаёт последние N строк journald для `tes3mp.service` |
| `backups list --type mods [--limit 200] [--before <name>] [--sha256]` | Страница архивов модов |
| `snapshot create [--label <text>] [--reason manual\|mods\|base] [--dry-run]` | Снапшот: мир + моды + конфиги |
| `snapshot list [--limit 200] [--before <name>]` | Список снапшотов |
| `snapshot restore (--latest\|--archive <name>) [--no-restart]` | Восстанавливает снапшот целиком |
| `checkpoint create [--label <text>] [--if-running] [--dry-run]` | Прогресс мира на базе активного снапшота |
| `checkpoint list [--limit 200] [--before <name>]` | Список чекпоинтов |
| `checkpoint restore (--latest\|--archive <name>) [--accept-mod-mismatch] [--no-restart]` | Восстанавливает мир из чекпоинта |
| `checkpoint pin\|unpin <name>` | Защищает чекпоинт от retention |
| `snapshot rm <name>... [--force] [--dry-run]` | Удаляет снапшоты вручную |
| `checkpoint rm <name>... [--force] [--dry-run]` | Удаляет чекпоинты вручную |
| `mods import --file <path>` | Проверяет и импортирует архив модов |
| `mods deploy (--current\|--latest\|--archive <name>) [--allow-plugin-removal] [--no-restart]` | Деплоит моды и делает снапшот |
| `staff set --player <name> --rank 0..3` | Меняет `settings.staffRank` |
| `config get --file server\|config\|banlist` | Отдаёт структурированные настройки |
| `config set --file ... --key <k> --value <v>` | Меняет одну настройку |

`mods deploy`, `snapshot restore` и `checkpoint restore` останавливают сервер
перед подменой файлов. Если он был запущен, после успеха они запускают его
снова; если был остановлен — оставляют остановленным. Флаг `--no-restart`
оставляет сервер остановленным и в первом случае; он не отменяет остановку
перед подменой.

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
- Читающие команды (`status`, `logs`, `backups list`, `snapshot list`,
  `checkpoint list`, `doctor`) lock не берут: архивы пишутся через temp + rename
  и не видны частично.
- `--dry-run` берёт shared lock и ничего не пишет.
- Внутренние операции не вызывают публичную команду повторно: `mods deploy`
  снимает чекпоинт через `createCheckpointLocked`, `checkpoint create` —
  снапшот через `createSnapshotLocked`, timer — через
  `createCheckpointLocked`. Иначе `flock` в том же процессе даст deadlock.

Любая запись файла — это temp-файл в том же каталоге, `fsync`, `rename`.
Никаких «записали прямо в `config.lua`» и «удалили `players/`, потом
распаковали».

## 6. Команды

### `doctor`

Проверяет:

- `--root` существует и читается; каталоги, в которые пишет `tes3mp`, доступны
  ему на запись; владельцы и режимы совпадают с §4 `02-architecture.md`;
- есть `configs/`, `players/`, `world/`, `mods/`, `backups/`, `panel-data/`;
- `/etc/os-release` — Debian 12+ или Ubuntu 24.04+; незнакомая ОС — `warn`,
  отсутствие файла — `error`;
- `uname -m` — `x86_64`; другая архитектура — `error`;
- `systemctl` работает, версия systemd читается; не-systemd — `error`;
- `tes3mp.service` и `tes3mp-panel.service` установлены и не в `failed`;
  `tes3mp-easy-backup.timer` и `tes3mp-easy-guard.timer` активны и `enabled`;
- journald читается, `journalctl -u tes3mp.service -n 1` не требует пароля;
- `tes3mp-server` и библиотеки релиза на месте; `libluajit-5.1.so.2` находится
  через `ldconfig -p`; `LD_LIBRARY_PATH=/opt/tes3mp-easy/tes3mp/lib ldd
  tes3mp-server.x86_64` не показывает `not found`;
- RAM не меньше 512 MiB; swap не меньше 480 MiB (bootstrap создаёт 512 MiB
  `/swapfile`, если swap меньше 480 MiB), `vm.swappiness=10`;
- cgroup-лимиты юнитов соответствуют §3 `02-architecture.md`;
- свободное место: `warn` меньше 512 MiB, `error` меньше 256 MiB; маркер
  `backups/guard.json` уровня `stop` делает проверку `error` независимо от
  текущего `statfs`;
- `panel-data/uploads` и `backups/` на одной файловой системе (`stat.Dev`);
- `backups/snapshots`, `backups/checkpoints` и `backups/mods` в пределах
  бюджета 1.5 GiB / 256 MiB / 512 MiB;
- `backups/mods/current.txt` канонический, указывает на существующий архив,
  sha256 совпадает;
- `requiredDataFiles.json` читается и соответствует `mods/plugins/` (кроме трёх
  базовых ESM, которых в `mods/plugins/` нет);
- манифесты последних снапшотов и чекпоинтов читаются, digest слоёв совпадает с
  содержимым, у каждого живого чекпоинта есть база; иначе `warn`;
- конфиги парсятся, `customScripts.lua` соответствует `mods/scripts/`;
- нет осиротевших `.part`, `.old-*`, `.staging/` и старых lock-файлов;
- journald-капы `SystemMaxUse=100M`, `RuntimeMaxUse=32M`.

Результат — список проверок в форме `doctor` из `03-contracts.md` §4; схема
здесь не дублируется.

Порядок в `data.checks` стабилен. Проверки:

- `root` — `--root` существует, является каталогом и читается; есть
  `configs/`, `players/`, `world/`, `mods/`, `backups/`, `panel-data/`;
  каталоги игровых данных не являются symlink и доступны на запись текущему
  процессу;
- `os` — `/etc/os-release` читается и разбирается; Debian 12+ и Ubuntu 24.04+
  дают `ok`, другая ОС или старая версия — `warn`, отсутствие файла или
  `ID`/`VERSION_ID` — `error`;
- `arch` — `uname -m` равен `x86_64`, иначе `error`;
- `systemd` — `systemctl --version` читается и начинается с `systemd`;
- `units` — `tes3mp.service` и `tes3mp-panel.service` установлены
  (`LoadState=loaded`) и не в `failed`; `tes3mp-easy-backup.timer` и
  `tes3mp-easy-guard.timer` активны и `enabled`;
- `disk` — свободное место на файловой системе `--root`: `warn` меньше
  512 MiB, `error` меньше 256 MiB; маркер `backups/guard.json` уровня `stop`
  даёт `error` с текстом «сервер остановлен guard-ом: мало места»;
- `ownership` — точные владельцы и режимы из §4 `02-architecture.md`:
  `/` `root:root 0755`, игровые каталоги и их файлы `tes3mp:tes3mp`,
  `panel-data/` `tes3mp-panel:tes3mp 0750`, `uploads/` `2770`, `.locks/` и
  `.staging/` `0700`, HOME-каталоги OpenMW `0700`; symlink вместо каталога —
  `error`;
- `journald` — `journalctl -u tes3mp.service -n 1 --no-pager -o cat` не
  требует пароля; при недоступном systemd проверка не вызывается и даёт
  `error`;
- `runtime` — обязательные файлы TES3MP 0.8.1 на месте и исполняемые,
  `libluajit-5.1.so.2` виден в `ldconfig -p`, а `LD_LIBRARY_PATH=lib ldd
  tes3mp-server.x86_64` не содержит `not found`;
- `memory` — RAM не меньше 512 MiB, swap не меньше 480 MiB и
  `vm.swappiness=10` в drop-in и, если доступно, в `/proc/sys/vm/swappiness`;
  если bootstrap не смог создать `/swapfile`, проверка остаётся `error`;
  потребление RAM/CPU — метрики панели, не проверка `doctor`;
- `cgroup` — `MemoryHigh`/`MemoryMax` четырёх service-юнитов совпадают с §3
  `02-architecture.md` (`384M/448M`, `48M/64M`, `48M/64M`, `16M/32M`);
- `filesystems` — `panel-data/uploads` и `backups/` имеют одинаковый
  `stat.Dev`; при разных устройствах `mods import` вернёт `invalid_argument`.
  Отдельные bind mounts внутри одного устройства проходят проверку, поэтому
  panel unit использует один `ReadWritePaths=/tes3mp-easy`; `EXDEV` при
  импорте дополнительно обрабатывается копированием;
- `budgets` — `backups/snapshots` не больше 1.5 GiB, `backups/checkpoints` не
  больше 256 MiB, `backups/mods` не больше 512 MiB; превышение — `warn`, не
  `error`;
- `current` — `backups/mods/current.txt` канонический, указывает на
  существующий обычный архив, sha256 совпадает;
- `datafiles` — `mods/plugins/requiredDataFiles.json` читается, все три
  базовых ESM перечислены, каждый плагин в каталоге есть в JSON и наоборот;
  указанные CRC32 проверяются по локальным файлам;
- `archives` — самый новый снапшот и самый новый чекпоинт читаются как tar.gz,
  `manifest.json` лежит первой записью, digest слоёв совпадает с файлами;
  если архивов ещё нет — `warn`;
- `legacy` — каталога `backups/legacy-state/` нет; если он есть, `warn` с
  подсказкой убрать старые state-архивы в холодный архив;
- `configs` — `tes3mp-server-default.cfg` (INI), `config.lua`
  (`config.key = value`) и `banlist.json` разбираются; `customScripts.lua`
  содержит ровно `require("custom.<name>")` из `mods/scripts/*.lua` в
  лексикографическом порядке;
- `cleanup` — нет `.part`, `.old-*`, непустого `.staging/` и посторонних
  файлов в `.locks/` (разрешён только `ops.lock`);
- `journald-config` — drop-in содержит `SystemMaxUse=100M` и
  `RuntimeMaxUse=32M`.

Проверки `budgets` и пустого `archives` при нарушении дают `warn`; остальные —
`error`. `warn` не меняет код выхода. `doctor` не берёт lock и не изменяет
data root.

`doctor` возвращает 0, если нет `error` (предупреждения допустимы), и 10
(`checks_failed`), если есть хотя бы одна ошибка. Код 2 зарезервирован за
неверными аргументами.

### `status`

Форма `data` — из `03-contracts.md` §4, `status`; схема здесь не дублируется.

Состояния: `running`, `stopped`, `starting`, `unhealthy`, `unknown`.

Источник — `systemctl show <unit> --property=ActiveState,SubState,ActiveEnterTimestamp,LastTriggerUSec,LoadState,UnitFileState --timestamp=utc` и `systemctl is-active`.
Время запрашивается в UTC, `started_at` отдаётся в RFC3339 (`Z`).

Маппинг состояния: `failed` → `unhealthy`; `activating`/`reloading` →
`starting`; `active` → `running`; `inactive`/`deactivating` → `stopped`;
`unknown` или пустое значение → `unknown`; `LoadState=error` → `unhealthy`.

`health` = `unhealthy`, если unit в `failed`, `LoadState=error|masked` или
`is-active` вернул ошибку; `unknown` при неизвестном состоянии; иначе
`healthy`. Ошибка `is-active` после успешного `show` не меняет `state`, но
делает `health=unhealthy`. `LoadState=not-found` и недоступный systemd — exit
6, `service_unavailable`.

Для таймера `state` — `running`, если таймер активен, иначе `stopped`
(переходные состояния не показываются); `started_at` — `LastTriggerUSec`, если
доступен, иначе `null`. Для сервиса `started_at` — `ActiveEnterTimestamp`;
`n/a` превращается в `null`.

`--unit` принимает только `tes3mp.service` (по умолчанию),
`tes3mp-easy-backup.timer` и `tes3mp-easy-guard.timer`; другое имя — exit 2.
Поле `backend` отсутствует.

### `start` / `stop` / `restart`

```text
sudo -n /usr/local/libexec/tes3mp-easy-rootctl start
sudo -n /usr/local/libexec/tes3mp-easy-rootctl stop
sudo -n /usr/local/libexec/tes3mp-easy-rootctl restart
```

`start` идемпотентен. `stop` ждёт завершения. `restart` не пересоздаёт unit.
Ошибки systemd/rootctl/прав → exit 6, `service_unavailable`. Ops не вызывает
`systemctl` напрямую для этих трёх глаголов; только через rootctl.

### `logs`

`journalctl -u tes3mp.service -n N --no-pager -o cat`.

`--tail N` обязателен: `N` — целое от 1 до 10000. Отсутствующий, нулевой,
отрицательный или слишком большой `N` — exit 2. Команда не принимает другие
аргументы и не читает data root.

Без `--json` — просто текст. С `--json` — объект из `03-contracts.md` с
`data.lines`; панель показывает массив в `<pre>`. Пустой journal — `lines: []`,
exit 0. Если journald недоступен — exit 6, `service_unavailable`.

### `guard`

Внутренняя команда `tes3mp-easy-guard.service`, запускаемая
`tes3mp-easy-guard.timer` раз в 60 с. Пользователю не показывается, но это
обычный ops-глагол с тем же envelope.

1. `statfs` на `--root`; пороги — константы 512 MiB и 256 MiB, не флаги.
2. ≥ 512 MiB: удалить устаревший маркер, если он был, вернуть
   `{"level": "ok", "free": N}`.
3. 256–512 MiB: записать `backups/guard.json` (`0640`) с
   `{"level": "warn", "free": N, "at": "..."}` и строку в journald; сервер не
   трогать.
4. < 256 MiB: то же с `"level": "stop"`, затем `rootctl stop`.
5. Если `tes3mp.service` уже `inactive`/`failed`, вызывается только запись
   маркера: повторный тик видит `stop` и лишь обновляет `free`.

`start`/`restart` удаляют маркер перед вызовом rootctl: админ явно просит
запуск. Если места всё ещё меньше 256 MiB, guard остановит сервер на следующем
тике и снова поставит `stop` — скрытого автозапуска после просадки диска нет.

Lock guard не берёт: `rootctl stop` идемпотентен, а lock в момент нехватки
места занят долгим снапшотом, и ждать его — значит не спасти диск. Guard ничего
не архивирует и не удаляет; удаление — только явные `rm` и retention.

`doctor` читает `backups/guard.json`: уровень `stop` даёт `error` в проверке
`disk`, `warn` — `warn`. Панель строит баннер по `doctor`, а не по своему
таймеру.

### `backups list`

Читает `backups/mods/*.tar.gz`, сортирует по mtime (новые сверху). По
умолчанию не считает sha256 — это дорого на больших архивах. Флаг `--sha256`
добавляет хеш только для текущей страницы.

- `--limit` 1..500, по умолчанию 200;
- `--before <name>` — следующие элементы строго старше курсора. Имя проверяется
  как безопасное (`unsafe_path`, exit 8) и как архив выбранного типа
  (`invalid_argument`, exit 2). Если архив-курсор уже удалён retention, используется
  UTC-время из его имени;
- `next_before` — последний элемент страницы, если страница заполнена;
- `mods` включает `mods-*` и `mods-init-*`. Файлы вне шаблонов
  `03-contracts.md` §6.8, symlink и `*.pin` игнорируются;
- `current` берётся из `backups/mods/current.txt`. Отсутствующий `current.txt` —
  это `null`, битый — `archive_invalid`, exit 7;
- без `--json` выводятся строки `<name>\t<size>\t<mtime>\t<current|->`
  (с `--sha256` добавляется пятая колонка), а также `type`, `current` и
  `next_before`.

```json
{
  "ok": true,
  "data": {
    "type": "mods",
    "current": "mods-2026-10-22_12-00-00.tar.gz",
    "items": [
      {"name": "mods-2026-10-22_12-00-00.tar.gz", "size": 123456, "mtime": "...", "current": true}
    ],
    "next_before": null
  }
}
```

### `snapshot create`

1. Берёт lock.
2. `--dry-run`: проверяет диск и сверяет живой `mods/` с `current.txt` по
   шагам 3–4, но ничего не создаёт; возвращает план и `warnings`, файлы не
   меняются.
3. Проверяет диск: свободно меньше 512 MiB → `no_space`; меньше 1 GiB —
   warning, но продолжает.
4. Сверяет живой `mods/` с архивом из `backups/mods/current.txt`. При
   расхождении сначала создаёт `mods-<timestamp>` из живого каталога и
   обновляет `current.txt` одной транзакцией: иначе `mods_sha256` снапшота не
   совпадёт ни с одним чекпоинтом.
5. Открывает `backups/snapshots/<name>.tar.gz.part` и пишет потоково:
   `manifest.json` первой записью, затем `state/`, `mods/`, `configs/`.
   Digest каждого слоя считается по ходу записи, без промежуточной копии.
6. `fsync` файла и каталога, `rename` в `.tar.gz`.
7. Применяет retention и бюджеты; возвращает `archive`, `size`, `sha256`,
   `mods_archive`, `pruned`, `warnings`.

`--reason manual` — значение по умолчанию; `--reason mods` использует
`mods deploy`, `--reason base` — только ленивая база из `checkpoint create`
(вручную не нужен). Таймер снапшоты не создаёт: при мод-паке в сотни мегабайт это
дороже всего 15-минутного цикла. Снапшот снимается с живого сервера;
согласованную точку даёт `checkpoint-predeploy-*` из `mods deploy` и
восстановления.

### `snapshot list` / `checkpoint list`

Оба читают только `manifest.json` — первую запись архива, не распаковывая его.
Пагинация (`--limit`, `--before`, `next_before`) — как у `backups list`.
`snapshot list` строит карту ссылок по манифестам всех чекпоинтов каталога и
отдаёт `used_by`/`protected`; `checkpoint list` читает `.pin` и проверяет
наличие базы. Битый или чужой манифест — `archive_invalid`, exit 7, без
частичного списка.

### `snapshot restore`

1. Взять lock; проверить архив: `manifest.json` первой записью, `format`,
   `version`, наличие `state/`, `mods/`, `configs/`, совпадение digest слоёв;
   иначе `archive_invalid`.
2. Проверить диск.
3. Сделать `checkpoint-predeploy-*` через `createCheckpointLocked`.
4. Остановить `tes3mp.service`, если он работал.
5. Распаковать `mods/` в `.staging/snapshot-<id>/`, подменить `mods/plugins` и
   `mods/scripts` через rename, завести `mods-*` в `backups/mods/` (или
   переиспользовать архив с тем же sha256), записать `current.txt`,
   сгенерировать `customScripts.lua`.
6. Распаковать `state/` в `.staging/snapshot-<id>/` и перенести
   верхнеуровневые записи `players/` и `world/` так же, как в
   `checkpoint restore`.
7. Распаковать `configs/` поверх `configs/`; каждому файлу — `.bak-<timestamp>`.
8. Запустить сервер, если он работал и не указан `--no-restart`.
9. Ошибка на шагах 5–8: вернуть `mods/`, `current.txt`, `players/`, `world/` и
   `configs/` из staging, запустить сервер, если он работал, и вернуть ошибку.

`--latest` берёт самый новый снапшот любого `reason`. После успеха снапшот
становится базой следующих чекпоинтов: его `mods_sha256` совпадает с
`current.txt`.

### `checkpoint create`

1. Взять lock.
2. `--if-running`: не `active` → `skipped=true`, `reason=not_running`, exit 0;
   недоступный systemd — `service_unavailable`, exit 6, а не skipped.
3. Найти базу: самый новый снапшот, чей `mods_sha256` равен sha256 архива из
   `current.txt`. Если подходящего нет — создать `snapshot-*` с `reason=base`
   через `createSnapshotLocked`.
4. Проверить диск.
5. Писать `backups/checkpoints/<name>.tar.gz.part`: `manifest.json`,
   `players/`, `world/{cell,world,map,recordstore,custom}/`,
   `requiredDataFiles.json`; digest слоя считается по ходу записи.
6. `fsync`, `rename`, retention, ответ.

`--label` попадает только в манифест и в имя не входит. `--dry-run` проверяет
`--if-running` и выбирает базу по шагам 2–3, но не создаёт ни чекпоинт, ни
ленивый `snapshot-*` с `reason=base`. Predeploy-чекпоинты
создаёт та же функция с `reason=predeploy`; `--latest` их не выбирает.
Чекпоинт снимается с живого сервера и не транзакционен: файл, который TES3MP
пишет в этот момент, может попасть в архив частично. При 15-минутном интервале
и типичных ~100 KB это приемлемый recovery point.

### `checkpoint pin` / `checkpoint unpin`

`pin` пишет `<name>.pin` атомарно (temp + rename), повторный вызов
идемпотентен и возвращает `pinned=true`; `unpin` удаляет файл, отсутствие — не
ошибка. Lock берётся: pin влияет на retention, который может идти параллельно.
Архив не переписывается.

### `checkpoint restore`

1. Взять lock; проверить чекпоинт: манифест, digest, `players/`,
   `world/{пять}`, `requiredDataFiles.json`.
2. Найти базу по `base.name` и `base.sha256`. Базы нет и `--accept-mod-mismatch`
   не указан — `not_found`, exit 3, данные не меняются.
3. Если активный мод-пак не совпадает с `base.mods_sha256`, развернуть моды
   базы по шагу 5 `snapshot restore`; иначе `base_deployed=false`.
4. `checkpoint-predeploy-*`, остановка сервера, staging, подмена
   верхнеуровневых записей `players/` и `world/`, запуск — как в
   `snapshot restore`.
5. `configs/` не читается и не пишется. С `--accept-mod-mismatch` моды не
   меняются, `base_missing=true`, а `missing_plugins` перечисляет плагины
   чекпоинта, которых нет в текущем паке. `current.txt` тоже не меняется:
   база следующих чекпоинтов выбирается по текущему паку, а при отсутствии
   подходящего снапшота первый `checkpoint create` создаст `snapshot reason=base`
   (нужно ≥512 MiB свободно).

Сами каталоги `players/` и `world/` не переименовываются: data root —
`root:root 0755`, ops работает от `tes3mp`, и `rename` каталога в корне вернул
бы `EACCES`. Переносятся только записи внутри них, на одном mount
(`ReadWritePaths=/tes3mp-easy`).

### `snapshot rm` / `checkpoint rm`

Правила и JSON — `03-contracts.md` §4. `checkpoint rm` удаляет и обычные
чекпоинты, и `checkpoint-predeploy-*`. Ops проверяет всю пачку до первого
удаления: без `--force` защищённое имя даёт `archive_in_use`, exit 12, и не
удаляется ничего. С `--force` удаляется закреплённый чекпоинт или база (если
это не последний снапшот); чекпоинты базы остаются с `available=false`, их
имена попадают в `warnings`. Последний снапшот не удаляется никогда — только
`skipped`. Retention после удаления не запускается: админ освобождает место, а
не пересобирает политику.

### `mods import`

Вход — путь к `.tar.gz`, который панель сохранила в `panel-data/uploads/`.
Ops не доверяет ни имени, ни содержимому.

Проверки:

- upload и `backups/mods` на одной файловой системе (`stat.Dev`), иначе
  `invalid_argument`; panel unit держит их в одном `ReadWritePaths=/tes3mp-easy`.
  Для совместимости с нестандартными unit-файлами `EXDEV` всё равно
  обрабатывается атомарным копированием;
- gzip и tar читаются потоково, размер в пределах 512 MiB, записей ≤ 50 000,
  распаковано ≤ 1 GiB;
- нет абсолютных путей, `..`, symlink, hardlink, устройств, NUL;
- верхний уровень — только `plugins/` и `scripts/`;
- `plugins/requiredDataFiles.json` существует и парсится как массив объектов;
- `requiredDataFiles.json` в корне архива — ошибка `archive_invalid`;
- каждый плагин из JSON есть в `plugins/`, кроме трёх базовых ESM: они в архив
  не входят и проверяются клиентом по Morrowind; расширение — из белого списка;
- оригинальные `Morrowind.esm`/`Tribunal.esm`/`Bloodmoon.esm` отсутствуют в
  `plugins/`;
- CRC32 (IEEE) каждого плагина из архива совпадает с `0x...` в JSON; CRC
  базовых ESM в архиве не проверяется;
- `scripts/` содержит только `.lua` (без вложенных каталогов);
- нет дубликатов имён.

При успехе архив переименовывается в
`backups/mods/mods-YYYY-MM-DD_HH-MM-SS.tar.gz`. В штатном panel unit upload и
`backups/mods` находятся на одном mount. Если `rename` всё же возвращает
`EXDEV` (нестандартный unit), ops копирует upload в `.part` в `backups/mods`,
делает `fsync`, переименовывает `.part` в целевое имя и удаляет исходный
upload. Копирования между разными файловыми системами нет:
такой случай отсекает проверка `stat.Dev`. `current.txt` **не** меняется. При
ошибке исходный upload не удаляется: панель сама решает, когда чистить
`panel-data/uploads`.

```json
{
  "ok": true,
  "data": {
    "archive": "mods-2026-10-22_12-00-00.tar.gz",
    "sha256": "...",
    "plugins": 12,
    "scripts": 2,
    "pruned": [],
    "warnings": []
  }
}
```

### `mods deploy`

Аргумент: `--current` (взять из `current.txt`), `--latest` (самый новый
`mods-*`/`mods-init-*`) или `--archive <name>`.

Порядок:

1. Взять lock.
2. Найти и проверить архив; для `--current` `current.txt` обязан быть
   каноническим, иначе `archive_invalid`.
3. Сравнить плагины архива с `mods/plugins/` и с `requiredDataFiles.json`
   хранимых чекпоинтов. Удаление плагина, на который ссылается точка отката, —
   `plugin_in_use`, exit 11. `--allow-plugin-removal` разрешает деплой и
   возвращает имена таких чекпоинтов в `warnings`: их восстановление потребует
   `--accept-mod-mismatch`.
4. Посчитать diff по именам и sha256, без чтения содержимого: `added`,
   `removed`, `changed` для плагинов и скриптов.
5. Проверить диск: снапшот + распаковка + запас.
6. Сделать `checkpoint-predeploy-*` через `createCheckpointLocked`, чтобы был
   откат мира.
7. Запомнить, был ли сервер запущен; остановить `tes3mp.service` через
   rootctl.
8. Распаковать `plugins/` и `scripts/` в
   `/tes3mp-easy/.staging/mods-<id>/` потоково, с теми же лимитами.
9. Проверить распакованное ещё раз (пути, файлы, CRC плагинов из архива;
   базовые ESM в архиве отсутствуют).
10. Подменить `mods/plugins` и `mods/scripts` через rename; старые каталоги
    оставить как `mods/plugins.old-<id>` до успеха.
11. Сгенерировать `configs/customScripts.lua` из `mods/scripts/*.lua`
    (отсортировано, `require("custom.<name>")`).
    Каталог `configs/` — `0700`, файл — `0600`: ровно это проверяет doctor.
12. Записать `backups/mods/current.txt` в формате `<sha256> <filename>`.
13. Создать `snapshot-*` с `reason=mods` через `createSnapshotLocked`: он
    становится базой следующих чекпоинтов.
14. Запустить `tes3mp.service`, если он был запущен и не указан
    `--no-restart`.
15. Удалить `.old-*` и staging.

Renames на шагах 10–12 выполняются в namespace панели, где
`ReadWritePaths=/tes3mp-easy` даёт один mount для `.staging` и `mods/`.
Granular-пути в unit вернули бы `EXDEV`; транзакция в этом случае откатилась бы.

Если шаги 10–12 упали: вернуть старые `mods/`, `current.txt` и
`customScripts.lua`, запустить сервер, если он работал до операции и не указан
`--no-restart`, и вернуть ошибку. Это не «автоматический откат вообще всего», а
завершение одной транзакции деплоя.

Ошибка на шаге 13 деплой не отменяет: моды уже применены и `current.txt`
записан. Имя снапшота попадает в `warnings`, а `checkpoint create` создаст базу
сам при следующем запуске.

`--no-restart` оставляет сервер остановленным. Если сервер был остановлен до
операции, ops оставляет его остановленным; если работал — запускает после
успешного деплоя. Штатный сценарий панели — деплой на работающем сервере;
`--no-restart` нужен для аварийного режима.

### `staff set`

```text
tes3mp-easy-ops staff set --player "John Doe" --rank 2
```

- Имя проверяется: непустое, без `/`, `\`, `..`, NUL, не `.`/`..`.
- Файл: `players/<name>.json`. Если файла нет — exit 3, `not_found`; ops не
  создаёт нового игрока.
- `settings.staffRank` = 0..3 (0 none, 1 moderator, 2 admin, 3 owner).
- Перед записью делается `players/<name>.json.bak-<timestamp>`.
- Запись атомарная; при ошибке JSON не портится.
- Возвращает `{"player": "...", "rank": 2, "restart_required": true}`.
- Сервер не перезапускает: панель показывает баннер «нужен рестарт».

Полный снапшот перед сменой ранга не нужен: это один маленький JSON, и рядом
остаётся `.bak`.

### `config get` / `config set`

Файлы:

| `--file` | Путь | Формат |
|---|---|---|
| `server` | `configs/tes3mp-server-default.cfg` | INI |
| `config` | `configs/config.lua` | `config.key = value` |
| `banlist` | `configs/banlist.json` | JSON |

`config get` отдаёт структурированный объект известных ключей, а не сырой
текст. Для `config` поддерживаются простые значения: string, number, boolean.
Таблицы и списки read-only.

`config set`:

- ключ должен быть в белом списке;
- тип и диапазон проверяются (порт 1024–65535, `maxClientsPerIP` 1–10,
  `difficulty` -100..100 и т.п.);
- `--key`/`--value` меняют одну настройку;
- строка `config.key = value` заменяется построчно, комментарии сохраняются;
- перед записью — `.bak-<timestamp>`;
- запись атомарная;
- панель присылает только изменённые ключи, но ops пишет файл целиком, а не
  патч;
- возвращает `restart_required`; сервер не перезапускает — это делает панель или
  админ;
- `customScripts.lua` через `config set` менять нельзя — он генерируется.

Ошибка валидации → exit 9, `config_invalid` с `field`; панель показывает её
рядом с полем, а не общую «500».

## 7. Retention

Алгоритм вызывается после `snapshot create`, `checkpoint create`, `mods import`
и создания `checkpoint-predeploy-*`, а также `doctor` как проверка размера.
Нормативная таблица — `03-contracts.md` §6.9.

1. Прочитать имена архивов и mtime; содержимое архивов не открывается.
2. Построить карту ссылок: для каждого `checkpoint-*` прочитать `base.name` из
   манифеста. Чекпоинт с нечитаемым манифестом считается повреждённым: он
   попадает в `warnings` и базу не защищает.
3. Защитить самый новый чекпоинт, самый новый снапшот, закреплённые
   чекпоинты и `current.txt` вместе с его архивом.
4. Чекпоинты вне 7 дней сгруппировать по UTC-дате (7–365 дней), ISO-неделе
   (1–3 года) и месяцу (старше 3 лет), оставив самого нового в каждой группе.
   Остальные незакреплённые — кандидаты.
5. `checkpoint-predeploy-*` — оставить 2 новейших.
6. Снапшоты: `reason=manual` — 5 новейших, `reason=mods`/`reason=base` —
   2 новейших, плюс все, на которые ссылается хранимый чекпоинт.
7. `mods-*` (кроме `mods-init-*`) — 5 новейших, `mods-init-*` — 2.
8. Удалить кандидатов, затем проверить бюджеты: `snapshots` 1.5 GiB,
   `checkpoints` 256 MiB, `mods` 512 MiB и свободный резерв 512 MiB.
9. Если бюджет превышен, включить pressure pruning в порядке из
   `03-contracts.md` §6.9: predeploy сверх 2 → незащищённые снапшоты
   `reason=base` → чекпоинты вне горячего окна (сначала не представители,
   затем дневные, затем недельные) → самая старая защищённая база.
10. При снятии защиты базы её чекпоинты остаются, получают `available=false`,
    а имена базы и чекпоинтов попадают в `warnings`.
11. Месячные представители, ручные снапшоты и закреплённые чекпоинты не
    удаляются никогда. Если бюджет всё ещё превышен — вернуть `warnings`;
    созданный архив при этом не откатывается.

Удаление — `os.Remove` + `fsync` каталога. Ошибка удаления не отменяет уже
созданный архив: имя попадает в `warnings`, а не в `pruned`, и `doctor`
покажет проблему.

`rm` из §6 политику не запускает: админ удаляет ровно то, что назвал.

## 8. Архивы и генерация

Форматы `mods`, `snapshot`, `checkpoint`, `requiredDataFiles.json`,
`current.txt` и лимиты — только в `03-contracts.md` §6. Здесь остаётся то, что
делает ops поверх этих форматов.

### Baseline от bootstrap

Bootstrap ставит проверенный TES3MP 0.8.1 в `/opt/tes3mp-easy/tes3mp` под
`root:root`.
Версия, URL и sha256 фиксированы в `server/tes3mp-version.txt`; bootstrap
скачивает архив, сверяет sha256 до распаковки и атомарно переименовывает staging
в runtime-каталог. Маркер `.tes3mp-install-info` пишется внутри staging до rename
и хранит те же три значения. Существующий runtime принимается только при
совпадении маркера и наличии обязательных файлов; иначе bootstrap останавливается
и не перезаписывает каталог. Для осознанной переустановки каталог удаляют вручную.

Изменяемые пути runtime — symlinks в `/tes3mp-easy`: конфиги, `mods/scripts`,
`players/`, `world/*`, `requiredDataFiles.json` и `banlist.json`. Базовые
`tes3mp-server-default.cfg`, `config.lua`, `customScripts.lua`, `banlist.json` и
`mods/plugins/requiredDataFiles.json` создаются только при отсутствии. Затем
bootstrap создаёт `mods-init-*`, пишет канонический `current.txt` и при повторном
запуске принимает только канонический `current.txt` с существующим архивом и
совпадающим sha256. Поэтому ops всегда стартует с валидным текущим mod-паком,
даже до первого пользовательского импорта. Затем bootstrap включает и запускает
`tes3mp.service` и `tes3mp-easy-backup.timer`. Пока бинарник ops не установлен,
тики таймера пропускаются по `ConditionPathExists`; панель включается отдельно
после установки её бинарника.

### Потоковая обработка

- `archive/tar`, `compress/gzip` и `io.Copy` с буфером 32–64 KiB.
- Никаких `bytes.Buffer` для архива, `io.ReadAll` для upload или промежуточной
  копии мира.
- `io.LimitReader` на gzip/tar и на каждый файл; счётчики записей и
  распакованного размера.
- Временный файл в целевом каталоге; `fsync` файла и каталога; `rename`.
- `mods import` делает rename upload → `backups/mods/`; при `EXDEV` между
  bind mounts — атомарное копирование через `.part` + `rename`.
- `state/` снапшота и `players/` + `world/` чекпоинта пишутся прямо из data
  root, без staging-копии.
- `tar` детерминированный: сортировка путей, mtime из файлов, uid/gid 0, без
  лишних заголовков. Байт-в-байт совпадение с GNU tar не цель.

## 9. Systemd-адаптер

Пакет `internal/systemd` — единственное место, где ops вызывает systemd,
journald и rootctl. Команды не собирают argv вручную.

```go
type Runner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

- Production: `ExecRunner` запускает `exec.CommandContext` с `systemctl`,
  `journalctl` и `sudo -n /usr/local/libexec/tes3mp-easy-rootctl`. Никакого
  `sh -c` и склейки аргументов в строку.
- `Client.Show` → `systemctl show <unit> --timestamp=utc` с фиксированным набором
  свойств: `ActiveState`, `SubState`, `ActiveEnterTimestamp`, `LastTriggerUSec`,
  `LoadState`, `UnitFileState`; `KEY=VALUE` разбирается в `Properties`.
- `Client.IsActive` → `systemctl is-active <unit>`. Штатные `inactive`, `failed`
  и `unknown` без stderr — это `false` без ошибки; недоступность systemd
  возвращается ошибкой и на уровне команды становится `service_unavailable`.
- `Client.Logs` → `journalctl -u <unit> -n <tail> --no-pager -o cat`; результат —
  массив строк без пустого хвоста. Команда принимает `tail` 1..10000.
- `Client.Control` принимает только `start`, `stop`, `restart` и вызывает
  `sudo -n /usr/local/libexec/tes3mp-easy-rootctl <verb>`. Имя unit фиксировано
  в rootctl и через ops не передаётся.
- Unit-константы: `tes3mp.service`, `tes3mp-panel.service`,
  `tes3mp-easy-backup.service`, `tes3mp-easy-backup.timer`,
  `tes3mp-easy-guard.service`, `tes3mp-easy-guard.timer`. Некорректное имя
  unit или глагол отклоняется до запуска процесса (`ErrInvalidUnit`,
  `ErrInvalidAction`).
- `CommandError` хранит команду, argv, exit code, stderr и cause. Текст ошибки
  пригоден для `--json`, но команды строят машинный код по своему контексту.
- Таймаут — из `--command-timeout`: команда создаёт `context.WithTimeout` и
  передаёт его в `Runner`; адаптер не задаёт второй таймаут.
- В тестах — `internal/systemd/systemdtest.Runner`: записывает каждый вызов,
  возвращает заготовленный stdout/ошибку и не запускает процессы. Реальный
  systemd для unit-тестов не нужен; настоящая проверка остаётся в E2E.

## 10. Тестирование

| Уровень | Что проверяет | systemd |
|---|---|---|
| Unit | tar, CRC, `requiredDataFiles.json`, `current.txt`, retention, config, pathguard, lock | нет |
| Contract | запуск бинарника с `--root` и `--json`, коды выхода, envelope | fake |
| Integration | lifecycle на temp-root, `mods deploy`, `snapshot restore`, `checkpoint restore` | fake + temp-root |
| Fault | битый архив, path traversal, lock, нет места, base_missing, `rm` защищённого архива, systemd error, rollback | fake |
| E2E | чистая VM: установка, вход, чекпоинт, восстановление, клиент | настоящий |
| Docs | ссылки, отсутствие дублированных схем | нет |

Ключевые фикстуры:

- пустой мир с обязательными каталогами;
- снапшот с игроками и всеми пятью world-каталогами;
- чекпоинт с базой, чекпоинт без базы (`available=false`), закреплённый
  чекпоинт;
- чекпоинт, ссылающийся на снапшот, которого нет в каталоге;
- мир с плоскими каталогами (должен падать);
- снапшот без `configs/` и без `requiredDataFiles.json`;
- `current.txt` только с sha256;
- моды с CRC и без;
- архив с `../evil`, абсолютным путём, symlink;
- `current.txt`, указывающий на отсутствующий файл;
- конфиги с комментариями и нестандартным порядком ключей;
- retention-набор из 700 чекпоинтов за 8 дней, чтобы проверить дневные
  представители;
- `rm` пачки, где одно имя защищено: ни один файл не должен исчезнуть.

Temp-root: каждый тест создаёт `t.TempDir()`, кладёт туда `configs/`,
`players/`, `world/`, `mods/`, `backups/` и fake systemd. Это позволяет
проверять почти всё без VPS.

## 11. Non-goals ops

- HTTP-сервер, auth, HTML;
- планировщик с UI — расписание задаёт systemd timer;
- управление клиентом;
- несколько серверов/проектов в одном запуске;
- произвольные shell-команды;
- удаление месячных представителей и закреплённых чекпоинтов без явного
  `--force`;
- переход со старых форматов и клиентских конфигов.
