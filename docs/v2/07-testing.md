# 07 — Тестирование

Стратегия проверяет не «счастливый путь», а контракты и отказы. Каждый пункт
`03-contracts.md` имеет тест; каждый failure mode из `02-architecture.md` —
воспроизводимый сценарий.

## 1. Принципы

- **Контракт — источник тестов.** JSON-поля, коды выхода, форматы архивов и
  retention проверяются по `03-contracts.md`, а не по текущей реализации.
- **Без Morrowind в репозитории.** Фикстуры — синтетические файлы с нужными
  именами и CRC. Копий `Morrowind.esm`, BSA, текстур и звуков нет.
- **Systemd не нужен для большинства тестов.** Ops принимает `--root` и
  `Runner`; панель принимает `Ops`; клиент принимает HTTP-клиент и файловую
  систему.
- **Один и тот же бинарник в тестах и в проде.** Contract-тесты ops запускают
  `tes3mp-easy-ops`, а не только внутренние функции.
- **Failure injection — часть Definition of Done.** Если операция может
  упасть, тест умеет это сымитировать.
- **Память — такой же контракт, как JSON.** Тест на 512 MiB upload проверяет,
  что RSS не растёт до размера файла.

## 2. Наборы тестов

| Уровень | Что проверяет | Где живёт |
|---|---|---|
| Unit | tar/gzip, CRC, `requiredDataFiles.json`, `current.txt`, retention, config, pathguard, lock, auth, clientlog | `internal/**/*_test.go` |
| Contract (ops) | запуск бинарника, JSON-envelope, exit codes, stdout/stderr | `test/contract/ops` |
| Contract (panel) | HTTP-коды, заголовки, cookie, JSON публичного API | `test/contract/panel` |
| Client | разбор ответов API, профиль, установка модов и шрифтов, локализация, запуск | `internal/{app,serverapi,mods,fonts,locale,launcher}/**/*_test.go` |
| Infra | preflight и установка `bootstrap.sh` на fake `/proc`, systemd и `df`; rootctl; unit-файлы; установка бинарников из релиза | `test/infra` |
| Release | воспроизводимый tar.gz, `VERSION`, режимы файлов, sha256, отказ на symlink/неполный набор бинарников | `internal/release/**/*_test.go` |
| Fault | битый архив, path traversal, lock, недоступный systemd, обрыв записи | `test/fault` |
| Resource | cgroup 64 MiB, 512 MiB upload, потоковый tar/gzip | `test/resource` |
| Fixtures | валидные и невалидные mods/state-архивы, канонический `current.txt` | `test/fixtures` |
| Docs | ссылки, JSON-примеры, дублированные схемы, запрещённые имена | `test/docs` |

OS-матрица: Debian 12 и Ubuntu 24.04, x86_64, systemd. Другие ОС — best-effort
и не входят в приёмку. Контейнерный рантайм не тестируется и не входит в
приёмку.

`test/infra` запускает настоящий `bootstrap.sh` с подменёнными `/etc/os-release`,
`/proc`, системным каталогом systemd и командами `id`, `uname`, `df`,
`systemctl`, `systemd-analyze`, `sysctl`, `getent`, `groupadd`, `useradd`,
`usermod`, `install`, `mv`, `visudo`, `fallocate`, `dd`, `mkswap`, `swapon`.
Проверяются успех, предупреждения, каждый отказ (root, ОС, arch, systemd, RAM,
диск), границы порогов, автоматическое создание `/swapfile` при нехватке swap
и то, что `--check` не меняет ни одного файла.

Дополнительно `test/infra` проверяет:

- создание групп и пользователей, layout и режимы каталогов, установку
  `rootctl` (`root:tes3mp 0750`) и sudoers (валидация `visudo -cf` до записи);
- нехватку swap: warning вместо fatal, создание и активацию `/swapfile` на
  512 MiB, запись в `/etc/fstab`, идемпотентность и warning при неудачном
  `swapon` без отмены установки;
- отказы при отсутствии `systemd-journal` и при невалидном sudoers;
- идемпотентность: второй запуск не вызывает `useradd`/`groupadd` и не меняет
  дерево;
- установку четырёх unit-файлов, journald drop-in и sysctl drop-in с режимами
  `0644`, их содержимое и порядок «verify до install»;
- ровно один `ReadWritePaths=/tes3mp-easy` в panel unit: granular-пути ломают
  `rename` между `.staging` и `mods/`/`players/`/`world/`;
- допустимость отсутствующего `ExecStart` для ops/panel (бинарники ставятся
  отдельно) и отказ на любую другую ошибку `systemd-analyze verify`;
- `ConditionPathExists` у backup service: таймер не запускает сервис
  с отсутствующим `ExecStart` и не уходит в `failed`;
- что bootstrap запускает `tes3mp.service` и `tes3mp-easy-backup.timer`, не
  запускает и не включает панель без `bin/`, а повторный запуск использует
  `start`, а не `restart`;
- что релизный `bin/` устанавливает оба бинарника с режимом `0755`, а
  неполный набор отвергается до записи;
- что при наличии панели bootstrap включает её и использует `restart`, чтобы
  подхватить заменённый бинарник;
- что `reset-failed` для панели вызывается только при реальном `failed`:
  systemd 252 возвращает ошибку для inactive unit, и это не должно срывать
  первую установку;
- что `--tls-mode staging` пишет `TES3MP_PANEL_TLS_MODE=acme` с staging-CA в
  `TES3MP_PANEL_ACME_DIRECTORY` и по-прежнему требует свободный `80/tcp`, а
  `--tls-mode files` копирует пару в `/etc/tes3mp-easy/panel.{crt,key}` (`0640`
  `root:tes3mp-panel`) без публичного IPv4 и без проверки `80/tcp`; оба режима
  пишут `TES3MP_PANEL_RUNTIME_ROOT`, чтобы шапка панели показывала версию
  TES3MP при нестандартном `INSTALL_ROOT`;
- что неверный `--tls-mode`, `files` без пары и `--tls-cert` без `files`
  возвращают код 2, а URL вместо адреса в `files` отвергается;
- что прогон `acme` удаляет пару, скопированную предыдущим `files`-прогоном.

`internal/release` собирает архив дважды и сравнивает байты: tar/gzip не
содержат времени, uid/gid или пути хоста, `VERSION` генерируется из аргумента,
`.sha256` соответствует содержимому. Тест также проверяет режимы
`bootstrap.sh`/бинарников и отказ на symlink или отсутствующий бинарник.

`rootctl_test.go` запускает копию скрипта с fake `systemctl` и проверяет три
разрешённых глагола, отказ для пустого/лишнего/неизвестного аргумента и
проброс кода `systemctl`.

`test/contract/ops` собирает бинарник через `TestMain` и подменяет `systemctl`
и `journalctl` shell-скриптами через `PATH`. Проверяются точный argv,
`--timestamp=utc`, маппинг active/inactive/failed/activating/timer,
`started_at` в UTC RFC3339, `null` для `n/a`, `is-active` с ошибкой →
`health=unhealthy`, недоступный systemd/journald → exit 6, `not-found` →
exit 6, неверные `--unit`/`--tail` → exit 2 без вызова systemd, `lines: []`,
отсутствие `backend`. `status` и `logs` не читают data root и не берут lock.

## 3. Ops

### 3.1. Архивы

- `internal/archive` проверяет tar/gzip: пустой архив, запрещённые типы
  записей, абсолютные пути, `..`, symlink/hardlink, дубли, мусор после tar и
  gzip, обрезанный архив, лимиты сжатого/распакованного размера и числа
  записей.
- Валидатор mods принимает minimal, scripts, `no-crc` и `mods-init`; отклоняет
  неверный layout, неверный `requiredDataFiles.json`, CRC-несовпадение,
  опасные пути и архивы с базовыми ESM.
- Валидатор state принимает пустой, полный и неявные каталоги; отклоняет
  плоский `world/`, отсутствующий `current.txt`, несовпадение мод-пака и
  небезопасные записи.
- `WriteState` проверяет детерминированный layout, пустых игроков, лимиты,
  небезопасные записи и проброс `ENOSPC`.
- Имена архивов проверяются по шаблонам `03-contracts.md` §6.7; коллизия
  получает суффикс.

### 3.2. `current.txt`

Парсер принимает только `<sha256> <filename>` с lowercase SHA256 и
существующим именем; строка только с хешем, лишние поля, неверный hex и
неподдерживаемое имя отклоняются. `ReadCurrentFile` проверяет обычный файл,
symlink и отсутствие.

### 3.3. Retention

`internal/retention` проверяет пустые и отсутствующие каталоги, ручные и
predeploy-лимиты, защиту текущего mods-архива и защиту всех модов при
невалидном `current.txt`, пятилетнюю историю, выбор последнего за день,
неделю и месяц, pressure pruning с сохранением месячных, порядок удаления,
коллизии имён, бюджеты mods и свободное место. Retention использует имена
архивов, а не mtime.

### 3.4. Команды

`test/contract/ops` покрывает:

- `doctor`: успех, warning/error, `checks_failed` (exit 10), точный порядок и
  содержимое проверок;
- `status`/`logs`: коды, UTC, пустой хвост, ошибки systemd/journald;
- `start`/`stop`/`restart`: успех, dry-run, ошибка, неверные аргументы, lock;
- `backups list`: пагинация, фильтры, невалидный `current.txt`, поля
  `mods_archive`/`mods_available` у state;
- `backups create`: state, `--if-running`, dry-run, `locked`, `no_space`,
  отсутствующий конфиг (`archive_invalid`), state-only retention;
- `backups prune`: dry-run, защита пакетов, указанных в state;
- `mods import`: валидный архив, `--name` с сохранением имени и коллизией,
  битый gzip, отсутствующий upload, lock, cross-filesystem, `EXDEV` copy
  fallback (успех и ошибка копирования), non-`EXDEV` rename error;
- `state import`: валидный state на активном пакете, basename/`--name`,
  коллизия, отсутствующий пакет модов, dry-run, `EXDEV` copy fallback,
  retention защищает импортированный архив;
- `mods deploy`: `--current`/`--latest`/`--archive`, `--no-restart`, отсутствие
  метаданных, неверные цели, `service_unavailable`, rollback; `configs/` —
  `0700`, `customScripts.lua` — `0600`, полный `doctor` после деплоя без ошибок;
- `state deploy`: успех, `--no-restart`, восстановление `config.lua` с
  сохранением `tes3mp-server-default.cfg`, `banlist.json` и
  `customScripts.lua`, несовпадение активного пакета модов, state без
  `configs/config.lua` и старый state с cfg/banlist (`archive_invalid`),
  ошибки архива, rollback, lock, отсутствие переименований в data root
  (ops работает без записи в `root:root 0755`);
- `doctor`: пороги `disk` (512 MiB — `error`, 1 GiB — `warn`) и `no_space`
  в `backups create`/`mods deploy`;
- `staff set`: успех, dry-run, отсутствующий игрок, небезопасный путь, пустой
  `settings`;
- `config get/set`: таблицы в `read_only`, dry-run, валидация, symlink,
  человекочитаемый вывод.

## 4. Панель

`test/contract/panel` собирает бинарник панели и запускает его на эфемерном
HTTPS-порту с fixture-парой в режиме `files`. Проверяются:

- lifecycle: env/flags и derived paths, ACME fake-obtainer (IP SAN без
  Common Name, shortlived profile, wrong IP, renewal window, отказ продления),
  `files` pair, TLS ≥ 1.2, `/healthz`, HSTS, bind rollback, `SIGTERM`;
- auth: создание/reuse/corrupt `auth.json`, режимы `0600`, `reset-password`,
  первый вход с временным паролем, смена пароля, cookie-флаги, CSRF,
  rate limit;
- публичные маршруты: `/`, `/backups` с двумя колонками, скачивание `mods`,
  `state`, ZIP-сборка state+mods, пагинация, `/api/v1/server`,
  `/api/v1/mods/current`, `/api/v1/mods/current/archive`, path traversal,
  не-`.tar.gz`, отсутствующий файл, `Content-Disposition`;
- админские маршруты: дашборд, статус, логи, `doctor`, моды, состояния,
  конфиги, staff, upload, приватное скачивание state, restore из ZIP
  (`mods import --name` → `mods deploy` → `state import` → `state deploy`);
  отдельный state проверяется против активного пакета, несовпадение даёт `422`
  до мутаций;
- метрики: парсинг fixture-файлов `memory.current`, `cpu.stat`, `/proc/meminfo`
  и `/proc/stat`, расчёт CPU % игры и системы и `null` при остановленном
  сервере или пропуске больше 45 секунд, окна 1 ч/6 ч/24 ч и бакеты (≤360
  точек), `available=false` без cgroup, отсутствие записи на диск, SVG без
  inline `style=` и без внешних библиотек;
- встроенный UI: `/static/panel.css` отдаётся с `text/css`, все полные шаблоны
  подключают общий `head`, в шаблонах нет inline `style=`, активный раздел
  админки помечен `aria-current="page"` (включая POST-пути);
- шапка админки: строка `TES3MP <сервер> · panel <панель> · client <клиент>` на
  всех страницах, версия сервера из `<runtime-root>/.tes3mp-install-info`, `—`
  без маркера; маркер разбирает `internal/serverruntime` (размер, symlink,
  отсутствующий `TES3MP_VERSION`);
- ops-клиент: фиксированный argv через sudo, разбор ровно одного
  JSON-объекта, `OpsError`, маппинг exit code → HTTP, таймауты, лимиты
  stdout/stderr.

Панель не тестирует tar и CRC — это тесты ops. Панель тестирует, что она
правильно вызывает ops и правильно показывает результат. Тест на загрузку
большого файла проверяет, что память панели не растёт пропорционально размеру
upload. Bind на 443 и временный 80 проверяются на VM с
`CAP_NET_BIND_SERVICE`; unit-тесты используют эфемерные порты и не требуют root.

## 5. Клиент

- `internal/morrowind` проверяет поиск по реестру, Steam VDF, GOG и Flatpak,
  точное и регистронезависимое сравнение, наличие трёх ESM и read-only
  поведение.
- `internal/serverapi` проверяет строгий JSON, лимит 1 MiB, HTTPS (HTTP только
  для loopback), таймауты, три попытки на сеть/5xx, `no_mods`, запрет
  редиректа на другой хост и обязательный путь архива.
- `internal/config` проверяет версию, нормализацию `id`, очистку неверного
  `active_server`, права `0600`, атомарную запись и отказ на битый JSON.
- `internal/app` проверяет сценарии установки/обновления/запуска без Fyne,
  включая `NeedsSetup`, `CompleteSetup`, проверку рабочей папки и запись
  клиентского лога.
- `internal/mods`, `internal/fonts`, `internal/locale` проверяют кеш по
  имени/размеру/SHA256, безопасную распаковку, сборку `Data Files`, профиль
  `openmw.cfg`, глобальный overlay локализации и вставку `data=`,
  `openmw_font.xml` и идемпотентность.
- `internal/launcher` проверяет Windows-запуск, поиск Proton, создание prefix и
  symlink `C:\Morrowind`, подготовку активного `openmw.cfg` и формирование
  окружения.
- `internal/clientlog` проверяет ротацию, уровни и `Tail`; пароль сервера в
  лог не пишется по построению.
- `internal/ui` проверяет логику без окна: `footerVersion` собирает строку
  версий клиента и TES3MP для подвала.

GUI-тесты не гоняются в CI: Fyne требует дисплея. Логика живёт в `app` и
тестируется без окна. Ручной smoke клиента — обязательная часть релиза.

## 6. Фикстуры

Статических `testdata/` с игровыми файлами нет. `test/fixtures` генерирует
архивы в памяти и во временных каталогах:

- mods: minimal, scripts, `no-crc`, `bad-crc`, `mods-init`, неверный layout и
  опасные пути;
- state: пустой, полный, плоский `world/`, без `current.txt`, с чужим
  мод-паком;
- `current.txt`: канонический, только sha256, неверный hex, symlink;
- API-ответы: успешные и ошибочные envelope-ы.

Правила:

- имена и CRC детерминированы;
- tar-фикстуры собираются в тесте;
- «плохие» архивы описываются кодом, который их ломает, а не бинарным
  дампом;
- фикстуры не содержат пользовательских данных и файлов игры;
- retention-история генерируется программно с заданными mtime, а не лежит
  тысячами файлов в Git.

## 7. Failure injection

- **Нет места.** Fake writer возвращает `ENOSPC`; тест проверяет проброс через
  `WriteState`, CLI-тесты — `no_space`, exit 5 и сохранность старых файлов.
- **systemd недоступен.** Runner возвращает ошибку; `test/fault` проверяет
  `status` на реальном бинарнике с пустым `PATH` — exit 6
  `service_unavailable`, без `data` и без «running».
- **Битый gzip/tar.** `test/fault` подаёт не-gzip upload: `archive_invalid`,
  upload остаётся побайтно на месте, `backups/mods` пуст. Contract-тесты
  дополнительно проверяют staging и неизменность `current.txt`.
- **Лимиты архива.** Unit-тесты reader проверяют compressed, uncompressed и
  entries caps; `ExtractMods` завершается с `ErrLimit` на маленьком тестовом
  лимите.
- **Обрыв на середине.** `test/fault` убивает `backups create` через SIGKILL,
  когда `*.part` уже вырос: финального архива нет, `.part` остаётся, sha256
  живого мира не меняется, lock освобождён. Повторный запуск создаёт валидный
  архив.
- **Lock.** `test/fault` держит exclusive `flock` и проверяет exit 4 `locked`
  без создания `backups/`; unit- и contract-тесты покрывают shared lock,
  timeout и `--dry-run`.
- **Падение панели.** Сессии в памяти, после рестарта вход заново; публичные
  бэкапы, игра и таймер работают.
- **Падение backup-сервиса.** Ошибка видна в journald и `/admin/logs`; ручное
  `backups create` работает; следующий тик таймера проходит.
- **Пересечение timer-запусков.** Предыдущий `backups create` ещё держит
  lock; второй запуск не стартует или получает `locked`, но не плодит архивы.
- **Бюджет превышен.** Retention не удаляет месячные state и пакеты, указанные
  в state; JSON содержит `warnings`, `doctor` — `warn`.
- **Мало места.** `doctor` при свободных <512 MiB даёт `error`, панель
  блокирует мутации; `backups create` и `mods deploy` возвращают `no_space`,
  но `tes3mp.service` продолжает работать. После освобождения места `doctor`
  снова проходит.
- **Разные FS.** Upload и `backups/` на разных mount: `doctor` — `error`,
  `mods import` — `invalid_argument`, файлы не теряются.

## 8. Ресурсные тесты

- `test/resource` собирает реальный `tes3mp-easy-ops` и запускает его с
  `GOMEMLIMIT=64MiB`, `GOMAXPROCS=1`, `TZ=UTC`. `sudo` и `systemd-run` не
  используются.
- `MemoryMax=64M` применяется, только если `/sys/fs/cgroup` уже writable:
  тест создаёт leaf-cgroup напрямую, пишет `memory.max` и `memory.swap.max`,
  добавляет PID в `cgroup.procs` и после завершения читает `memory.peak`.
  Если cgroup read-only, тест не пропускается целиком: он логирует отсутствие
  MemoryMax и проверяет `GOMEMLIMIT` + RSS.
- `mods import` получает потоковый tar.gz с несжимаемым плагином 510 MiB
  (практически предел 512 MiB); `backups create` — мир с файлом 500 MiB.
  Оба входа генерируются потоково, без буфера размером с архив.
- Пик RSS дочернего процесса читается из `wait4`/`Rusage.Maxrss`, а не из
  `/proc` теста. Для каждой команды сравниваются маленький вход (8 MiB mods,
  1 MiB state) и большой. Порог: абсолютный RSS ≤ 128 MiB и рост ≤ 32 MiB
  относительно маленького входа.
- На 1 vCPU отдельно проверяется, что backup не вытесняет игру по CPU.
- Буфер метрик панели на 5760 сэмплов (~0.6 MiB) не выводит панель за
  `MemoryMax=64M`; при переполнении старые сэмплы вытесняются, аллокаций на
  каждый запрос нет.

## 9. Ручная приёмка

Сценарии на чистой VM **Debian 12 или Ubuntu 24.04, 1 vCPU, 512–640 MiB RAM,
512 MiB swap, 7 GiB NVMe**:

1. **Релиз.** Собранный `cmd/tes3mp-easy-pack` архив проходит `sha256sum -c`;
   распаковка на чистой VM и `bootstrap.sh` ставят оба бинарника, пользователей,
   sudoers, rootctl, systemd, journald, sysctl, runtime и base mods. Затем
   `doctor` без ошибок → панель по HTTPS → первый пароль → смена пароля.
2. **Автобэкап.** Таймер активен; через 15–20 минут появляется
   `state-*`; `backups list --type state` показывает его и `mods_archive`;
   при остановленной игре `--if-running` даёт `skipped`.
3. **Моды.** Загрузить архив → deploy → сервер перезапустился (если был
   запущен) → `doctor` без ошибок (особенно `ownership` и `configs`) → клиент
   видит `mods_sha256` и скачивает `mods-*`.
4. **Сохранения.** Создать сохранение → изменить мир и `config.lua` →
   восстановить одной кнопкой → мир и `config.lua` вернулись,
   `tes3mp-server-default.cfg` с паролем, `banlist.json` и `customScripts.lua`
   не изменились, сервер запущен, `state-predeploy-*` существует. Проверить
   сохранение со старым пакетом модов: пока пакет не активен, «Восстановить»
   всё равно доступна и сама вызывает `mods deploy`, затем `state deploy`;
   ошибка включения пакета останавливает восстановление. Старый state с
   cfg/banlist отклоняется.
5. **Игроки и банлист.** `/admin/players` показывает игроков с рангами и
   бейджем доступа; «Забанить» добавляет имя в `banlist.json`
   (`playerNames`), «Разбанить» убирает, повторное нажатие ничего не пишет;
   оба действия дают баннер «нужен рестарт». Textarea банлиста сохраняет оба
   списка и при отсутствии изменений показывает «Изменений нет».
6. **Публичные архивы.** Без логина `/backups` одновременно показывает две
   колонки: состояния и моды. У state видны дата и имя его `mods_archive`; при
   доступном пакете кнопка «state + моды» скачивает ZIP с обоими архивами.
   В скачанном state нет `tes3mp-server-default.cfg` и `banlist.json`. Старый
   state с этими файлами отдаёт `404` и напрямую, и через bundle. Список не
   раскрывает пути и логи; независимая пагинация работает на длинной истории.
7. **Retention.** Синтетическая история на 5 лет: месячные state живы, дневные и
   недельные прорежены, бюджет не превышен; пакет, указанный в state, не удалён;
   после `backups prune` исчезают только незащищённые mods.
8. **TLS.** Панель получает сертификат Let's Encrypt для публичного IP через
   HTTP-01; браузер не показывает предупреждение. После освобождения порта 80
   продление проходит в фоне без рестарта. `files` режим с готовой парой тоже
   поднимается.
9. **Отказ systemd.** Недоступный systemd или unit → панель показывает 503
   `service_unavailable`; после восстановления статус возвращается.
10. **Мало места.** Заполнить диск до порога → `doctor` `error`, мутации
   возвращают `no_space`, данные не теряются.
11. **Клиент.** Windows 10/11: бинарник + Morrowind → мастер из четырёх шагов →
    добавление сервера → игра. Шрифты после Finish качаются в фоне, и строка
    Installation показывает прогресс. Linux: Proton найден автоматически.
12. **Битый `current.txt`.** API отдаёт `404 no_mods` → клиент показывает
    ошибку установки и не запускает vanilla-сервер молча.
13. **Долгая история.** Большое число state-архивов: панель листает страницами,
    не строит одну гигантскую HTML-таблицу, память панели не растёт.
14. **Метрики.** Дашборд показывает RAM/CPU игры и системы за 1 ч/6 ч/24 ч;
    после остановки сервера линия игры рвётся, после рестарта панели история
    начинается заново; при недоступном cgroup — `available=false` без ошибок
    и без блокировки действий.

Критерий приёмки — не только «получилось», но и понятное сообщение при
неудаче, путь к логу и отсутствие потери данных.

Первый сценарий автоматизирован: на одноразовой VM запустите

```bash
sudo ./test/e2e/release-smoke.sh /tmp/tes3mp-easy-<version>-linux-amd64
```

Скрипт делает preflight, установку, проверяет бинарники, systemd-юниты и
таймеры, `/etc/tes3mp-easy/panel.env`, SAN сертификата Let's Encrypt для
публичного IP, отсутствие постоянного listener на `:80`, `doctor`, `/healthz`,
ручное создание state и таймер. Остальные сценарии требуют Morrowind/Proton и
браузера, поэтому остаются ручными.

## 10. CI

`.github/workflows/ci.yml` на каждый push и pull request:

```bash
bash -n bootstrap.sh
bash -n test/e2e/release-smoke.sh
sh -n rootctl/tes3mp-easy-rootctl
sudo apt-get update
sudo apt-get install -y libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols
sudo visudo -cf sudoers.d/tes3mp-easy
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go test -race ./...
go build -o /tmp/tes3mp-easy-pack ./cmd/tes3mp-easy-pack
go build -o /tmp/tes3mp-easy-ops ./cmd/tes3mp-easy-ops
go build -o /tmp/tes3mp-easy-panel ./cmd/tes3mp-easy-panel
go build -o /tmp/tes3mp-easy-client ./cmd/tes3mp-easy-client
go run ./cmd/tes3mp-easy-pack --version 0.0.0 --output /tmp/release-a
go run ./cmd/tes3mp-easy-pack --version 0.0.0 --output /tmp/release-b
cmp /tmp/release-a/*.tar.gz /tmp/release-b/*.tar.gz
```

- `test/fault` и `test/resource` входят в обычный `go test ./...`, но исключены
  из `-race` build-тегами. Cgroup `MemoryMax` — best-effort: без writable
  cgroup v2 проверка RSS продолжается.
- Contract-тесты ops и панели собирают и запускают реальные бинарники.
- Fyne GUI не запускается в CI; клиент собирается под Linux.
- Windows-клиент собирается на `windows-latest` в
  `.github/workflows/release.yml`; GUI smoke остаётся ручной проверкой по §9.
- `.github/workflows/release.yml` на тег `server-v<version>` собирает серверный
  архив и проверяет его точную воспроизводимость, на `client-v<version>` —
  Linux/Windows клиентские архивы. `.sha256` прикладывается к GitHub Release в
  обоих каналах. Тот же job `publish` переставляет движущийся тег
  `<channel>-latest` на выпущенный коммит, кладёт рядом копии архивов без
  версии в имени и пересчитывает их `.sha256`, поэтому ссылка «на последний
  релиз» канала не меняется; `test/install-smoke.sh` проверяет, что такие имена
  не сбивают выбор серверной версии в `install.sh`.
- `test/docs` проверяет локальные ссылки, валидность JSON-примеров,
  отсутствие дублированных схем, запрещённых полей и устаревших имён
  backend-ов.

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
