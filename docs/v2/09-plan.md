# 09 — План реализации

**Статус:** living, рабочий план.

**Обновлено:** 2026-10-23.

Документ фиксирует декомпозицию v2 на проверяемые задачи. Это не контракт:
JSON-схемы, коды ошибок, форматы архивов и HTTP API по-прежнему живут только
в `03-contracts.md`. Если план расходится с `03`, прав `03`.

План обновляется в том же коммите, что и задача: статус, фактический результат
и отклонения. Задача без acceptance-критерия не считается готовой.

## 1. Правила нарезки

- Один milestone — одна ветка подсистемы и один набор задач.
- Одна задача — 1–3 дня, один логический результат, отдельный PR/коммит.
- Один PR не смешивает ops, infra, panel и client.
- Новая задача начинается с контракта, если меняется JSON, код ошибки, формат
  архива или HTTP-ответ. Контракт, фикстуры и тесты меняются вместе.
- Каждая задача проверяет не только happy path, но и failure mode.
- Панель и клиент не начинают потреблять внутренние Go-типы ops: только JSON,
  коды выхода и публичный HTTP API.
- Если задача разрастается, она делится до начала реализации, а не после.
- После каждой задачи — короткий отчёт для человека (до 10 строк) в
  `docs/v2/reports/<ID>.md`: что сделано, как проверено, что осталось. Отчёт
  входит в тот же коммит, что и задача.

## 2. Текущее состояние

- `v2-docs` и `v2-ops` переведены на `cacdb20` (финальная документация).
- `v2` содержит foundation C01–C07 в `78838bb`; рабочая ветка — `v2-infra`.
- C01 закрыт в `aa7561c`: добавлены `go.mod`, каркас `tes3mp-easy-ops`, CI,
  `.gitignore` и команды разработки.
- C02 закрыт: добавлены JSON-envelope, коды выхода 0–10, глобальные флаги,
  `--help` и тесты контракта процесса.
- C03 закрыт: добавлены `internal/pathguard` и потоковый `internal/archive`
  с лимитами и защитой от небезопасных записей.
- C04 закрыт: добавлена потоковая валидация mods-архивов — layout, JSON, CRC,
  расширения, имена скриптов и запрет базовых ESM.
- C05 закрыт: добавлена валидация state, канонического `current.txt` и
  соответствия текущему mods-архиву в `backups/mods/`.
- C06 закрыт: добавлены UTC-имена и коллизии, retention-планировщик с
  buckets, protected-архивами, pressure pruning и бюджетами.
- C07 закрыт: добавлены contract harness реального бинарника, общие builders
  mods/state/config/api-фикстур и docs test для ссылок, схем и старых имён.
- I01 закрыт: добавлен `bootstrap.sh` с preflight для root, ОС, arch, systemd,
  RAM, swap и диска; режим `--check` ничего не меняет.
- I02 закрыт: добавлены пользователи/группы, layout данных, sudoers и
  `rootctl`; повторный запуск установки идемпотентен.
- I03 закрыт: добавлены `tes3mp.service`, `tes3mp-panel.service`,
  `tes3mp-easy-backup.service/.timer`, drop-in journald и `vm.swappiness=10`;
  bootstrap ставит их идемпотентно, но не запускает до появления runtime.
- I04 закрыт: bootstrap ставит проверенный TES3MP 0.8.1, базовые конфиги,
  `mods-init-*`/`current.txt` и связывает runtime с data root; fake-host тесты
  покрывают checksum, отказы, symlinks и идемпотентность.
- I05 закрыт: bootstrap включает и запускает `tes3mp.service` и
  `tes3mp-easy-backup.timer`, проверяет `is-active` и не запускает панель до
  M4; `ConditionPathExists` не даёт таймеру падать до появления ops.
- O01 закрыт: добавлен `internal/systemd` с `Runner`/`ExecRunner`, типизированными
  `Show`, `IsActive`, `Logs`, `Control`, `CommandError` и fake runner
  `internal/systemd/systemdtest`; CLI-команды ещё не подключены.
- O02 закрыт: `status` и `logs` подключены к CLI и адаптеру, `Show` запрашивает
  время в UTC; contract-тесты запускают реальный бинарник с fake
  `systemctl`/`journalctl` через `PATH`.
- O03 закрыт: `doctor` проверяет root/layout, OS, arch, systemd, unit-ы и диск,
  отдаёт `data.checks` и exit 10 без объекта `error`; точные владельцы/режимы и
  остальные проверки — O04.
- O04 закрыт: `doctor` проверяет владельцев/режимы, journald, runtime, память,
  cgroup, файловые системы, бюджеты, `current.txt`, `requiredDataFiles.json`,
  state-архивы, конфиги, cleanup и journald-кэпы.
- O05 закрыт: `backups list` читает mods/state-архивы, отдаёт точные ключи,
  `current`, `--limit`, `--before`, `--sha256`, безопасные коды 2/8.
- O06 закрыт: сквозной contract-набор read-команд на реальном бинарнике:
  envelope, неизменность temp-root, отсутствие ops lock, границы root и
  недоступный systemd/journald.
- W01 закрыт: добавлены `internal/lock` (exclusive/shared `flock` с timeout и
  `ErrLocked`) и `internal/atomicfile` (`.part`, `fsync`, `rename`, `fsync`
  каталога, `Abort`); `pathguard` переиспользован без изменений.
- W02 закрыт: `backups create` пишет state-архив потоково через `atomicfile`,
  берёт exclusive lock (shared для `--dry-run`), проверяет диск и
  `--if-running`, применяет retention и возвращает `pruned`/`warnings`.
  `--lock-timeout=0` принимается, `ErrLocked` маппится в exit 4.
- Следующая задача — W03 (`mods import`).

Схема веток на момент старта:

```text
v2-docs-resource-budget
└── v2-docs
    └── v2
        └── v2-ops
            └── v2-ops-contracts
```

## 3. Критический путь

```text
C01 → C02 → C03 → C04 → C05 → C06 → C07
    → I01…I05 → O01…O06 → W01…W07
    → P01…P06 → K01…K06 → E01…E04
```

Для одного разработчика порядок строго последовательный. При двух разработчиках
после C02 можно параллелить infra и ops read; панель стартует после стабилизации
read-контрактов, клиент — после появления публичного API.

## 4. M0.5 — контракты и фикстуры (foundation)

Ветка: `v2-ops-contracts`. Результат вливается в `v2` до `v2-m1-infra`.
Оценка: 6–8 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| C01 | done | Каркас Go и CI | `go.mod`, `cmd/tes3mp-easy-ops`, CI |
| C02 | done | JSON-envelope, exit codes, флаги | стабильный машинный интерфейс |
| C03 | done | Безопасный tar.gz reader | потоковое чтение и pathguard |
| C04 | done | Валидация mods-архивов | `requiredDataFiles.json`, plugins/scripts |
| C05 | done | Валидация state и `current.txt` | вложенная структура, sha256 |
| C06 | done | Retention и имена архивов | buckets, protected, бюджеты |
| C07 | done | Contract harness и docs test | проверка реального бинарника |

### C01 — каркас Go и CI (done)

Сделано: модуль `github.com/Saagilat/tes3mp-easy` на Go 1.24, минимальный
`tes3mp-easy-ops` с `--help` и валидацией аргументов, unit-тесты, CI с
`gofmt`, `go vet`, `go test`, `go test -race` и сборкой. Функциональные
команды сознательно не добавлялись.

### C02 — JSON-envelope и коды выхода (done)

Сделано: `internal/contract` с типами `Envelope` и `Error`, `internal/ops`
с кодами 0–10 и их отображением в `error.code`, разбор глобальных флагов,
единый JSON-вывод и табличные тесты. Реальный бинарник проверен на JSON,
кодах выхода и help.

Scope:

- типы envelope и ошибки в `internal/contract`;
- коды выхода 0–10 и их отображение в `error.code`;
- глобальные флаги `--root`, `--json`, `--dry-run`, `--lock-timeout`,
  `--command-timeout`;
- неизвестная команда и неверные аргументы → `invalid_argument`, exit 2;
- при `--json` ровно один JSON-объект в stdout, логи только в stderr.

Тесты: таблица всех кодов 0–10, отсутствие лишнего stdout, неверные флаги,
неизвестная команда, `--help` без JSON.

Готово, когда реальный бинарник проходит contract-тест на JSON и коды выхода.

### C03 — безопасный tar.gz reader (done)

Сделано: `internal/pathguard` проверяет базовые имена и пути; `internal/archive`
потоково читает tar.gz, принимает только regular file/directory, отклоняет
symlink/hardlink/device/FIFO, дубликаты, регистровые коллизии и хвостовые данные.
Лимиты 512 MiB / 1 GiB / 50 000 записей настраиваются через `Limits` для тестов.
Пустой tar читается как архив с нулём записей; layout проверяют C04/C05.

Scope:

- `internal/pathguard`: базовое имя, отсутствие `..`, абсолютных путей и NUL;
- `internal/archive`: потоковое чтение tar.gz без загрузки архива в память;
- разрешены только regular file и directory;
- symlink, hardlink, device, FIFO, дубликаты и неоднозначный регистр
  отклоняются;
- лимиты: 512 MiB сжатого, 1 GiB распакованного, 50 000 записей.

Тесты: tar-фикстуры генерируются в тесте; отдельные случаи на каждый запрет,
пустой tar, gzip-мусор после tar и превышение лимитов.

### C04 — валидация mods-архивов (done)

Сделано: `internal/archive` получил `ValidateMods`/`ValidateModsFile`/
`ValidateModsReader` и `ParseRequiredDataFiles`. Плагины читаются потоково,
CRC32 считается без буферизации содержимого; JSON ограничен 8 MiB. Проверяются
layout, порядок базовых ESM, уникальность имён без учёта регистра, формат CRC,
наличие каждого плагина в обе стороны, расширения и имена скриптов. Базовые ESM
в `plugins/` отклоняются.

Scope:

- layout `plugins/` и `scripts/` верхнего уровня;
- `requiredDataFiles.json` только внутри `plugins/`;
- расширения плагинов, имена скриптов, запрет вложенных каталогов;
- три базовых ESM не принимаются в архиве;
- каждый плагин из JSON есть в архиве и наоборот (кроме самого JSON);
- CRC32 в формате `0x` + 8 hex-цифр.

Тесты: `minimal`, `scripts`, `no-crc`, `bad-crc`, `root-json`, `traversal`,
`symlink`, `duplicate` и граничные имена.

### C05 — state и `current.txt` (done)

Сделано: `internal/archive` получил `ValidateState`/`ValidateStateFile`/
`ValidateStateReader`, `ParseCurrent`/`ReadCurrentFile` и `VerifyStateMods`.
Проверяются только `players/` и пять каталогов `world/`, обязательные
`requiredDataFiles.json` и `current.txt`, канонический LF-формат, UTC-имя
mods-архива, его наличие в `backups/mods/` и совпадение sha256 с текущим
`current.txt`. Любое расхождение даёт `ErrInvalid` (`archive_invalid`).

Scope:

- только вложенная структура `players/` +
  `world/{cell,world,map,recordstore,custom}`;
- обязательны `requiredDataFiles.json` и `current.txt`;
- канонический `<sha256> <filename>\n`, проверка sha256 и имени архива;
- `current.txt` указывает на существующий архив в `backups/mods/`;
- state не подменяет моды: несовпадение текущего мод-пака — `archive_invalid`
  до остановки сервера.

Тесты: `empty`, `full`, `flat`, `no-current`, `missing-world`, `old-current`,
несуществующий mods-архив, чужой current, CRLF и лишние пробелы.

### C06 — retention и имена архивов (done)

Сделано: `internal/archive` получил `ParseArchiveName`/`ValidateArchiveName`,
`ValidateModsArchiveName`, `ValidateStateArchiveName`, `ArchiveName` и
`NextArchiveName`/`NextArchiveNameInDir`. `internal/retention` получил `Build`
(чистый план), `Apply` (удаление и fsync каталога), UTC-buckets, защиту
newest/current/monthly, pressure pruning и бюджеты с warnings. `Build` не
меняет диск; `Apply` продолжает после ошибки удаления и возвращает её в
`Warnings`.

Scope:

- UTC-имена по шаблонам из `03-contracts.md` §6.6 и суффиксы `-01`, `-02`;
- горячий, дневной, недельный и месячный buckets;
- защита current-архива, самого нового state и месячных представителей;
- pressure pruning в заданном порядке;
- бюджеты 1 GiB state / 512 MiB mods / 512 MiB свободного резерва;
- warnings вместо удаления месячных при переполнении.

Тесты: синтетическая история на 5 лет, одинаковые mtime, коллизии имён,
защита current, месячные при pressure, бюджеты и пустые каталоги.

### C07 — contract harness, фикстуры и docs test (done)

Сделано: `test/contract/ops` собирает `tes3mp-easy-ops` один раз в `TestMain`
и запускает реальный бинарник с `--root` на temp-dir; проверяются JSON-envelope,
exit code, разделение stdout/stderr, help и неизменность заполненного temp-root.
`test/fixtures` генерирует mods/state/config/api-фикстуры без Morrowind и
пользовательских данных. `test/docs` проверяет локальные ссылки, валидность
JSON-примеров, отсутствие дублированных схем, запрещённых полей и старых
backend-имён; дубли `doctor`/`status` из `04-ops.md` убраны в пользу ссылок на
`03-contracts.md`.

Scope:

- `test/contract/ops` запускает собранный бинарник с `--root` на temp-dir;
- общие builders для mods/state/config/api-фикстур;
- проверка JSON, exit code, stdout/stderr;
- `test/docs`: битые ссылки, дубли схем, старые backend-названия;
- фикстуры не содержат Morrowind и пользовательских данных.

Готово, когда `go test ./...` проверяет контракты через бинарник, а не только
через внутренние функции.

## 5. M1 — инфраструктура

Ветка: `v2-infra`. Оценка: 5–7 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| I01 | done | Preflight bootstrap | OS, arch, systemd, root, диск, идемпотентность |
| I02 | done | Пользователи, каталоги, sudoers, rootctl | узкая модель привилегий |
| I03 | done | systemd-юниты, timer, journald, sysctl | лимиты и установка; запуск в I05 |
| I04 | done | Runtime TES3MP и базовые моды | готовый игровой сервер |
| I05 | done | Первый запуск и bootstrap | сервер и таймер поднимаются без ручных шагов |

### I01 — preflight bootstrap (done)

Сделано: добавлен корневой `bootstrap.sh` с read-only preflight. Скрипт
проверяет root, Debian 12+/Ubuntu 24.04+, x86_64, systemd PID 1 и `systemctl`,
RAM, swap, общий размер и свободное место диска. Все ошибки собираются, а не
обрывают проверку на первой; пороги учитывают служебные потери (512 MiB VM и
7 GiB ФС), ниже 640 MiB RAM выводится предупреждение. `--check` и `--root`
позволяют проверить хост и будущий data root без изменений.

Идемпотентность: preflight и режим `--check` не создают и не изменяют файлы;
тест `--check` сравнивает дерево файлов до и после запуска. Установочные стадии
добавляются в I02–I05; в I01 их ещё не было.

Тесты: `test/infra` запускает настоящий `bootstrap.sh` с fake `/etc/os-release`,
`/proc`, systemd и командами `id`/`uname`/`df`/`systemctl`; покрыты успех,
предупреждение, пороги и каждый отказ.

Готово, когда preflight проходит на поддерживаемом хосте, объясняет каждый
отказ и не меняет диск.

### I02 — privilege model (done)

Сделано: `bootstrap.sh` без `--check` создаёт системные группы и пользователей
`tes3mp`/`tes3mp-panel`, раскладывает data root (`configs`, `players`, `world/*`,
`mods/*`, `backups/*`, `panel-data/*`, `.locks`, `.staging`) с правами из
`02-architecture.md`, ставит `rootctl` и `/etc/sudoers.d/tes3mp-easy`.
`--check` остаётся полностью read-only; preflight failure не начинает установку.

Пользователи: `tes3mp` входит в `tes3mp` и `systemd-journal`, `tes3mp-panel` — в
`tes3mp-panel` и `tes3mp`. `usermod -aG` идемпотентен и сохраняет чужие
дополнительные группы. `rootctl` — фиксированный
`/bin/systemctl <start|stop|restart> tes3mp.service`, ровно один аргумент,
произвольные глаголы и имя юнита отклоняются. Файл ставится
`root:tes3mp 0750`: sudo проверяет execute-бит у вызывающего `tes3mp` до
повышения прав. Sudoers валидируется через `visudo -cf` до установки.

Тесты: `test/infra` fake-host проверяет создание пользователей/групп, layout,
режимы, содержимое sudoers, порядок `visudo`, идемпотентность (повторный запуск
не вызывает `useradd`/`groupadd`), отказ при отсутствии `systemd-journal` и
отказ `visudo`; `rootctl_test.go` проверяет три разрешённых глагола, отказ для
пустого/лишнего/неизвестного аргумента и проброс кода `systemctl`. Real-host
smoke на Debian 12 подтвердил ownership/modes, `visudo -c` и sudo-правила:
`tes3mp-panel` запускает ops от `tes3mp`, `tes3mp` запускает rootctl, а
`status`/лишний аргумент отклоняются.

Готово, когда на чистом Debian 12 пользователи, каталоги, rootctl и sudoers
созданы, повторный запуск не меняет состояние, а rootctl не даёт обойти
фиксированный набор команд.

### I03 — systemd units, journald, sysctl (done)

Сделано: в `systemd/` добавлены `tes3mp.service`, `tes3mp-panel.service`,
`tes3mp-easy-backup.service` и `tes3mp-easy-backup.timer`; в
`systemd/journald.conf.d/tes3mp-easy.conf` — капы `SystemMaxUse=100M` и
`RuntimeMaxUse=32M`, в `sysctl.d/99-tes3mp-easy.conf` — `vm.swappiness=10`.
`bootstrap.sh` валидирует каждый unit через `systemd-analyze verify` до записи,
ставит файлы `0644 root:root` в `/etc/systemd/system`, journald drop-in и
sysctl drop-in, затем делает `systemctl daemon-reload`, перезапускает
`systemd-journald.service` и применяет `sysctl -q -p`.

На момент I03 runtime TES3MP и бинарники ops/panel ещё не установлены
(runtime появится в I04, ops — в M2, panel — в M4), поэтому `systemd-analyze`
сообщает `Command ... is not executable: No such file or
directory`. Bootstrap допускает только эту диагностику для ожидаемого
`ExecStart` соответствующего unit и всё равно падает на синтаксических ошибках,
неизвестных директивах, опечатках в пути и ошибках зависимостей. Юниты
не запускаются и не включаются в I03: первый запуск — в I05, после установки
runtime. Панель получит `CAP_NET_BIND_SERVICE`, `NoNewPrivileges=no` и доступ на
запись только к путям, которые нужны ей и дочернему `ops`; сервер и backup —
`NoNewPrivileges=yes`, `UMask=0027` и минимальные `ReadWritePaths`.

Тесты: `test/infra` fake-host проверяет установку всех шести файлов, режимы,
содержимое, порядок «verify до install», `daemon-reload`, перезапуск journald и
`sysctl -p`; отдельные тесты — допустимая диагностика отсутствующего runtime,
отказ на синтаксической ошибке, отсутствие `systemd-analyze`/`sysctl`,
отсутствие source-файлов, сбой `daemon-reload` и `sysctl`. `systemd_test.go`
проверяет лимиты, пользователей, `ExecStart`, timer и drop-in-файлы. Real-host
smoke на Debian 12 подтвердил установку, режимы `0644`, `systemd-analyze verify`
с ожидаемыми отсутствующими `ExecStart`, `daemon-reload`, restart journald,
`vm.swappiness=10` и идемпотентность; юниты остались `inactive`.

Ограничение I03: unit-файлы используют production-пути `/tes3mp-easy` и
`/opt/tes3mp-easy`; `--root` по-прежнему управляет только data layout и
preflight. Полная проверка запуска и лимитов — после I04/I05 на чистой VM.

Критерий milestone: на Debian 12/Ubuntu 24.04 x86_64 после `bootstrap.sh`
поднимаются `tes3mp.service` и backup timer; unit панели установлен и проходит
`systemd-analyze verify`, но сам бинарник панели появляется только в M4.

### I04 — runtime TES3MP и базовые моды (done)

Сделано: `server/tes3mp-version.txt` фиксирует версию 0.8.1, URL и sha256.
Bootstrap ставит `libluajit`/`curl`/`tar`, скачивает архив, проверяет sha256 до
распаковки, собирает staging (`--strip-components=1 --no-same-owner`), делает
`chown -R root:root`, выставляет executable-биты, пишет
`.tes3mp-install-info` внутри staging и атомарно переименовывает каталог в
`/opt/tes3mp-easy/tes3mp`. Любая ошибка удаляет staging/архив; существующий
runtime принимается только при совпадении маркера и обязательных файлов и
никогда не перезаписывается.

Изменяемые пути runtime — symlinks в `/tes3mp-easy`: три конфига,
`mods/scripts`, `players/`, пять `world/*`, `requiredDataFiles.json` и
`banlist.json`. Базовые конфиги и `requiredDataFiles.json` создаются только при
отсутствии и с отказом на symlink/не-regular-file. Bootstrap также создаёт
`mods-init-*` и канонический `current.txt`, HOME-каталоги
`.config/openmw`/`.local/share/openmw`; `tes3mp.service` получает
`Environment=HOME=/tes3mp-easy` и `ReadWritePaths=.../configs` для banlist.

Тесты: fake-host bootstrap проверяет пакеты, скачивание, checksum, `chown`,
режимы и содержимое маркера, 12 symlinks, конфиги, base-архив, canonical
`current.txt`, HOME-каталоги и идемпотентность. Отказы: checksum mismatch,
runtime без маркера/неполный, чужой layout, сбой `curl`/`chown`, symlinked
config/requiredData/current, неканонический `current.txt`, суффиксы `-1`/`-010`
и невозможная дата. `systemd_test.go` проверяет `HOME` и `configs` в
`ReadWritePaths`.

Ограничение: установка на реальном хосте не выполнялась; checksum, layout и
запуск официального архива с symlinks проверены в sandbox, а real-host smoke
входит в I05 на чистой VM. Известный контракт: runtime требует ровно версию из
`server/tes3mp-version.txt`; для смены версии каталог отводят вручную по
runbook.

Критерий I04: на чистом Debian 12 bootstrap создаёт root-owned TES3MP 0.8.1,
конфиги и валидный `mods-init-*`/`current.txt`, а повторный запуск не меняет
состояние; запуск юнитов и end-to-end smoke — I05.

### I05 — первый запуск и bootstrap (done)

Сделано: после I04 `bootstrap.sh` делает `systemctl enable` и `start` для
`tes3mp.service`, проверяет `is-active --quiet`, затем делает
`enable --now tes3mp-easy-backup.timer` и тоже проверяет активность. Ошибка
любой команды или переход сервера в `inactive` сразу обрывает установку с
указанием на `journalctl`. Повторный запуск использует `start`, а не
`restart`, поэтому идемпотентен и не отключает игроков. `tes3mp-panel.service`
намеренно не включается и не запускается: бинарника панели нет до M4.

Чтобы таймер можно было включить до появления ops, в
`tes3mp-easy-backup.service` добавлен
`ConditionPathExists=/opt/tes3mp-easy/bin/tes3mp-easy-ops`: до M2 тики
пропускаются без `failed`, а после установки ops первый же тик создаёт
state-бэкап. `tes3mp.service` и таймер включаются в автозагрузку.

Тесты: fake `systemctl` различает `start`, `enable --now` и `is-active` и умеет
сымитировать отказ каждой операции и неактивный unit. Покрыты успешный запуск,
отказ `start`/`enable --now`, сервер, вышедший сразу после старта, неактивный
таймер, отсутствие запуска панели и backup service до появления их бинарников,
а также идемпотентность. `systemd_test.go` проверяет `ConditionPathExists`.

Ограничение: полноценный real-host smoke на чистой VM Debian 12/Ubuntu 24.04
не выполнялся — в окружении разработки нет root/systemd-менеджера. Удалось
запустить настоящий TES3MP 0.8.1 транзиентным `systemd --user` unit с
`Type=simple`, `MemoryHigh/MemoryMax`, `NoNewPrivileges`, `UMask` и
`PrivateTmp`: сервер стал `active`, слушал `25565/udp` и писал `world/`.
`ProtectSystem=strict`, `ProtectHome=yes`, `ReadWritePaths` и запуск от
`tes3mp` под системным systemd остаются обязательной проверкой I05 на VM
вместе с `libluajit-5.1.so.2`.

Критерий I05: на чистом Debian 12/Ubuntu 24.04 после `sudo ./bootstrap.sh`
`tes3mp.service` и `tes3mp-easy-backup.timer` активны и включены, сервер
отвечает на `25565/udp`, панель остаётся неактивной до M4, а повторный bootstrap
не перезапускает сервер.

## 6. M2 — ops только чтение

Ветка: `v2-ops-read`. Оценка: 6–8 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| O01 | done | Systemd/journald adapter | интерфейс + fake для тестов |
| O02 | done | `status` и `logs` | состояния unit и журнал |
| O03 | done | `doctor` core | root, OS, arch, systemd, units, диск |
| O04 | done | `doctor` deep | владельцы/режимы, current, конфиги, архивы, бюджеты |
| O05 | done | `backups list` | пагинация, сортировка, `--sha256` |
| O06 | done | Contract-тесты read-команд | реальный бинарник на temp-root |

Критерий milestone: read-команды не берут lock, не меняют файлы, а при
недоступном systemd/journald возвращают `service_unavailable` и exit 6.

### O01 — Systemd/journald adapter (done)

Сделано: `internal/systemd` содержит `Runner` с production `ExecRunner` и
типизированный `Client`: `Show` (`systemctl show` + разбор `KEY=VALUE`),
`IsActive`, `Logs` (`journalctl -o cat`) и `Control` (`sudo -n` + фиксированный
rootctl). Ошибки несут argv, exit code и stderr через `CommandError`; имена
unit и глаголы валидируются до запуска. `internal/systemd/systemdtest.Runner`
записывает вызовы и отдаёт заготовленный ответ. CLI-команды не менялись.

Тесты: точный argv, парсинг свойств, active/inactive/failed/недоступность,
хвостовые и пустые строки журнала, три глагола rootctl, отказ для неверного
unit/глагола, `ExecRunner` на helper-процессе (stdout, stderr, exit code,
context, отсутствие shell).

### O02 — `status` и `logs` (done)

Сделано: CLI получил read-only команды `status` и `logs`. `status` читает
`systemctl show` (с `--timestamp=utc`) и `is-active`, отдаёт `unit`, `state`,
`health`, `started_at`; поддерживаются `tes3mp.service` и
`tes3mp-easy-backup.timer`. Для таймера `started_at` берётся из
`LastTriggerUSec`, для сервиса — из `ActiveEnterTimestamp`. `logs` требует
`--tail 1..10000`, вызывает `journalctl ... -o cat` и отдаёт текст или
`data.lines`.

Ошибки: недоступный systemd/journald и `LoadState=not-found` — exit 6
`service_unavailable`; неверные unit/tail/лишние аргументы — exit 2
`invalid_argument`; ошибка `is-active` после успешного `show` даёт
`health=unhealthy`, но не скрывает состояние. Обе команды не берут lock и не
трогают data root.

Тесты: unit-тесты маппинга/таймстемпов/флагов и contract-тесты реального
бинарника с fake `systemctl`/`journalctl` в `PATH` (точный argv, JSON-поля,
`lines: []`, коды выхода, отсутствие `backend`).

### O03 — `doctor` core (done)

Сделано: добавлен пакет `internal/doctor` и CLI-команда `doctor`. Проверки
`root`, `os`, `arch`, `systemd`, `units`, `disk` идут в стабильном порядке.
`root` требует читаемый каталог, шесть каталогов данных без symlink и доступ
на запись к игровым каталогам; `os` принимает Debian 12+/Ubuntu 24.04+, старую
или незнакомую ОС помечает `warn`; `arch` требует `x86_64`; `systemd` читает
`systemctl --version`; `units` проверяет оба service и активный/enabled timer;
`disk` даёт `warn` ниже 1 GiB и `error` ниже 512 MiB. `systemd.Client` получил
`Version`.

Ошибки: любая проверка со статусом `error` даёт exit 10, но конверт остаётся
`ok: true` без объекта `error`; `warn` — exit 0; неверные аргументы — exit 2 до
вызова systemd. `doctor` не берёт lock и ничего не пишет в root. Точные
владельцы/режимы и остальные проверки из `04-ops.md` оставлены O04.

Тесты: unit-тесты всех статусов и порогов, а также contract-тесты реального
бинарника с fake `systemctl`/`uname` в `PATH` и `TES3MP_EASY_OS_RELEASE` для
`/etc/os-release`; проверены точный argv, отсутствие записи в root и
data-only-форма ошибки.

### O04 — `doctor` deep (done)

Сделано: после шести проверок O03 добавлены `ownership`, `journald`, `runtime`,
`memory`, `cgroup`, `filesystems`, `budgets`, `current`, `datafiles`,
`archives`, `configs`, `cleanup` и `journald-config`. Порядок стабилен и
зафиксирован тестами.

`ownership` сверяет точные владельцы и режимы §4 `02-architecture.md`, включая
`panel-data/uploads 2770`, `.locks/`/`.staging/` `0700`, HOME-каталоги OpenMW и
файлы конфигов `0600`. `journald` читает последнюю строку через
`journalctl`; `runtime` проверяет файлы TES3MP, `ldconfig -p` и `ldd` без
`not found`; `memory` — RAM/swap 512 MiB и `vm.swappiness=10`; `cgroup` —
`MemoryHigh`/`MemoryMax` всех service-юнитов через новый
`systemd.Client.ShowProperties`.

`filesystems` сравнивает `stat.Dev` uploads и backups; `budgets` даёт `warn`
выше 1 GiB/512 MiB; `current` проверяет канонический `current.txt` и sha256;
`datafiles` сверяет JSON с `mods/plugins/` и CRC32; `archives` читает до трёх
state-архивов; `configs` разбирает INI/Lua/JSON и генерируемый
`customScripts.lua`; `cleanup` ищет `.part`, `.old-*`, непустой `.staging/` и
лишние lock-файлы; `journald-config` проверяет кэпы journald.

Тесты: unit-тесты каждой проверки и отдельных парсеров, обновлённые
contract-тесты с полным temp-root, fake `ldconfig`/`ldd` и точным argv
`systemctl show --property=MemoryHigh,MemoryMax`; проверено, что `doctor`
по-прежнему не пишет в root.

### O05 — `backups list` (done)

Сделано: CLI-команда `backups list` читает `backups/mods` или
`backups/state`, отбирает только имена из §6.6, сортирует по mtime (новые
сверху, ничья — по имени по убыванию) и отдаёт страницу `items` с
`current`/`next_before`. `mods` включает `mods-*` и `mods-init-*`, `state` —
все три класса state. Symlink и файлы вне шаблонов игнорируются.

`--limit` 1..500 (по умолчанию 200). `--before` проверяется через
`pathguard` (exit 8 `unsafe_path`) и как архив выбранного типа (exit 2); при
удалённом курсоре используется UTC-время из имени. `current` для `mods`
читается каноническим `archive.ReadCurrentFile` (битый — exit 7), для `state`
всегда `null`. `--sha256` считает хеш только для элементов страницы; без флага
поле отсутствует.

Тесты: unit-тесты разбора флагов, фильтрации, сортировки, пагинации,
удалённого курсора, `current`, sha256 и человекочитаемого вывода;
contract-тесты реального бинарника на temp-root — пустой/отсутствующий
каталог, точные ключи JSON, `items: []`, state `current: null`, битый
`current.txt`, коды 2/8 и обход 2016 архивов страницами по 500.

### O06 — Contract-тесты read-команд (done)

Сделано: сквозной contract-набор для `status`, `logs`, `backups list` и
`doctor` на реальном бинарнике и полном temp-root. Проверяются точный набор
ключей envelope (`ok`/`data`) и `data` каждой команды, отсутствие `backend` и
`error` в успешном ответе, пустой stderr и ровно один JSON-объект.

Read-команды запускаются на одном дереве с `.locks/`: снимок root до и после
совпадает, `.locks/ops.lock` не создаётся. Отдельный тест удерживает
эксклюзивный `flock` на `.locks/ops.lock` и запускает все четыре команды с
`--lock-timeout=1ns`; они завершаются с exit 0, то есть lock не берут.

`status` и `logs` работают, даже если `--root` указывает на обычный файл:
systemd-команды data root не читают. Глобальные `--root`/`--json` после
команды проверены для всех четырёх команд. При недоступном systemd/journald
`status`/`logs` дают exit 6 `service_unavailable`, `doctor` остаётся
диагностическим (exit 10, `ok: true`, без `error`), а `backups list` работает
без вызовов systemctl/journalctl.

Тесты: `test/contract/ops/read_commands_test.go`, Linux-помощник удержания
lock `lock_linux_test.go`; для не-Linux платформ тест lock пропускается.
Обновлён `--help`: `backups list` перечислен как реализованная read-команда.

## 7. M3 — ops мутации

Ветка: `v2-ops-mutations`. Оценка: 9–13 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| W01 | done | Lock, atomicfile, pathguard | общие примитивы записи |
| W02 | done | `backups create` + retention | автобэкап и `--if-running` |
| W03 | planned | `mods import` | проверка и импорт архива |
| W04 | planned | `mods deploy` | predeploy, остановка, restart |
| W05 | planned | `state deploy` | staging, проверка, rollback |
| W06 | planned | `staff set`, `config get/set` | точечные изменения |
| W07 | planned | Fault/resource-тесты | отказы, 512 MiB архив, 64 MiB cgroup |

Критерий milestone: любая мутация либо завершается успешно, либо оставляет
сервер и данные в исходном состоянии; `--no-restart` и «если он работал до
операции» проверены тестами.

### W01 — Lock, atomicfile, pathguard (done)

Сделано: `internal/lock` открывает `.locks/ops.lock` с режимом 0600 и берёт
`flock` в режиме `Exclusive` для мутаций или `Shared` для `--dry-run`.
`Acquire` делает неблокирующие попытки с коротким polling, по истечении
timeout возвращает `ErrLocked`, а нулевой timeout означает ровно одну попытку.
`Close` освобождает lock и оставляет файл на месте; повторный `Close`
идемпотентен. Без `syscall.Flock` пакет возвращает `ErrUnsupported`, а не
мнимый успех.

`internal/atomicfile` пишет детерминированный `<target>.part` в целевом
каталоге, делает `fsync` файла, `rename` поверх цели и `fsync` каталога.
Ошибки `fsync`/`rename` до подмены оставляют старый файл нетронутым и убирают
`.part`; kill -9 до `Commit` оставляет `.part` для `doctor`. Есть потоковый
`Writer` (`Commit`/`Abort`) и удобный `WriteFile`. `pathguard.Base`/`Path`
из C03 уже покрывают имена игроков, скриптов и архивные пути, поэтому новых
функций не потребовалось.

Тесты: эксклюзивная и разделяемая блокировки, ожидание освобождения, timeout
0/50ms, повторный `Close`, права и тип lock-файла, ошибки валидации;
атомарная замена, видимость старого файла до `rename`, `Abort`, ошибки
`fsync`/`rename`/directory sync и default mode.

### W02 — `backups create` + retention (done)

Сделано: `internal/archive.WriteState` потоково пишет state tar.gz из живых
`players/` и `world/`: фиксированные пять `world/*/`, сортировка файлов,
uid/gid 0, mtime из файлов и метаданные `requiredDataFiles.json` +
канонический `current.txt` в конце. Symlink, вложенные каталоги и лишние
`world/*/` отклоняются; лимиты 50 000 записей и 1 GiB распакованного
проверяются по ходу записи.

CLI `backups create --type state [--manual] [--if-running]` парсит аргументы до
lock, берёт exclusive `flock` (shared для `--dry-run`), проверяет свободное
место (<512 MiB — exit 5, <1 GiB — warning), имя с collision-суффиксом и
`current.txt` с существующим mods-архивом. Архив пишется в
`backups/state/<name>.tar.gz.part` через `atomicfile` (fsync → rename → fsync
каталога), sha256 считается по потоку. После успеха вызывается retention;
ошибки удаления/бюджетов остаются в `warnings`, созданный архив не
откатывается. `state-predeploy` использует ту же внутреннюю функцию
`createStateBackupLocked`, чтобы не брать lock повторно.

`--if-running` вызывает `systemctl is-active tes3mp.service`: остановленный
сервер даёт `skipped=true`, `reason=not_running`, недоступный systemd — exit 6.
`--dry-run` не пишет ни байта (включая `ops.lock`, если его ещё нет),
показывает имя, размер, sha256 и планируемый `pruned`. `--lock-timeout=0`
теперь принимается парсером и означает одну неблокирующую попытку; `ErrLocked`
маппится в exit 4 `locked`.

Тесты: архивный layout/порядок/uid-gid/детерминизм/пустые каталоги/symlink;
CLI unit-тесты успеха, `--manual`, retention 5 ручных, `--if-running` active/
stopped/unavailable, lock/timeout 0, dry-run без изменений, no_space,
`current.txt`/mods-архив; contract-тесты реального бинарника на точные ключи,
archive/sha256, lock, skipped, exit 6/7/4/2 и неизменность root.

## 8. M4 — панель MVP

Ветка: `v2-panel`. Оценка: 9–12 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| P01 | planned | HTTP-сервер, TLS, конфиг, lifecycle | панель слушает 80/443 |
| P02 | planned | Auth, сессии, Argon2id, CSRF | вход и смена первого пароля |
| P03 | planned | Ops-клиент и маппинг ошибок | HTTP-коды из exit codes |
| P04 | planned | Публичный API и страницы | server, mods, backups, healthz |
| P05 | planned | Админка и upload | статус, логи, моды, state, конфиги |
| P06 | planned | Contract-тесты и лимиты | 64 MiB cgroup, cookie, ошибки |

Критерий milestone: панель не пишет игровые данные напрямую, не хранит
админский пароль в ops и корректно показывает `service_unavailable`,
`no_space`, `archive_invalid` и `locked`.

## 9. M5 — клиент MVP

Ветка: `v2-client`. Оценка: 9–12 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| K01 | planned | Каркас Fyne, config, i18n | окно и профили без бизнес-логики в UI |
| K02 | planned | Morrowind и TES3MP | поиск, проверка ESM, установка |
| K03 | planned | Server API и моды | скачивание, sha256, распаковка |
| K04 | planned | Шрифты, локализация, launcher | запуск Windows/Linux/Proton |
| K05 | planned | Мастер и ошибки | понятные сообщения и путь к логу |
| K06 | planned | Сборки и smoke | Windows 10/11, Linux + Proton |

Критерий milestone: битый `current.txt`, отсутствующий ESM и офлайн-режим
показывают понятную ошибку; клиент не запускает vanilla-сервер молча.

## 10. M6 — упаковка и E2E

Оценка: 5–7 рабочих дней.

| ID | Статус | Задача | Результат |
|---|---|---|---|
| E01 | planned | Go-пакейджер и release tar | воспроизводимый релиз |
| E02 | planned | Bootstrap из релиза | установка без исходников |
| E03 | planned | E2E Debian 12/Ubuntu 24.04 | 1 vCPU, 512–640 MiB, 7 GiB |
| E04 | planned | Release docs и runbook | обновлённые инструкции |

Критерий milestone: чистая VM проходит acceptance из `07-testing.md` §9,
включая автобэкап, моды, state, retention, TLS, отказ systemd и нехватку места.

## 11. Definition of Done

Задача закрыта, если:

- есть тест на новый контракт и на его failure mode;
- contract-тест ops проверяет JSON и exit code через реальный бинарник;
- panel-тест проверяет HTTP-код и пользовательское сообщение;
- client-тест проверяет поведение без сети и с битым ответом;
- retention-тест обновлён, если меняется политика или бюджеты;
- resource-тест подтверждает, что память не растёт с размером архива;
- `03-contracts.md` обновлён первым, если контракт изменился;
- `07-testing.md` обновлён, если появился новый класс проверок;
- в этом плане обновлён статус задачи и, при отклонении, причина.

## 12. Риски и контроль

- **Память 512 MiB.** Потоковый tar и отсутствие загрузки архива в память
  проверяются уже в C03, а не только в E2E.
- **Systemd fake vs real.** Fake-тесты дешёвые, но не заменяют I05 и E03 на
  чистой VM.
- **Upload 512 MiB.** Панель должна стримить файл в temp, не читать его в
  память; проверка — P05/P06.
- **Fyne и Windows.** Логика клиента тестируется без GUI, но K06 требует
  реальной сборки и smoke.
- **Дрейф документации.** Любое изменение JSON, кода ошибки или формата
  архива начинается с `03-contracts.md`, иначе задача не принимается.

