# 07 — Тестирование

**Статус:** draft.

**Обновлено:** 2026-10-23.

Стратегия проверяет не «счастливый путь», а контракты и отказы. Каждый пункт
`03-contracts.md` должен иметь тест; каждый failure mode из `02-architecture.md`
— воспроизводимый сценарий.

## 1. Принципы

- **Контракт — источник тестов.** JSON-поля, коды выхода, форматы архивов и
  retention проверяются по `03-contracts.md`, а не по текущей реализации.
- **Без Morrowind в репозитории.** Фикстуры — пустые файлы с правильными
  именами и CRC. Никаких копий `Morrowind.esm`, BSA, текстур и звуков.
- **Systemd не нужен для большинства тестов.** Ops принимает `--root` и
  `Runner`; panel принимает `Ops`; client принимает HTTP-клиент и файловую
  систему.
- **Один и тот же бинарник в тестах и в проде.** Contract-тесты запускают
  `tes3mp-easy-ops`, а не только внутренние функции.
- **Failure injection — часть Definition of Done.** Если операция может
  упасть, тест должен уметь это сымитировать.
- **Память — такой же контракт, как JSON.** Тест на 512 MiB upload обязан
  проверять, что RSS не растёт до размера файла.

## 2. Уровни

| Уровень | Что проверяет | Где живёт |
|---|---|---|
| Unit | tar, CRC, `requiredDataFiles.json`, `current.txt`, retention, config, pathguard, lock, auth | `internal/**/*_test.go` |
| Contract (ops) | запуск бинарника, JSON-envelope, exit codes, stdout/stderr | `test/contract/ops` |
| Contract (panel) | HTTP-коды, заголовки, cookie, JSON публичного API | `internal/panel/**/*_test.go` |
| Contract (client) | разбор ответов API, профиль, установка модов | `internal/{app,serverapi,mods}/**/*_test.go` |
| Infra | preflight `bootstrap.sh` на fake `/proc`, systemd и `df` | `test/infra` |
| Integration | lifecycle на temp-root, mods/state deploy, retention | `test/integration` |
| Fault | битый архив, path traversal, lock, нет места, systemd error, rollback | `test/fault` |
| Resource | 64 MiB cgroup, 512 MiB upload, потоковый tar/gzip | `test/resource` |
| E2E | чистая VM: установка, вход, бэкап, восстановление, клиент | ручной/nightly |
| Docs | ссылки, отсутствие дублированных схем | `test/docs` |

OS-матрица: Debian 12 и Ubuntu 24.04, x86_64, systemd. Другие ОС — best-effort
и не входят в acceptance. Docker-вариант не входит в acceptance; его замер —
в `02-architecture.md` §9.

`test/infra` запускает настоящий `bootstrap.sh` с подменёнными `/etc/os-release`,
`/proc`, системным каталогом systemd и командами `id`, `uname`, `df`,
`systemctl`, `systemd-analyze`, `sysctl`, `getent`, `groupadd`, `useradd`,
`usermod`, `install`, `visudo`. Проверяются успех, предупреждения, каждый отказ
(root, ОС, arch, systemd, RAM, swap, диск), границы порогов и то, что `--check`
не меняет ни одного файла.

Для I02 дополнительно проверяются создание групп/пользователей, layout и режимы
каталогов, установка `rootctl` (`root:tes3mp 0750`) и sudoers (валидация
`visudo -cf` до записи), отказы при отсутствии `systemd-journal` и при
невалидном sudoers, а также идемпотентность: второй запуск не вызывает
`useradd`/`groupadd` и не меняет дерево. `rootctl_test.go` запускает копию
скрипта с fake `systemctl` и проверяет три разрешённых глагола, отказ для
пустого/лишнего/неизвестного аргумента и проброс кода `systemctl`.

Real-host smoke I02 выполнен на Debian 12 x86_64: `id`, ownership/modes,
`visudo -c`, `tes3mp-panel` → ops от `tes3mp`, `tes3mp` → rootctl; `status` и
лишний аргумент отклоняются.

Для I03 дополнительно проверяются установка четырёх unit-файлов, journald
drop-in и sysctl drop-in с режимами `0644`, их содержимое и порядок «verify до
install». Fake `systemd-analyze` покрывает допустимую диагностику
`Command ... is not executable: No such file or directory` (runtime появится в
I04, бинарник ops — в M2, панели — в M4) и отказ на любой другой ошибке; fake
`systemctl` — `daemon-reload` и
перезапуск journald, fake `sysctl` — применение `-p`. Отдельно проверяются
отсутствие `systemd-analyze`/`sysctl`, отсутствие source-файлов, сбой
`daemon-reload` и `sysctl`, а `test/infra/systemd_test.go` — лимиты памяти,
OOM score, пользователи, `ExecStart`, timer и drop-in-файлы.

Real-host smoke I03 на Debian 12: units установлены `0644 root:root`,
`systemd-analyze verify` выдал только ожидаемые ошибки отсутствующих
`ExecStart`, journald перезапущен, `vm.swappiness=10`, повторный bootstrap не
изменил содержимое; юниты не запускались.

Для I04 fake-host дополнительно подменяет `apt-get`, `curl` и `chown`, отдаёт
TES3MP-архив из фикстуры и проверяет: обязательные пакеты, скачивание и
sha256 до распаковки, вызов `chown -R root:root`, режимы runtime и маркера,
содержимое `.tes3mp-install-info`, 12 symlinks в data root, режимы базовых
конфигов и `requiredDataFiles.json`, канонический `mods-init-*`/`current.txt`,
HOME-каталоги и идемпотентность. Отдельные тесты отклоняют несовпадение
sha256, неполный или чужой runtime без перезаписи, неканонический
`current.txt`, symlink вместо `current.txt`, сбой скачивания и `chown`, а также
symlink/не-regular-file вместо конфига или `requiredDataFiles.json`. Для имён
архивов проверяются collision-суффикс `-01`, отказ для `-1`/`-010` и
несуществующей календарной даты.

Для I05 fake `systemctl` дополнительно различает `start`, `enable --now` и
`is-active --quiet` и умеет вернуть отказ для каждой операции или статус
`inactive` для выбранного unit. Проверяются: успешный запуск
`tes3mp.service` и `tes3mp-easy-backup.timer`, их `enable`, отсутствие
`start`/`enable` для `tes3mp-panel.service` и
`tes3mp-easy-backup.service`, отказ команды `start`, сервер, вышедший сразу
после старта, отказ `enable --now` таймера и неактивный таймер. Отдельно
`systemd_test.go` требует `ConditionPathExists` для ops: до M2 таймер не
запускает backup service с отсутствующим `ExecStart`. Идемпотентность
проверяется повторным bootstrap: он вызывает `start`, а не `restart`, поэтому
не трогает уже работающий сервер.

Ограничение: полный real-host smoke I05 на чистой VM Debian 12/Ubuntu 24.04 не
выполнялся; fake-host проверяет контракт вызовов systemd. В dev-окружении
настоящий TES3MP 0.8.1 запускался транзиентным `systemd --user` unit:
`active`, `25565/udp`, запись `world/` подтверждены. `ProtectSystem=strict`,
`ProtectHome=yes`, `ReadWritePaths` и системный `tes3mp.service` ещё нужно
проверить на VM.

Для O01 `internal/systemd` покрыт unit-тестами с
`internal/systemd/systemdtest.Runner`: проверяются точный argv без shell,
разбор `systemctl show`, active/inactive/error для `is-active`, `journalctl`
с хвостовыми и пустыми строками, три глагола rootctl, отказ для неверного
unit/глагола и отсутствие вызова процесса при ошибке валидации. `ExecRunner`
проверяется на реальном helper-процессе: stdout, stderr, exit code, отмена по
context и отказ для имени команды с shell-синтаксисом.

Для O02 CLI `status`/`logs` проверен unit-тестами с тем же fake runner и
contract-тестами реального бинарника. В contract-тестах `systemctl` и
`journalctl` подменяются shell-скриптами через `PATH`: проверяются точный argv,
`--timestamp=utc`, маппинг active/inactive/failed/activating/timer, `started_at`
в UTC RFC3339, `null` для `n/a`, `is-active` с ошибкой → `health=unhealthy`,
недоступный systemd/journald → exit 6, `not-found` → exit 6, неверные
`--unit`/`--tail` → exit 2 без вызова systemd, `lines: []`, отсутствие
`backend`. `status` и `logs` не читают data root и не берут lock.

## 3. Фикстуры

```text
testdata/
├── mods/
│   ├── minimal/          # один esp, requiredDataFiles.json
│   ├── scripts/          # esp + два lua
│   ├── no-crc/           # requiredDataFiles.json с пустыми массивами
│   ├── bad-crc/          # CRC не совпадает
│   ├── root-json/        # requiredDataFiles.json в корне (должен падать)
│   ├── traversal/        # ../evil
│   ├── symlink/          # symlink внутри tar
│   └── duplicate/        # два файла с разным регистром
├── state/
│   ├── empty/            # players/ + world/<5>/ + оба файла
│   ├── full/             # игроки + все пять world-каталогов
│   ├── flat/             # cell/ в корне — должен падать
│   ├── no-current/       # без current.txt — должен падать
│   ├── missing-world/    # нет world/custom/ — должен падать
│   └── old-current/      # current.txt только sha256 — должен падать
├── configs/
│   ├── tes3mp-server-default.cfg
│   ├── config.lua
│   └── banlist.json
└── api/
    ├── server.json
    ├── mods-current.json
    └── errors.json
```

Правила фикстур:

- имена и CRC детерминированы;
- tar-фикстуры собираются в тесте, а не хранятся бинарниками, если это
  возможно;
- для «плохих» архивов можно хранить байты, но с комментарием, что именно
  ломается;
- `testdata` не содержит пользовательских данных;
- retention-фикстуры генерируются программно с заданными mtime, а не лежат
  тысячами файлов в git.

## 4. Ops

### 4.1. Архивы

- `mods import` принимает `minimal`, `scripts`, `no-crc`; три базовых ESM из
  `requiredDataFiles.json` не обязаны лежать в архиве;
- `root-json`, `traversal`, `symlink`, `duplicate`, `bad-crc` → конкретный
  `error.code`;
- лимиты: 512 MiB сжатого, 50 000 записей, 1 GiB распакованного;
- tar с абсолютным путём, `..`, hardlink, device, FIFO;
- gzip с мусором после tar;
- пустой tar;
- архив без `plugins/`;
- `scripts/` с вложенным каталогом;
- скрипт с пробелом/точкой в имени;
- upload и `backups/` на разных файловых системах → `invalid_argument`.

### 4.2. State

- `empty` и `full` принимаются, `current.txt` и `requiredDataFiles.json`
  обязательны;
- `flat`, `no-current`, `missing-world`, `old-current` → `archive_invalid`;
- state с `current.txt` на отсутствующий `mods-*` → `archive_invalid`;
- state с `current.txt` на чужой текущий мод-пак → `archive_invalid` до
  остановки сервера;
- архив без `players/` или без любого из пяти `world/*/` → `archive_invalid`;
- symlink/hardlink в state → `archive_invalid`/`unsafe_path`.

### 4.3. `current.txt`

- канонический `<sha256> <filename>` принимается;
- строка только с `sha256` → `archive_invalid`;
- лишние пробелы, CRLF, пустая строка, два пробела → `archive_invalid`;
- неверная длина хэша, не hex → `archive_invalid`;
- имя с `/`, `..`, не `.tar.gz`, `mods-`/`mods-init-` вне шаблона →
  `archive_invalid`;
- файл ссылается на отсутствующий архив → `archive_invalid`;
- sha256 не совпадает с файлом → `archive_invalid`;
- новый `current.txt` всегда пишется только в каноническом формате.

### 4.4. Retention

Сценарии строятся на синтетическом наборе `state-*` с mtime и проверяют
точный набор выживших:

| Сценарий | Ожидание |
|---|---|
| 120 дней по 5 минут | все 5-минутные старше 7 дней удалены; по одному на каждый день |
| 2 года | дневные старше года удалены; по одному на каждую ISO-неделю |
| 5 лет | недельные старше 3 лет удалены; по одному на каждый месяц; месячные живы |
| 5 лет + давление | месячные всё равно живы; удаляются дневные/недельные по порядку |
| 10 лет | каждый месяц присутствует, даже если бюджет превышен |
| 6 `state-manual-*` | остаются 5 новейших |
| 3 `state-predeploy-*` | остаются 2 новейших |
| 6 `mods-*` | остаются 5 новейших; текущий не удаляется |
| 3 `mods-init-*` | остаются 2 новейших и текущий, если он старше |
| бюджет 1 GiB превышен только месячными | `warnings` непустой, месячные не удалены |
| ошибка `os.Remove` | имя в `warnings`, не в `pruned`; архив создания цел |

Порядок pressure pruning проверяется отдельным тестом: 5-минутные старше 24
часов → дневные → недельные. Месячные не должны появляться в `pruned` ни при
каком сценарии.

### 4.5. Lock и атомарность

- две параллельные мутации: одна проходит, вторая получает `locked` (exit 4);
- `--lock-timeout 0` не ждёт;
- `mods deploy` внутри себя не вызывает `backups create` повторно;
- timer-запуск не блокирует сам себя;
- `--dry-run` не меняет ни одного байта;
- kill -9 в момент записи `current.txt`/архива: старый файл цел, `.part`
  остаётся, `doctor` предупреждает;
- запись конфига: старый файл цел при ошибке fsync/rename (fake writer).

W01 проверяет эти примитивы отдельно. `internal/lock`: exclusive не пускает
exclusive/shared, shared не пускает exclusive, два shared сосуществуют,
ожидание освобождения, timeout 0 и 50ms, повторный `Close`, права 0600,
regular-файл и ошибки пустого пути/отрицательного timeout/неизвестного режима.
`internal/atomicfile`: старый файл виден до `rename`, `.part` имеет ожидаемое
имя, `Abort` не меняет цель, ошибки `fsync`, `rename` и directory sync
возвращаются, а при ошибке до `rename` старый файл цел и `.part` удалён.

### 4.6. Команды

| Команда | Обязательные случаи |
|---|---|
| `doctor` | ok/warn/error, exit 0/0/10, systemd недоступен, незнакомая ОС, non-x86_64, нет места, битый `current.txt`, разные FS, превышен бюджет |
| `status` | running/stopped/starting/unhealthy/unknown, таймер, `started_at`, systemd недоступен, оба unit-а, нет `backend` |
| `logs` | точный argv, `--tail` 1..10000, пустой journal, JSON `lines`, journald недоступен |
| `backups list` | пусто, сортировка, `current`, `--sha256`, `--limit`, `--before`, 2016 записей |
| `backups create` | успех, `--if-running` + stopped, `--if-running` + systemd недоступен (exit 6), `--manual`, нет места, обрыв, `pruned`/`warnings`, имя `state-*` / `state-manual-*` |
| `mods import` | успех, все ошибки архива, upload не удалён при ошибке, cross-FS |
| `mods deploy` | первый деплой, rollback, `--no-restart`, сбой на каждом шаге, `state-predeploy` |
| `state deploy` | успех, старый формат, нет `current.txt`, несовпадение модов, сбой распаковки, rollback, `--no-restart` |
| `staff set` | 0..3, неверный ранг, имя с `/`, отсутствующий игрок, `.bak` |
| `config get/set` | типы, диапазоны, комментарии, read-only, `.bak`, `config_invalid` |

Для O03/O04 `doctor` проверен на уровне пакета и реального бинарника: точный
порядок `root`, `os`, `arch`, `systemd`, `units`, `disk`, `ownership`,
`journald`, `runtime`, `memory`, `cgroup`, `filesystems`, `budgets`, `current`,
`datafiles`, `archives`, `configs`, `cleanup`, `journald-config`; ok/warn/error
и exit 0/0/10; отсутствие `error` в JSON при найденных ошибках; недоступный
systemd (только `systemctl --version`, без `show`/`journalctl`); незнакомая ОС,
старые Debian/Ubuntu, non-x86_64, отсутствующие каталоги, symlink вместо
каталога, недоступный на запись каталог и пороги диска 512 MiB/1 GiB.

O04 дополнительно покрыт тестами владельцев/режимов, journald, runtime
(`ldconfig`/`ldd`), памяти и swappiness, cgroup-лимитов, `stat.Dev`, бюджетов,
`current.txt`+sha256, `requiredDataFiles.json`+CRC32, state-архивов, разбора
INI/Lua/JSON, `customScripts.lua`, `.part`/`.old-*`/`.staging`/lock-файлов и
journald-кэпов. Contract-тесты подменяют `systemctl`, `journalctl`, `uname`,
`ldconfig` и `ldd` через `PATH`; `/etc/os-release`, `/proc`, sysctl и journald
drop-in — переменными `TES3MP_EASY_*`; они нужны только для тестов и не
являются публичными флагами. Полный temp-root собирается с точными режимами,
а `TES3MP_EASY_DOCTOR_SKIP_OWNERS=1` позволяет non-root CI проверить режимы без
`chown`. Отдельный тест сверяет дерево temp-root до и после `doctor`.

O05 дополнительно проверен на пустом и отсутствующем каталоге, сортировке по
mtime с разрешением ничьей по имени, фильтрации классов `mods`/`state`,
`current` для `mods` и `null` для `state`, битом `current.txt` (`archive_invalid`,
exit 7), `--sha256` только для текущей страницы, безопасном и типизированном
`--before`, курсоре, удалённом между страницами, и полном обходе 2016 архивов
страницами по 500. Contract-тесты запускают реальный бинарник и проверяют
`items: []`, точный набор ключей, отсутствие `sha256` без флага, коды выхода
`2`/`8` до чтения каталогов и неизменность дерева data root (lock не берётся).

O06 добавляет сквозной contract-набор для всех read-команд. На одном полном
temp-root `status`, `logs`, `backups list` и `doctor` проверяются на точные
ключи envelope и `data`, отсутствие `backend`/`error`, пустой stderr и один
JSON-объект. Снимок root до и после серии совпадает, `.locks/ops.lock` не
создаётся. Пока тест удерживает эксклюзивный `flock`, все четыре команды с
`--lock-timeout=1ns` обязаны завершиться с exit 0. `status` и `logs` работают
при `--root`, указывающем на обычный файл; глобальные `--root`/`--json` после
команды проверены для всех read-команд. При недоступном systemd/journald
`status`/`logs` дают exit 6, `doctor` — exit 10 без `error`, а `backups list`
не вызывает systemctl/journalctl и продолжает отдавать архивы.

W02 проверяет state-бэкап на трёх уровнях. `internal/archive` пишет архив в
`bytes.Buffer` и валидирует его тем же `ValidateStateReader`: точный порядок
`players/`, `world/`, пяти `world/*/`, затем `requiredDataFiles.json` и
`current.txt`; сортировка файлов, uid/gid 0, одинаковые mtime дают
байт-в-байт одинаковый gzip, пустые каталоги сохраняются, symlink, вложенный
каталог, лишний `world/*` и отсутствующие метаданные отклоняются.

CLI unit-тесты проверяют успешный JSON, `--manual`, retention ручных архивов,
`--if-running` для active/stopped/unavailable, exclusive lock и
`--lock-timeout=0`, shared lock у `--dry-run`, отсутствие lock-файла и архива
после dry-run, `no_space` (<512 MiB) и warning (<1 GiB), отсутствие
`current.txt` или текущего mods-архива. Для порогов диска используется
подменяемая функция `backupDiskFreeFunc`.

Contract-тесты запускают реальный бинарник на temp-root: точные ключи
`archive/size/sha256/skipped/pruned/warnings` (и `reason` без archive-полей при
skipped), проверка `archive.ValidateStateFile`, sha256 файла, отсутствие
`.part`, создание `ops.lock`, stopped `--if-running`, exit 6 при недоступном
systemd, exit 4 при удержанном lock, exit 7 для битого `current.txt` и
неизменность всего дерева после `--dry-run`.

W03 проверяет `mods import` на тех же уровнях. Архивный валидатор C04
принимает `minimal`, `scripts` и `no-crc`; в CLI unit-тестах отклоняются
`root-json`, missing metadata, bad CRC, базовый ESM в `plugins/`, symlink,
duplicate, nested script, traversal и absolute path с кодами 7/8. Проверено,
что при любой ошибке upload остаётся на месте, а `backups/mods` не получает
новых файлов. Отдельные тесты покрывают отсутствующий upload (`not_found`),
symlink/каталог вместо upload, превышение compressed-лимита, cross-FS через
подменяемую `modsImportDeviceOfPathFunc`, symlink вместо `backups/mods`,
удержанный exclusive lock, collision-имя `-01`, retention (5 старых mods),
неизменность `current.txt` и `--dry-run`, который не создаёт ни `backups/`,
ни `.locks`.

Contract-тесты реального бинарника проверяют точные ключи
`archive/sha256/plugins/scripts/pruned/warnings`, sha256 сжатого файла,
`ValidateModsFile` перемещённого архива, удаление upload, неизменность
`current.txt`, exit 3/7/8/4/2 и отсутствие новых файлов при ошибке. На Linux
дополнительно проверяется настоящий cross-FS импорт с `/dev/shm`: если
устройства совпадают, тест пропускается.

## 5. Панель

- логин: верный/неверный пароль, 5 неудач/15 минут, 429, cookie-флаги;
- Argon2id: `memory=32768`, `threads=1`, `time=3`, verify;
- pre-session CSRF на форме логина и CSRF на каждой админской форме;
- сессия: idle 12 ч, absolute 7 дней, рестарт панели = разлогин;
- публичные страницы: `/`, `/backups`, `/api/v1/server`,
  `/api/v1/mods/current`, `/api/v1/mods/current/archive`, `/healthz`;
- P01 lifecycle: env/flags и derived paths, self-signed generation/reuse,
  неполная/битая пара, режимы `0600`/`0700`, SAN, HTTP `308`, ACME `404`,
  TLS ≥ 1.2, security headers, `/healthz`, bind rollback, graceful `SIGTERM`;
- пагинация: `limit`/`before`, `next_before`, 2016 записей, группировка
  день/неделя/месяц;
- скачивание: `..`, абсолютный путь, не `.tar.gz`, отсутствующий файл,
  `Content-Disposition`, `ETag`/304;
- загрузка: `MaxBytesReader` 512 MiB, `MultipartReader`, UUID-имя, вызов fake
  ops, очистка upload;
- ошибки ops: каждый код из таблицы `03-contracts.md` §3 → HTTP и текст;
- конфиги: валидация, повторное чтение, `config_invalid` рядом с полем;
- staff: чтение `players/*.json`, вызов `ops staff set`, баннер рестарта;
- шаблоны: нет неэкранированного пользовательского ввода, нет stack trace;
- заголовки безопасности на всех ответах;
- `service_unavailable` не превращается в «running»;
- неактивный backup timer показывает предупреждение.

Панель не тестирует tar и CRC: это тесты ops. Она тестирует, что правильно
вызывает ops и правильно показывает результат. P01 не тестирует ACME-выпуск:
он ещё не реализован, а `:80` проверяет только redirect и `404` на
challenge-path. Unit-тесты слушают эфемерные порты; реальный bind 80/443
проверяется на VM с `CAP_NET_BIND_SERVICE`.

## 6. Клиент

- поиск Morrowind: реестр, Steam-библиотеки, GOG, Flatpak, ручной выбор;
- проверка ESM, регистр имён, отсутствующий файл;
- `openmw.cfg`: профиль → активный конфиг, сохранение пользовательских строк,
  `.bak`, переключение серверов;
- моды: `no_mods` как ошибка установки, совпадение sha256, скачивание, обрыв,
  битый архив, path traversal, отсутствующий плагин;
- имена архивов: клиент работает только с `mods-*`/`mods-init-*` из API;
- шрифты: успех, 404, битый файл, fallback без шрифтов;
- локализация: overlay, `encoding=win1251`, отказ от установки в Morrowind;
- запуск: Windows env, Linux Proton env, отсутствие Proton;
- TLS: self-signed → понятная ошибка, без отключения проверки;
- GUI не тестируется в CI; `app` тестируется без Fyne.

## 7. Failure injection

- **Нет места.** Fake writer возвращает `ENOSPC`; `TestWriteStatePropagatesNoSpace`
  проверяет проброс через `WriteState`, CLI-тесты — `no_space`, exit 5 и
  сохранность старых файлов.
- **systemd недоступен.** Runner возвращает ошибку; `test/fault` проверяет
  `status` на реальном бинарнике с пустым `PATH` — exit 6
  `service_unavailable`, без `data` и без «running»; панель позже должна
  отдавать 503.
- **Битый gzip/tar.** `test/fault` подаёт не-gzip upload: `archive_invalid`,
  upload остаётся побайтно на месте, `backups/mods` пуст. Contract-тесты
  дополнительно проверяют staging и неизменность `current.txt`.
- **Лимиты архива.** Unit-тесты reader проверяют compressed, uncompressed и
  entries caps; `TestExtractModsEnforcesUncompressedLimit` подтверждает, что
  `ExtractMods` завершается с `ErrLimit` на маленьком тестовом лимите.
- **Обрыв на середине.** `test/fault` убивает `backups create` через SIGKILL,
  когда `*.part` уже вырос: финального архива нет, `.part` остаётся, sha256
  живого state не меняется, lock освобождён. Повторный запуск создаёт валидный
  архив; `.part` под старым именем остаётся для `doctor`, если новое имя
  отличается.
- **Lock.** `test/fault` держит exclusive `flock` и проверяет exit 4 `locked`
  без создания `backups/`; unit- и contract-тесты покрывают shared lock,
  timeout и `--dry-run`.
- **Падение панели.** Сессии в памяти, после рестарта вход заново; публичные
  бэкапы, игра и таймер работают.
- **Падение backup-сервиса.** Панель показывает предупреждение; ручной
  `backups create` работает; следующий тик таймера проходит.
- **Пересечение timer-запусков.** Предыдущий `backups create` ещё держит lock;
  второй запуск не стартует или получает `locked`, но не плодит архивы.
- **Бюджет превышен.** Retention не удаляет месячные; JSON содержит `warnings`,
  `doctor` — `warn`.
- **Разные FS.** Upload и `backups/` на разных mount: `doctor` — `error`,
  `mods import` — `invalid_argument`, файлы не теряются.

## 8. Ресурсные тесты

### 8.1. Память

- `test/resource` собирает реальный `tes3mp-easy-ops` и запускает его с
  `GOMEMLIMIT=64MiB`, `GOMAXPROCS=1`, `TZ=UTC`. `sudo` и `systemd-run` не
  используются.
- `MemoryMax=64M` применяется, только если `/sys/fs/cgroup` уже writable:
  тест создаёт leaf-cgroup напрямую, пишет `memory.max` и `memory.swap.max`,
  добавляет PID в `cgroup.procs` и после завершения читает `memory.peak`.
  Если cgroup read-only, тест не пропускается целиком: он логирует отсутствие
  MemoryMax и проверяет `GOMEMLIMIT` + RSS. В dev-окружении cgroup был
  read-only, поэтому MemoryMax не применялся.
- `mods import` получает потоковый tar.gz с несжимаемым плагином 510 MiB
  (практически предел 512 MiB); `backups create` — state с файлом 500 MiB.
  Оба входа генерируются потоково, без буфера размером с архив.
- Пик RSS дочернего процесса читается из `wait4`/`Rusage.Maxrss`, а не из
  `/proc` теста: `/proc/self/status` показал бы RSS тестового процесса. Для
  каждой команды сравниваются маленький вход (8 MiB mods, 1 MiB state) и
  большой. Порог: абсолютный RSS ≤ 128 MiB и рост ≤ 32 MiB относительно
  маленького входа.
- Фактический dev-замер: `mods import` — 14.0 MiB против 13.8 MiB;
  `backups create` — 13.8 MiB против 11.7 MiB. Обе операции укладываются в
  секунду на 500 MiB.
- Resource-тесты исключены из `-race` (`//go:build linux && !race`):
  race-детектор меняет память и время, поэтому его прогон не является
  ресурсным измерением. Обычный CI-прогон `go test ./...` их выполняет.
- `ulimit -v` для Go-тестов не используем: среда выполнения резервирует
  виртуальное адресное пространство, и лимит ломает не память, а запуск.
  Ограничение — cgroup `MemoryMax`, `GOMEMLIMIT` и измерение RSS.
- `backups create` на state 500 MiB не держит state в памяти: только буфер
  32–64 KiB на файл.

### 8.2. CPU и I/O

- `GOMAXPROCS=1` не создаёт больше одного P; тест не проверяет скорость, а
  проверяет, что нет `GOMAXPROCS` из окружения хоста.
- Backup-сервис имеет `Nice=19` и `IOSchedulingClass=idle`; в unit-тесте
  проверяется сгенерированный unit, в E2E — что бэкап не вытесняет игру по
  CPU на 1 vCPU.

## 9. Acceptance

Сценарии с чистой VM **Debian 12 или Ubuntu 24.04, 1 vCPU, 512–640 MiB RAM,
512 MiB swap, 7 GiB NVMe**:

1. **Установка.** `bootstrap.sh` → пользователи, sudoers, rootctl, systemd,
   journald, sysctl → панель по HTTPS → первый пароль → смена пароля →
   `doctor` без ошибок.
2. **Автобэкап.** Таймер активен; через 5–10 минут появляется `state-*`;
   `backups list` показывает его; при остановленной игре `--if-running` даёт
   `skipped`.
3. **Моды.** Загрузить архив → deploy → сервер перезапустился (если был
   запущен) → клиент видит `mods_sha256` и скачивает `mods-*`.
4. **State.** Создать бэкап → изменить мир → восстановить → состояние
   вернулось, сервер запущен, `state-predeploy-*` существует.
5. **Публичные бэкапы.** Скачать mods/state без логина; список не раскрывает
   пути и логи; пагинация работает на длинной истории.
6. **Retention.** Синтетическая история на 5 лет: месячные живы, дневные и
   недельные прорежены, бюджет не превышен.
7. **TLS.** Сертификат выпущен по IP; HTTP редиректит на HTTPS; после
   перевыпуска клиент не ругается.
8. **Отказ systemd.** Fake/недоступный systemd или unit → панель показывает 503
   `service_unavailable`; после восстановления статус возвращается.
9. **Мало места.** Заполнить диск до порога → `doctor` `error`, мутации
   возвращают `no_space`, данные не теряются.
10. **Клиент.** Windows 10/11: бинарник + Morrowind → мастер → игра. Linux:
    Proton найден автоматически.
11. **Битый `current.txt`.** API отдаёт `404 no_mods` → клиент показывает
    ошибку установки и не запускает vanilla-сервер молча.
12. **Долгая история.** 2016 state-архивов: панель листает страницами, не
    строит одну гигантскую HTML-таблицу, память панели не растёт.

Критерий приёмки — не только «получилось», но и понятное сообщение при
неудаче, путь к логу и отсутствие потери данных.

## 10. CI

Обязательные шаги на каждый PR:

```bash
bash -n bootstrap.sh
sh -n rootctl/tes3mp-easy-rootctl
sudo visudo -cf sudoers.d/tes3mp-easy
go test ./...
go test -race ./...
go vet ./...
GOOS=windows GOARCH=amd64 go build ./cmd/tes3mp-easy-client
GOOS=linux   GOARCH=amd64 go build ./cmd/tes3mp-easy-client
GOOS=linux   GOARCH=amd64 go build ./cmd/tes3mp-easy-ops
GOOS=linux   GOARCH=amd64 go build ./cmd/tes3mp-easy-panel
```

- Integration-тесты используют fake systemd/journalctl на каждом PR.
- `test/fault` и `test/resource` запускаются в обычном `go test ./...`;
  resource исключён из `-race`, fault-kill тоже. Cgroup `MemoryMax` —
  best-effort: без writable cgroup v2 проверка RSS продолжается.
- Реальный systemd E2E — nightly на VM Debian 12/Ubuntu 24.04 с 1 vCPU /
  512 MiB / 7 GiB.
- Fyne GUI не запускается в CI; собирается бинарник.
- Contract-тесты ops запускают собранный бинарник.
- `test/docs` проверяет, что ссылки в `docs/v2/` не битые, что схемы не
  дублируются и что в документации нет старых backend-названий и запрещённых
  полей.

## 11. Definition of Done

Изменение считается готовым, если:

- есть тест на новый контракт и на его failure mode;
- contract-тест ops проверяет JSON и exit code;
- panel-тест проверяет HTTP-код и пользовательское сообщение;
- client-тест проверяет поведение без сети/с битым ответом;
- retention-тест обновлён, если меняется политика или бюджеты;
- resource-тест подтверждает, что память не растёт с размером архива;
- `03-contracts.md` обновлён первым;
- `07-testing.md` обновлён, если появился новый класс проверок.
