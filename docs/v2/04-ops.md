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
| `status [--unit tes3mp.service\|tes3mp-easy-backup.timer] [--json]` | Состояние unit |
| `start` / `stop` / `restart` | Управляет `tes3mp.service` через rootctl |
| `logs --tail N` | Отдаёт последние N строк journald для `tes3mp.service` |
| `backups list --type mods\|state [--limit 200] [--before <name>] [--sha256]` | Страница архивов; state-items содержат `mods_archive` и `mods_available` |
| `backups create --type state [--manual] [--if-running] [--dry-run]` | Создаёт state и запускает state-only retention |
| `backups prune [--dry-run]` | Полный retention state + mods |
| `mods import --file <path>` | Проверяет и импортирует архив модов |
| `mods deploy (--current\|--latest\|--archive <name>) [--no-restart]` | Деплоит пакет модов, создаёт `state-predeploy-*`, запускает full retention |
| `state deploy (--latest\|--archive <name>) [--no-restart]` | Восстанавливает world+players+configs на текущем пакете модов |
| `staff set --player <name> --rank 0..3` | Меняет `settings.staffRank` |
| `config get --file server\|config\|banlist` | Отдаёт структурированные настройки |
| `config set --file ... --key <k> --value <v>` | Меняет одну настройку |

`mods deploy` и `state deploy` останавливают сервер перед подменой файлов. Если
он был запущен, после успеха они запускают его снова; если был остановлен —
оставляют остановленным. Флаг `--no-restart` оставляет сервер остановленным и в
первом случае; он не отменяет остановку перед подменой.

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
- Читающие команды (`status`, `logs`, `backups list`, `doctor`) lock не берут:
  архивы пишутся через temp + rename и не видны частично.
- `--dry-run` берёт shared lock и ничего не пишет.
- Внутренние операции не вызывают публичную команду повторно: `mods deploy`
  снимает `state-predeploy-*` через `createStateBackupLocked`, timer — через ту
  же функцию. Иначе `flock` в том же процессе даст deadlock.

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
  `tes3mp-easy-backup.timer` активен и `enabled`;
- journald читается, `journalctl -u tes3mp.service -n 1` не требует пароля;
- `tes3mp-server` и библиотеки релиза на месте; `libluajit-5.1.so.2` находится
  через `ldconfig -p`; `LD_LIBRARY_PATH=/opt/tes3mp-easy/tes3mp/lib ldd
  tes3mp-server.x86_64` не показывает `not found`;
- RAM не меньше 512 MiB; swap не меньше 480 MiB (bootstrap создаёт 512 MiB
  `/swapfile`, если swap меньше 480 MiB), `vm.swappiness=10`;
- cgroup-лимиты юнитов соответствуют §3 `02-architecture.md`;
- свободное место: `warn` меньше 1 GiB, `error` меньше 512 MiB;
- `panel-data/uploads` и `backups/` на одной файловой системе (`stat.Dev`);
- `backups/state` и `backups/mods` в пределах бюджета 1.0 GiB / 512 MiB;
- `backups/mods/current.txt` канонический, указывает на существующий архив,
  sha256 совпадает;
- последние state-архивы читаются, их `current.txt` ссылается на существующий
  пакет модов; последние mods-архивы проходят валидацию;
- `requiredDataFiles.json` читается и соответствует `mods/plugins/` (кроме трёх
  базовых ESM, которых в `mods/plugins/` нет);
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
  (`LoadState=loaded`) и не в `failed`; `tes3mp-easy-backup.timer` активен и
  `enabled`;
- `disk` — свободное место на файловой системе `--root`: `warn` меньше
  1 GiB, `error` меньше 512 MiB. Проверка не останавливает сервер и не
  блокирует `ops`; `backups create` отдельно требует резерв 512 MiB, а
  `mods deploy` — резерв плюс место на распаковку и predeploy (`no_space`,
  exit 5);
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
- `budgets` — `backups/state` не больше 1.0 GiB, `backups/mods` не больше
  512 MiB; превышение — `warn`, не `error`;
- `current` — `backups/mods/current.txt` канонический, указывает на
  существующий обычный архив, sha256 совпадает;
- `datafiles` — `mods/plugins/requiredDataFiles.json` читается, все три
  базовых ESM перечислены, каждый плагин в каталоге есть в JSON и наоборот;
  указанные CRC32 проверяются по локальным файлам;
- `archives` — последние state-архивы читаются как tar.gz, их `current.txt`
  ссылается на существующий пакет модов; последние mods-архивы проходят
  валидацию; если архивов ещё нет — `warn`;
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

`--unit` принимает только `tes3mp.service` (по умолчанию) и
`tes3mp-easy-backup.timer`; другое имя — exit 2.
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
  `03-contracts.md` §6.7, symlink игнорируется;
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

### `backups create`

CLI: `backups create --type state [--manual] [--if-running] [--dry-run]`.

1. Взять ops lock.
2. При `--if-running` проверить `tes3mp.service`; если остановлен — вернуть
   `skipped=true`, `reason=not_running` и ничего не создавать.
3. Проверить свободное место. Если меньше 1 GiB, выполнить full retention:
   state-политика и mods с защитой всех пакетов из `current.txt` состояний.
4. Прочитать `players/`, `world/`, три файла `configs/` и
   `backups/mods/current.txt`; `customScripts.lua` не включать.
5. Записать `current.txt` первой записью tar, затем `configs/`, корневой
   `requiredDataFiles.json`, `players/` и пять каталогов `world/`.
6. Атомарно опубликовать архив и выполнить state-only retention.

State-создание никогда не открывает и не удаляет mods-архивы, кроме
disk-pressure full retention до записи.

### `backups prune`

CLI: `backups prune [--dry-run]`.

Полный retention по `03-contracts.md` §6.8. Для защиты mods читается
`current.txt` каждого state; текущий `backups/mods/current.txt` защищён всегда.
Ошибки отдельных удалений возвращаются в `warnings`, остальные файлы
обрабатываются. `--dry-run` только печатает имена.

### `state deploy`

CLI: `state deploy (--latest|--archive <name>) [--no-restart] [--dry-run]`.

1. Найти state по имени или `--latest`.
2. Проверить layout, `current.txt` и три конфига; старые state без `configs/`
   допустимы.
3. Проверить, что `current.txt` состояния совпадает с
   `backups/mods/current.txt`, а sha256 — с пакетом. Иначе `archive_invalid`.
4. Создать `state-predeploy-*` из живых данных.
5. Остановить сервер, если он работал.
6. Распаковать state в `.staging`, заменить содержимое `players/`, `world/` и
   три файла `configs/`. `customScripts.lua` и посторонние файлы в `configs/`
   остаются.
7. Запустить сервер, если он работал и не указан `--no-restart`.
8. При ошибке откатить `players/`, `world/`, `configs/` и очистить staging.

Моды не подменяются автоматически. Панель предлагает сначала `mods deploy`
для указанного пакета, затем `state deploy`.

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
3. Проверить диск: state-predeploy + распаковка + запас.
4. Создать `state-predeploy-*` через `createStateBackupLocked`, чтобы был
   откат мира.
5. Запомнить, был ли сервер запущен; остановить `tes3mp.service` через
   rootctl.
6. Распаковать `plugins/` и `scripts/` в
   `/tes3mp-easy/.staging/mods-<id>/` потоково, с теми же лимитами.
7. Проверить распакованное ещё раз (пути, файлы, CRC плагинов из архива;
   базовые ESM в архиве отсутствуют).
8. Подменить `mods/plugins` и `mods/scripts` через rename; старые каталоги
   оставить как `mods/plugins.old-<id>` до успеха.
9. Сгенерировать `configs/customScripts.lua` из `mods/scripts/*.lua`
   (отсортировано, `require("custom.<name>")`).
   Каталог `configs/` — `0700`, файл — `0600`: ровно это проверяет doctor.
10. Записать `backups/mods/current.txt` в формате `<sha256> <filename>`.
11. Запустить `tes3mp.service`, если он был запущен и не указан
    `--no-restart`.
12. Удалить `.old-*` и staging.
13. Выполнить full retention: state-политика и mods с защитой пакетов,
    указанных в `current.txt` любого state.

Renames на шагах 8–10 выполняются в namespace панели, где
`ReadWritePaths=/tes3mp-easy` даёт один mount для `.staging` и `mods/`.
Granular-пути в unit вернули бы `EXDEV`; транзакция в этом случае откатилась бы.

Если шаги 8–10 упали: вернуть старые `mods/`, `current.txt` и
`customScripts.lua`, запустить сервер, если он работал до операции и не указан
`--no-restart`, и вернуть ошибку. Это завершение одной транзакции деплоя.

Ошибка retention после успешного деплоя не отменяет моды: предупреждения
попадают в `warnings`.

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

Полный state backup перед сменой ранга не нужен: это один маленький JSON, и
рядом остаётся `.bak`.

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

Нормативная политика — `03-contracts.md` §6.8.

**State-only** запускается после каждого state-архива:

1. Прочитать имена и mtime state-архивов. Содержимое не открывается.
2. Защитить самый новый state, переданные защищённые имена и месячных
   представителей.
3. Сгруппировать автоматические state по возрасту: 0–7 дней — все; 7–365 дней
   — новый за UTC-дату; 1–3 года — новый за ISO-неделю; старше 3 лет — новый
   за месяц навсегда.
4. Оставить 5 новейших ручных и 2 новейших predeploy.
5. Удалить кандидатов и проверить бюджет `backups/state` и свободный резерв.
   Pressure pruning удаляет только state-архивы.

**Full** запускается на `mods import`, успешном `mods deploy`,
`backups prune` и при нехватке места:

1. Выполнить state-only политику.
2. Оставить 5 новейших `mods-*` и 2 новейших `mods-init-*`.
3. Защитить `backups/mods/current.txt`.
4. Прочитать `current.txt` каждого state и защитить все указанные пакеты.
   Если `current.txt` не читается, защитить все mods.
5. Удалить незащищённые mods-архивы.

Удаление — `os.Remove` + `fsync` каталога. Ошибка удаления не отменяет уже
созданный архив: имя попадает в `warnings`, а не в `pruned`.

## 8. Архивы и генерация

Форматы `mods`, `state`, `requiredDataFiles.json`, `current.txt` и лимиты —
только в `03-contracts.md` §6. Здесь остаётся то, что делает ops поверх этих
форматов.

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
- `players/`, `world/` и `configs/` state-архива пишутся прямо из data root, без
  staging-копии.
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
  `tes3mp-easy-backup.service`, `tes3mp-easy-backup.timer`. Некорректное имя
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
| Integration | lifecycle на temp-root, `mods deploy`, `state deploy`, восстановление configs | fake + temp-root |
| Fault | битый архив, path traversal, lock, нет места, systemd error, rollback | fake |
| E2E | чистая VM: установка, вход, state backup, восстановление, клиент | настоящий |
| Docs | ссылки, отсутствие дублированных схем | нет |

Ключевые фикстуры:

- пустой мир с обязательными каталогами;
- state с игроками, всеми пятью world-каталогами и тремя конфигами;
- state без `configs/` для обратной совместимости;
- state с повреждённым `current.txt` и с ссылкой на отсутствующий пакет;
- мир с плоскими каталогами (должен падать);
- state без `requiredDataFiles.json`;
- `current.txt` только с sha256;
- моды с CRC и без;
- архив с `../evil`, абсолютным путём, symlink;
- `current.txt`, указывающий на отсутствующий файл;
- конфиги с комментариями и нестандартным порядком ключей;
- retention-набор state за 5 лет и mods, где старый пакет защищён state;
- `backups prune`, который не удаляет пакет, указанный в state.

Temp-root: каждый тест создаёт `t.TempDir()`, кладёт туда `configs/`,
`players/`, `world/`, `mods/`, `backups/` и fake systemd. Это позволяет
проверять почти всё без VPS.

## 11. Non-goals ops

- HTTP-сервер, auth, HTML;
- планировщик с UI — расписание задаёт systemd timer;
- управление клиентом;
- несколько серверов/проектов в одном запуске;
- произвольные shell-команды;
- удаление месячных представителей state без явного `backups prune`;
- переход со старых snapshot/checkpoint и клиентских конфигов.
