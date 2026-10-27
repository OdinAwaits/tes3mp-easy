# 02 — Общая архитектура

Документ описывает нативную схему на systemd. Нормативные форматы, retention и
коды — в `03-contracts.md`. Контейнерный рантайм не используется: нативная
схема даёт меньший фоновый расход и узкую модель привилегий.

Целевая платформа — Debian 12+/Ubuntu 24.04+ x86_64 с systemd; другие ОС —
best-effort и не входят в поддерживаемую конфигурацию.

## 1. Схема

```text
                                  VPS, 1 vCPU
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  Игрок ──UDP 25565──────────────► tes3mp.service ──пишет──► players/     │
│                                      ▲                       world/      │
│                                      │ читает configs/ mods/             │
│                                      │                                  │
│  Клиент игрока ──HTTPS 443──┐        │                                  │
│  Браузер админа ──HTTPS 443─┼──► panel.service ──sudo──► ops ──► files   │
│  Браузер игрока ──HTTPS 443─┘        │                    │              │
│                                      │                    └──► rootctl   │
│                                      │                         │        │
│                                      │                         ▼        │
│                                      │                  systemctl        │
│                                      │                  tes3mp.service   │
│                                      │                                  │
│  backup.timer ──5 мин──► backup.service ──exec──► ops ──пишет──► backups/ │
│                                                                         │
│  /tes3mp-easy/{configs,players,world,mods,backups,panel-data}           │
└─────────────────────────────────────────────────────────────────────────┘
```

Ключевая идея: панель — единственная точка входа для админа и клиента, но
единственный **административный писатель** — `ops`. Панель не трогает
`players/`, `world/`, `mods/`, `configs/` и `backups/` напрямую. Игровой сервер
и таймер бэкапов живут в отдельных systemd-юнитах и не зависят от панели.

## 2. Компоненты

| Компонент | Где | Роль | Что пишет |
|---|---|---|---|
| `tes3mp.service` | VPS, systemd, UDP 25565 | Игровой сервер | `players/`, `world/`, `configs/banlist.json` во время игры |
| `tes3mp-panel.service` | VPS, systemd, :443 (+ временный :80 для ACME) | HTTPS-панель, публичный API, админка | `panel-data/` |
| `tes3mp-easy-backup.timer` | VPS, systemd | Запускает state backup каждые 15 минут | — |
| `tes3mp-easy-backup.service` | VPS, systemd, oneshot | Один запуск `ops backups create --type state --if-running` | через `ops` в `backups/` |
| `tes3mp-easy-ops` | CLI, `/opt/tes3mp-easy/bin` | Все административные операции | `configs/`, `players/`, `world/`, `mods/`, `backups/`, `.locks/` |
| `tes3mp-easy-rootctl` | root-helper, `/usr/local/libexec` | Фиксированные `start/stop/restart tes3mp.service` | — |
| `tes3mp-easy-client` | ПК игрока | Мастер установки, моды, запуск | свою рабочую папку |
| systemd | VPS | Запуск, ресурсы, журнал, таймеры | journald |

Панель и ops — разные процессы. Панель вызывает ops только фиксированным
набором команд; ops не умеет HTTP и не читает пароль панели.

## 3. Systemd-юниты и лимиты

| Unit | User | Память | Дополнительно |
|---|---|---|---|
| `tes3mp.service` | `tes3mp` | `MemoryHigh=384M`, `MemoryMax=448M` | `OOMScoreAdjust=-500`, `Restart=on-failure` |
| `tes3mp-panel.service` | `tes3mp-panel` | `MemoryHigh=48M`, `MemoryMax=64M` | `GOMAXPROCS=1`, `GOMEMLIMIT=64MiB`, `OOMScoreAdjust=+500`, `CAP_NET_BIND_SERVICE`, bounding `CAP_SETUID`/`CAP_SETGID`/`CAP_AUDIT_WRITE`/`CAP_SYS_RESOURCE` для sudo, `EnvironmentFile=-/etc/tes3mp-easy/panel.env` |
| `tes3mp-easy-backup.service` | `tes3mp` | `MemoryHigh=48M`, `MemoryMax=64M` | `GOMAXPROCS=1`, `GOMEMLIMIT=64MiB`, `Nice=19`, `IOSchedulingClass=idle`, `OOMScoreAdjust=+500` |
| `tes3mp-easy-backup.timer` | — | — | `OnBootSec=2min`, `OnUnitActiveSec=15min`, `AccuracySec=30s` |

`tes3mp-easy-backup.service` — `Type=oneshot` с `TimeoutStartSec=9min`.
Если предыдущий запуск ещё активен, systemd не запускает второй экземпляр;
следующий тик произойдёт после завершения текущего. `--if-running` внутри
ops дополнительно пропускает state backup, если игра остановлена.

`tes3mp.service`:

- `WorkingDirectory=/tes3mp-easy`, `Environment=HOME=/tes3mp-easy`;
- `ExecStart=/opt/tes3mp-easy/tes3mp/tes3mp-server`;
- `Restart=on-failure`, `RestartSec=5s`;
- `ProtectSystem=strict`, `PrivateTmp=yes`, `ProtectHome=yes`;
- `ReadWritePaths=/tes3mp-easy/players /tes3mp-easy/world /tes3mp-easy/configs`;
  `configs/` нужен целиком, потому что `banlist.json` сервера — symlink в этот
  каталог; `customScripts.lua` и остальные конфиги сервер только читает;
- `NoNewPrivileges=yes`, `UMask=0027`: сервер не повышает привилегии, а его
  файлы создаются как `0640`, каталоги — `0750`;
- `SupplementaryGroups=systemd-journal` юниту не нужен: сервер пишет
  stdout/stderr; группа `systemd-journal` у пользователя `tes3mp` нужна только
  для `ops logs`;
- порт `25565/udp`, без `CAP_NET_BIND_SERVICE` (непривилегированный порт).

`tes3mp-panel.service`:

- `WorkingDirectory=/opt/tes3mp-easy`;
- `EnvironmentFile=-/etc/tes3mp-easy/panel.env` задаёт `TES3MP_PANEL_TLS_MODE=acme`
  и `TES3MP_PANEL_PUBLIC_ADDRESS`;
- `ExecStart=/opt/tes3mp-easy/bin/tes3mp-easy-panel`;
- `AmbientCapabilities=CAP_NET_BIND_SERVICE`, `CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE CAP_SYS_RESOURCE`;
  ambient capability нужна для `:443` и временного `:80` во время ACME HTTP-01;
  остальные capability доступны только setuid-`sudo`, который панель использует
  для запуска фиксированной команды `ops` от `tes3mp`: `CAP_SETUID`/`CAP_SETGID`
  переключают пользователя, `CAP_AUDIT_WRITE` пишет audit-запись, а
  `CAP_SYS_RESOURCE` позволяет sudo применить PAM-лимиты без предупреждений;
- `ProtectSystem=strict`, `PrivateTmp=yes`, `ProtectHome=yes`;
- `ReadWritePaths=/tes3mp-easy` — один bind mount, который нужен дочернему
  `ops`, запускаемому через `sudo`: mount namespace с `ProtectSystem=strict`
  наследуется, а Unix-права не дают панели писать в игровые каталоги напрямую.
  Один mount обязателен для deploy-транзакций: при отдельных
  `ReadWritePaths` для `.staging` и `mods/`, `players/`, `world/` Linux
  возвращает `EXDEV` на `rename` между ними. Не добавляйте granular-пути;
- `NoNewPrivileges=no` — иначе не сработает узкий `sudo`; граница безопасности —
  не этот флаг, а точные правила sudoers и root-helper;
- `ProtectControlGroups` и `ProtectKernelTunables` не включаются: панель читает
  `/sys/fs/cgroup/system.slice/tes3mp.service/{memory.current,cpu.stat}` и
  `/proc/{meminfo,stat}` для графиков, но никогда туда не пишет;
- `Restart=on-failure`, `RestartSec=60s`, `StartLimitBurst=2`: падение ACME не
  должно превращаться в шквал запросов к Let's Encrypt. Let's Encrypt считает
  пять неудачных проверок на hostname в час, поэтому после двух попыток юнит
  останавливается, а оператор исправляет `80/tcp` и перезапускает bootstrap.
- `TimeoutStartSec=5min` ограничивает запуск процесса; готовность HTTPS после
  первого выпуска сертификата проверяет `bootstrap.sh` (до 300 с), а не
  systemd, потому что юнит имеет `Type=simple`.

`tes3mp-easy-backup.service` не получает прав root и не управляет юнитами:
ему достаточно прочитать состояние (`systemctl is-active`) и записать архив от
пользователя `tes3mp`. Юнит запускает
`tes3mp-easy-ops --root /tes3mp-easy --json backups create --type state --if-running`,
имеет `NoNewPrivileges=yes`, `UMask=0027` и пишет только в
`backups/`, `.locks/` и `.staging/`. `ConditionPathExists` на
`/opt/tes3mp-easy/bin/tes3mp-easy-ops` позволяет включить таймер до установки
бинарника: срабатывания пропускаются, а первый тик после установки ops создаёт
state backup без ручного вмешательства.

Bootstrap устанавливает `tes3mp.service`, `tes3mp-easy-backup.timer` и, если в
релизном архиве есть `bin/tes3mp-easy-panel`, `tes3mp-panel.service`. Игра запускается через `start`,
поэтому повторный bootstrap не отключает игроков. Панель перезапускается, чтобы
подхватить заменённый бинарник; при первом релизе это создаёт
`initial-password.txt`. Source-установка без `bin/` оставляет панель выключенной
до ручной установки бинарника.

## 4. Пользователи, группы и sudoers

| Пользователь | Группы | Владеет | Может |
|---|---|---|---|
| `tes3mp` | `tes3mp`, `systemd-journal` | `configs/`, `players/`, `world/`, `mods/`, `backups/`, `.locks/`, `.staging/` | играть, писать state, вызывать rootctl |
| `tes3mp-panel` | `tes3mp-panel`, `tes3mp` | `panel-data/` | читать `backups/`, `players/` и `mods/`, запускать ops от `tes3mp` |
| `root` | — | `/opt`, `/etc/systemd`, `/usr/local/libexec` | установка и аварийное восстановление |

Права:

- `/tes3mp-easy` — `root:root 0755`;
- `configs/` — `tes3mp:tes3mp 0700`, файлы `0600`: панель не читает их
  напрямую, только через `ops`;
- `players/`, `world/`, `mods/`, `backups/` — `tes3mp:tes3mp 0750`, файлы
  `0640`;
- `panel-data/` — `tes3mp-panel:tes3mp 0750`;
- `panel-data/auth.json`, `panel-data/initial-password.txt` —
  `tes3mp-panel:tes3mp-panel 0600`;
- `panel-data/tls/` — `0700`, файлы `0600`;
- `panel-data/acme/` — `0700`, `account.key` — `0600`;
- `panel-data/uploads/` — `tes3mp-panel:tes3mp 2770`, файлы `0640`; группа
  `tes3mp` должна иметь write, иначе `ops` не сможет перенести upload в
  `backups/mods/` и удалить исходный файл;
- `.locks/`, `.staging/` — `tes3mp:tes3mp 0700`;
- `.config/openmw/`, `.local/share/openmw/` — `tes3mp:tes3mp 0700`: это
  `$HOME/.config/openmw` и `$HOME/.local/share/openmw` для prelaunch-скрипта;
  заранее созданные каталоги не требуют записи в `/tes3mp-easy` при
  `ProtectSystem=strict`.

`tes3mp-panel` входит в группу `tes3mp` для чтения `backups/`, `players/` и
`mods/`; `uploads/` — узкое исключение с group-write, нужное для переноса
upload. Панель не имеет Unix-прав писать в игровые каталоги; `ops`, запущенный
от `tes3mp`, имеет.

Sudoers — два узких правила:

```text
tes3mp-panel ALL=(tes3mp) NOPASSWD: /opt/tes3mp-easy/bin/tes3mp-easy-ops
tes3mp ALL=(root) NOPASSWD: /usr/local/libexec/tes3mp-easy-rootctl start, \
    /usr/local/libexec/tes3mp-easy-rootctl stop, \
    /usr/local/libexec/tes3mp-easy-rootctl restart
```

`rootctl` — root-owned скрипт или бинарник `root:tes3mp 0750`, который
принимает ровно один аргумент `start|stop|restart`, не принимает имя юнита и
вызывает `/bin/systemctl <verb> tes3mp.service` без shell. Группа `tes3mp`
нужна для execute-бита: sudo проверяет доступность команды вызывающему
пользователю до повышения прав. Произвольный `systemctl`, `sh -c`, путь или
дополнительный аргумент отклоняются.

Почему так: панель не работает под root, игровой сервер не получает права
управлять systemd, а ops не получает произвольный root-shell. Компромисс —
`sudo` внутри панели; он ограничен точным путём ops и точными аргументами
rootctl.

## 5. Layout на VPS

```text
/opt/tes3mp-easy/                 # immutable runtime and installed binaries
├── bin/
│   ├── tes3mp-easy-ops
│   └── tes3mp-easy-panel
└── tes3mp/                       # TES3MP 0.8.1, root:root 0755
    ├── .tes3mp-install-info      # версия, URL и sha256 установленного релиза
    ├── tes3mp-server
    └── lib/                      # библиотеки из релиза TES3MP

/tes3mp-easy/                     # данные; сам каталог root:root 0755
├── configs/
│   ├── tes3mp-server-default.cfg
│   ├── config.lua
│   ├── banlist.json
│   └── customScripts.lua         # bootstrap копирует базовый; ops обновляет
├── players/
├── world/
│   ├── cell/
│   ├── world/
│   ├── map/
│   ├── recordstore/
│   └── custom/
├── mods/
│   ├── plugins/
│   │   └── requiredDataFiles.json
│   └── scripts/
├── backups/
│   ├── mods/
│   │   ├── current.txt           # активный пакет модов
│   │   ├── mods-init-*.tar.gz    # базовый архив, создаёт bootstrap
│   │   └── mods-*.tar.gz         # неизменяемые пакеты
│   ├── state/
│   │   ├── state-*.tar.gz        # 15-минутный таймер
│   │   ├── state-manual-*.tar.gz
│   │   └── state-predeploy-*.tar.gz
├── panel-data/                   # tes3mp-panel:tes3mp
│   ├── auth.json
│   ├── initial-password.txt
│   ├── acme/
│   │   └── account.key
│   ├── tls/
│   └── uploads/                  # временные загрузки модов
├── .config/openmw/               # HOME-каталоги для tes3mp-prelaunch
├── .local/share/openmw/
├── .staging/                     # распаковка и подмена, чистится ops
└── .locks/
    └── ops.lock
```

`bootstrap.sh` из релизного архива создаёт `/opt/tes3mp-easy/bin/` и ставит
оба бинарника. Сам архив можно распаковать во временный каталог: unit-файлы,
sudoers, rootctl и sysctl копируются в системные пути, а дерево исходников в
`/opt/tes3mp-easy` не остаётся. Source-установка без `bin/` копирует checkout в
`/opt/tes3mp-easy` и оставляет каталог ручной установке бинарников.

В дереве `/opt/tes3mp-easy/tes3mp` изменяемые пути — symlinks в data root:
`tes3mp-server-default.cfg`, `server/scripts/{config.lua,customScripts.lua}`,
`server/scripts/custom/` и `server/data/{player,cell,world,map,recordstore,custom}/`,
`server/data/requiredDataFiles.json`, `server/data/banlist.json`. Поэтому
обновление или замена runtime не трогает конфиги, state и моды. Конфиги и
`requiredDataFiles.json` bootstrap создаёт только при отсутствии; существующий
файл он не перезаписывает.

`mods-init-*` — base-архив с `plugins/requiredDataFiles.json` (три ESM с пустыми
CRC-списками) и пустым `scripts/`; `current.txt` указывает на него в каноническом
формате `<sha256> <name>\n`. Повторный bootstrap принимает только канонический
`current.txt` с существующим архивом и совпадающим sha256, иначе останавливается,
не перезаписывая данные.

`panel-data/uploads` — единственное место, куда панель пишет пользовательские
файлы. После успешного `ops mods import` панель удаляет временный файл, если он
ещё существует. Uploads и `backups/` обязаны быть на одной файловой системе:
`mods import` делает rename, а при `EXDEV` — атомарное копирование. Это
проверяет `doctor`.

`EXDEV` возможен и при одинаковом `stat.Dev`: systemd реализует каждую строку
`ReadWritePaths` отдельным bind mount, а Linux запрещает rename между точками
монтирования. Поэтому одной проверки `st_dev` недостаточно. Панель запускает
`ops` в своём namespace, и для неё используется ровно один
`ReadWritePaths=/tes3mp-easy`: тогда `.staging`, `mods/`, `players/`, `world/`
и `backups/` находятся на одном mount, а deploy-транзакции могут делать
атомарный `rename`. Granular-пути в panel unit ломают `mods deploy` и
`state deploy`.

Системные файлы:

- `/etc/systemd/system/tes3mp.service`;
- `/etc/systemd/system/tes3mp-panel.service`;
- `/etc/systemd/system/tes3mp-easy-backup.service` и `.timer`;
- `/etc/sudoers.d/tes3mp-easy`;
- `/usr/local/libexec/tes3mp-easy-rootctl`;
- `/etc/systemd/journald.conf.d/tes3mp-easy.conf` с `SystemMaxUse=100M`,
  `RuntimeMaxUse=32M`;
- `/etc/sysctl.d/99-tes3mp-easy.conf` с `vm.swappiness=10`;
- `/swapfile` на 512 MiB и запись в `/etc/fstab`, если swap меньше 480 MiB.

## 6. Потоки

### Админ входит в панель

```text
Браузер ──HTTPS──► tes3mp-panel.service
  GET /admin/login
  POST /admin/login (пароль)
  панель проверяет Argon2id-хеш в panel-data/auth.json
  панель ставит HttpOnly/Secure/SameSite cookie + CSRF-токен
```

### Статус и управление сервером

```text
Браузер ──POST /admin/server/restart──► panel
  panel ──sudo -u tes3mp──► ops restart --json
  ops ──sudo rootctl restart──► systemctl restart tes3mp.service
  ops возвращает JSON
  panel показывает результат и обновляет статус
```

Чтение статуса не требует root: `ops status` вызывает `systemctl show` и
`systemctl is-active`, а `logs` читает journald через группу
`systemd-journal`.

### Метрики ресурсов

```text
panel goroutine раз в 15 с
  cgroup v2 tes3mp.service: memory.current, cpu.stat usage_usec
  /proc/meminfo, /proc/stat, statfs("/tes3mp-easy")
  → ring buffer 24 ч (память панели) → GET /api/v1/admin/metrics
  → шаблон /admin: SVG-графики, admin.js обновляет points
```

`ops` в сборе метрик не участвует: это чтение файлов, а не операция над
данными, и sudo на каждый сэмпл не нужен. Если cgroup недоступен, поля игры
будут `null` и линия на графике порвётся; системные RAM/CPU/диск остаются.
Статус, логи и мутации продолжают работать.

### Загрузка и деплой модов

```text
Браузер ──multipart upload──► panel
  panel стримит в panel-data/uploads/<uuid>.tar.gz (MaxBytesReader, 512 MiB)
  panel ──sudo -u tes3mp──► ops mods import --file ... --json
  ops проверяет архив, CRC, пути; rename в backups/mods/mods-*.tar.gz
  (штатный panel unit — один mount; при EXDEV — атомарное копирование)
  panel показывает список и предлагает Deploy

Браузер ──POST /admin/mods/deploy──► panel
  panel ──sudo -u tes3mp──► ops mods deploy --archive ... --json
  ops делает state-predeploy, останавливает tes3mp, распаковывает в staging,
  меняет mods/, генерирует customScripts.lua и current.txt,
  запускает tes3mp, если он работал до операции (и не указан --no-restart),
  затем выполняет full retention

Браузер ──POST /admin/state/deploy──► panel
  panel ──sudo -u tes3mp──► ops state deploy --archive ... --json
  ops проверяет, что current.txt состояния совпадает с активным пакетом модов,
  делает state-predeploy, останавливает tes3mp, распаковывает state в staging,
  переносит players/, world/ и три файла configs/, запускает tes3mp
```

`mods deploy` и `state deploy` подменяют содержимое каталогов через `rename`.
Панель запускает `ops` в своём namespace, поэтому `.staging` и целевые каталоги
обязаны быть на одном mount — это обеспечивает единственный
`ReadWritePaths=/tes3mp-easy`. Восстановление не переименовывает сами
`players/`, `world/` и `configs/`: корень принадлежит `root:root 0755`, а ops
работает от `tes3mp`; он переносит верхнеуровневые записи и отдельные файлы
внутри этих каталогов. `customScripts.lua` не входит в state и не меняется.

### State backup и восстановление

```text
tes3mp-easy-backup.timer ──каждые 15 минут──► tes3mp-easy-backup.service
  ops backups create --type state --if-running
  lock → проверка диска → current.txt первым → configs/ → requiredDataFiles.json →
  players/ + world/ → потоковый tar.gz во временный файл → fsync → rename →
  state-only retention → JSON с archive/pruned/warnings

Браузер ──POST /admin/state/create──► ops backups create --type state --manual
  тот же state-архив, ручное имя state-manual-*

Браузер ──POST /admin/mods/deploy──► ops mods deploy --archive <mods_archive>
  сначала активирует пакет, на который ссылается выбранное состояние

Браузер ──POST /admin/state/deploy──► ops state deploy --archive <state>
  ops: lock → проверка архива и активного пакета модов → state-predeploy →
       stop → staging → players/ + world/ + configs/ → start →
       при ошибке вернуть старое
```

State-архив не содержит модов: он ссылается на пакет через `current.txt`.
Пакеты неизменяемы, поэтому старую точку можно воспроизвести только с тем
пакетом, который записан в state. Панель не подставляет другой пакет: сначала
«Задеплоить моды», затем «Восстановить».

### Нехватка места

Отдельного периодического процесса нет: место проверяют `doctor` и операции
записи, которым нужен запас.

```text
doctor: statfs → ≥1 GiB ok → 512 MiB–1 GiB warn → <512 MiB error
backups create: <1 GiB → full retention → <512 MiB → no_space
mods deploy: резерв 512 MiB + распаковка → predeploy → <1 GiB → full retention
```

`doctor` в статусе `error` блокирует мутации в панели, но не останавливает игру:
автоматической остановки по диску нет. Если места не хватает, админ вручную
останавливает сервер, освобождает место через `backups prune`, `journalctl
--vacuum-size` или `rm`, затем запускает сервер снова. RAM ограничена
cgroup-лимитами, её потребление видно на графике дашборда; аварийной остановки
по памяти тоже нет. Удаление архивов — `rm` и retention.

### Клиент игрока

```text
Клиент ──GET /api/v1/server──────────► panel  (имя, адрес, статус, mods_sha256)
Клиент ──GET /api/v1/mods/current────► panel  (имя архива, sha256, размер, url)
Клиент ──GET /api/v1/mods/current/archive──► panel ──read──► backups/mods/*.tar.gz
Клиент ──UDP 25565──────────────────► tes3mp.service
```

Пароль сервера в публичном API не отдаётся. Админ передаёт его игроку
отдельно; клиент спрашивает пароль в мастере.

## 7. Границы безопасности

- **Публично:** главная страница, список и скачивание бэкапов, текущий архив
  модов, информация о сервере (без пароля).
- **Только с паролем:** всё под `/admin/*` и `/api/v1/admin/*`.
- **Нет прямого доступа к управлению хостом.** Панель не получает root,
  сокет управления или произвольный `systemctl`.
- **Никакого `sh -c`.** Ops запускает процессы через `exec.Command` с
  аргументами-массивом. Пути и имена проверяются до передачи.
- **Sudo — только два правила.** Панель может запускать только ops от
  `tes3mp`; ops может запускать только rootctl с тремя глаголами. Никакого
  произвольного `systemctl`, shell или пути.
- **Path traversal.** Все имена архивов и игроков проверяются: только
  `filepath.Base`, без `/`, `\`, `..` и NUL. Распаковка tar запрещает
  абсолютные пути, `..`, symlink и hardlink.
- **TLS.** Панель не работает по голому HTTP для админки. Встроенный ACME-клиент
  получает и продлевает сертификат Let's Encrypt для публичного IP (RFC 8738,
  профиль `shortlived`). Постоянно открыт только `:443`; `:80` занимается
  HTTP-01 challenge на время выпуска/продления.
- **CSRF и сессии.** Все изменяющие запросы — с CSRF-токеном. Cookie:
  `HttpOnly`, `Secure`, `SameSite=Strict`.
- **Rate limit.** Логин и публичные скачивания ограничены по IP.
- **Загрузки.** `MaxBytesReader` 512 MiB, потоковая запись, проверка gzip/tar
  до распаковки, лимиты записей и распакованного размера.
- **Journald.** `SystemMaxUse=100M`, `RuntimeMaxUse=32M`; логи не могут занять
  диск и вытеснить state.

## 8. Что происходит при сбоях

| Сбой | Поведение |
|---|---|
| Панель упала | `tes3mp.service` и таймер state backup продолжают работать |
| ACME не выпустил сертификат | Панель не открывает `:443`, пишет причину в journald; `bootstrap.sh` ждёт `/healthz` и завершается ошибкой |
| State backup упал | Ошибка видна в journald и `/admin/logs`; ручное создание работает, следующий тик таймера повторяет попытку |
| systemd недоступен | Панель отдаёт 503 `service_unavailable`, не изображает «running» |
| Кривой архив модов | `ops mods import` отклоняет до записи в `backups/mods/`; `mods deploy` — до подмены `mods/` |
| Ошибка при восстановлении | `state deploy` возвращает старые `players/`, `world/` и `config.lua`; `tes3mp-server-default.cfg`, `banlist.json` и `customScripts.lua` не меняются; состояние сервера прежнее |
| Нет пакета модов для state | `state deploy` возвращает `archive_invalid`; панель предлагает сначала задеплоить указанный пакет |
| Мало места (512 MiB–1 GiB) | `doctor` — `warn`; значение видно в метрике «свободно на диске», `backups create` сначала запускает full retention |
| Мало места (<512 MiB) | `doctor` — `error`, мутации в панели заблокированы; `backups create` и `mods deploy` возвращают `no_space`; игра продолжает работать |
| Бюджет архивов превышен | Retention удаляет state-кандидатов и незащищённые mods, месячные представители остаются, возвращается `warn` |
| OOM-killer сработал | Автоматической остановки по RAM нет: `Restart=on-failure` поднимает игру; всплеск виден на графике RAM, отдельного баннера и остановки нет |
| cgroup недоступен | Метрики отдают `available=false`, поля игры `null`; системные RAM/CPU/диск, статус и управление работают |
| Одновременные операции | `.locks/ops.lock`: вторая мутация ждёт или получает `locked` |
| Питание/падение в момент записи | Временный файл + rename; недописанный архив не становится `current` |

## 9. Ресурсный бюджет

### RAM и CPU

Расчётная VM: **1 vCPU, 512–640 MiB RAM, 512 MiB swap, 7 GiB NVMe**.
Если swap меньше 480 MiB, `bootstrap.sh` создаёт `/swapfile` на 512 MiB и
включает его до запуска сервисов.

| Потребитель | Обычно | Лимит |
|---|---|---|
| Базовая ОС, systemd, journald | 80–120 MiB | — |
| `tes3mp.service` | 250–380 MiB | `MemoryHigh=384M`, `MemoryMax=448M` |
| `tes3mp-panel.service` | 26–46 MiB | `MemoryHigh=48M`, `MemoryMax=64M` |
| `tes3mp-easy-backup.service` | 15–40 MiB | `MemoryHigh=48M`, `MemoryMax=64M` |
| Page cache | по остатку | вытесняется |

- `GOMAXPROCS=1` у панели и backup-сервиса: на одном vCPU больше не нужно.
- `GOMEMLIMIT=64MiB` заставляет Go-процессы агрессивнее собирать мусор, а не
  раздувать RSS.
- Кольцевой буфер метрик занимает ~0.6 MiB (5760 сэмплов по 15 секунд за
  24 часа): это учтено в строке панели и не пишется на диск.
- `tes3mp` получает `OOMScoreAdjust=-500`, панель и backup — `+500`:
  при нехватке памяти ядро убивает вспомогательные процессы, а не игру.
- `vm.swappiness=10`: swap — страховка от OOM, а не рабочая память. Нехватка
  swap не блокирует установку: bootstrap создаёт `/swapfile` на 512 MiB, а
  `doctor` сообщает об ошибке, только если swap так и остался меньше 480 MiB.
- 512 MiB — минимум; 640 MiB рекомендуется, если на сервере одновременно
  работают панель, импорт модов и игроки. При 512 MiB пики панели и backup не
  должны совпадать; `MemoryHigh` их притормаживает.
- Игровые значения по умолчанию: `maximumPlayers=8`, `maxClientsPerIP=3`.
  Для 2–5 друзей этого достаточно.

Контейнерный вариант не используется: он добавляет фоновые процессы и требует
доступа к сокету управления, что расширяет модель привилегий. Нативная схема
оставляет весь бюджет RAM и диска игре, панели и бэкапам.

### Диск

| Статья | Обычно | Бюджет |
|---|---|---|
| ОС, пакеты, runtime TES3MP | 1.35 GiB | — |
| Живые `players/` + `world/` | 0.2 GiB | 0.5 GiB |
| Живые `mods/` (распакованные) | 0.5 GiB | 1.0 GiB |
| Staging/upload (пик — больший из двух) | 1.0 GiB | 1.0 GiB |
| `backups/state` | 0.3 GiB | 1.0 GiB |
| `backups/mods` | 0.3 GiB | 0.5 GiB |
| `panel-data` (auth, TLS) | 0.05 GiB | — |
| journald | 0.1 GiB | 0.1 GiB |
| Свободный резерв | 0.5 GiB | 0.5 GiB |
| **Итого** | **~4.3 GiB** | **~6.0 GiB из 7** |

- Бюджеты — верхние границы, а не ожидаемое потребление. `state` растёт каждые
  15 минут, но дневные/недельные/месячные представители ограничивают историю;
  `mods` меняются только при импорте и защищаются ссылками из state.
- Upload (512 MiB) и staging (1 GiB) не обязаны существовать одновременно:
  импорт заканчивается до деплоя, а upload удаляется после импорта.
- `doctor` предупреждает при свободном месте меньше 1 GiB и считает ошибкой
  меньше 512 MiB. `backups create` требует резерв 512 MiB, `mods deploy` — тот
  же резерв плюс место на распаковку и predeploy; иначе `no_space`; игру никто
  автоматически не останавливает.
- Сжатые state-архивы крошечные (типично ~150 KB + `config.lua`), поэтому 1 GiB —
  это больше трёх лет разреженной истории. Месячные представители не удаляются
  по возрасту.
- Если бюджеты всё же превышены, retention включает pressure pruning; когда
  защищённых state больше, чем позволяет диск, `doctor` показывает `warn`, а
  админ освобождает место через `backups prune` или копирует историю на другую
  машину.
- `mods import` не копирует между файловыми системами: upload и `backups/`
  должны быть на одном mount, иначе `doctor` — `error`, а импорт —
  `invalid_argument`.

## 10. Non-goals архитектуры

- мультитенантность и несколько серверов на панель;
- HA, репликация, внешняя БД;
- микросервисы;
- plugin API для панели;
- хранение игровых данных в облаке;
- управление сервером из клиента;
- произвольный доступ к файлам через панель;
- контейнерный рантайм и альтернативные backend-ы.
