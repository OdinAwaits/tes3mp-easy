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
| `tes3mp-panel.service` | VPS, systemd, :80/:443 | HTTPS-панель, публичный API, админка | `panel-data/` |
| `tes3mp-easy-backup.timer` | VPS, systemd | Запускает бэкап каждые 5 минут | — |
| `tes3mp-easy-backup.service` | VPS, systemd, oneshot | Один запуск `ops backups create` | через `ops` в `backups/` |
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
| `tes3mp-panel.service` | `tes3mp-panel` | `MemoryHigh=48M`, `MemoryMax=64M` | `GOMAXPROCS=1`, `GOMEMLIMIT=64MiB`, `OOMScoreAdjust=+500`, `CAP_NET_BIND_SERVICE` |
| `tes3mp-easy-backup.service` | `tes3mp` | `MemoryHigh=48M`, `MemoryMax=64M` | `GOMAXPROCS=1`, `GOMEMLIMIT=64MiB`, `Nice=19`, `IOSchedulingClass=idle`, `OOMScoreAdjust=+500` |
| `tes3mp-easy-backup.timer` | — | — | `OnBootSec=2min`, `OnUnitActiveSec=5min`, `AccuracySec=30s` |

`tes3mp-easy-backup.service` — `Type=oneshot` с `TimeoutStartSec=9min`.
Если предыдущий запуск ещё активен, systemd не запускает второй экземпляр;
следующий тик произойдёт после завершения текущего. `--if-running` внутри
ops дополнительно пропускает бэкап, если игра остановлена.

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
- `ExecStart=/opt/tes3mp-easy/bin/tes3mp-easy-panel`;
- `AmbientCapabilities=CAP_NET_BIND_SERVICE`, `CapabilityBoundingSet=CAP_NET_BIND_SERVICE`;
- `ProtectSystem=strict`, `PrivateTmp=yes`, `ProtectHome=yes`;
- `ReadWritePaths=/tes3mp-easy/panel-data`;
- `ReadWritePaths=/tes3mp-easy/players /tes3mp-easy/world /tes3mp-easy/mods`;
- `ReadWritePaths=/tes3mp-easy/backups /tes3mp-easy/configs /tes3mp-easy/.locks /tes3mp-easy/.staging`;
  эти пути нужны дочернему `ops`, который панель запускает через `sudo`:
  mount namespace с `ProtectSystem=strict` наследуется, а Unix-права не дают
  панели писать туда напрямую;
- `NoNewPrivileges=no` — иначе не сработает узкий `sudo`; граница безопасности —
  не этот флаг, а точные правила sudoers и root-helper;
- `Restart=on-failure`, `RestartSec=3s`.

`tes3mp-easy-backup.service` не получает прав root и не управляет юнитами:
ему достаточно прочитать состояние (`systemctl is-active`) и записать архив от
пользователя `tes3mp`. Юнит запускает
`tes3mp-easy-ops --root /tes3mp-easy --json backups create --type state
--if-running`, имеет `NoNewPrivileges=yes`, `UMask=0027` и пишет только в
`backups/`, `.locks/` и `.staging/`. `ConditionPathExists` на
`/opt/tes3mp-easy/bin/tes3mp-easy-ops` позволяет включить таймер до установки
бинарника: срабатывания пропускаются, а первый тик после установки ops создаёт
бэкап без ручного вмешательства.

Bootstrap включает и запускает `tes3mp.service` и
`tes3mp-easy-backup.timer`. `tes3mp-panel.service` устанавливается, но не
включается и не запускается: панель включают после установки её бинарника.
Повторный bootstrap использует `start`, а не `restart`, поэтому не отключает
игроков.

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
- `panel-data/auth.json`, `panel-data/initial-password.txt` — `0600`;
- `panel-data/tls/` — `0700`, файлы `0600`;
- `panel-data/uploads/` — `tes3mp-panel:tes3mp 2770`, файлы `0640`; группа
  `tes3mp` должна иметь write, иначе `ops` не сможет сделать rename upload в
  `backups/mods/`;
- `.locks/`, `.staging/` — `tes3mp:tes3mp 0700`;
- `.config/openmw/`, `.local/share/openmw/` — `tes3mp:tes3mp 0700`: это
  `$HOME/.config/openmw` и `$HOME/.local/share/openmw` для prelaunch-скрипта;
  заранее созданные каталоги не требуют записи в `/tes3mp-easy` при
  `ProtectSystem=strict`.

`tes3mp-panel` входит в группу `tes3mp` для чтения `backups/`, `players/` и
`mods/`; `uploads/` — узкое исключение с group-write, нужное для rename.
Панель не имеет Unix-прав писать в игровые каталоги; `ops`, запущенный от
`tes3mp`, имеет.

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
/opt/tes3mp-easy/                 # неизменяемые файлы релиза, root:root
├── bin/
│   ├── tes3mp-easy-ops
│   └── tes3mp-easy-panel
├── tes3mp/                       # TES3MP 0.8.1, root:root 0755
│   ├── .tes3mp-install-info      # версия, URL и sha256 установленного релиза
│   ├── tes3mp-server
│   └── lib/                      # библиотеки из релиза TES3MP
├── systemd/
│   ├── tes3mp.service
│   ├── tes3mp-panel.service
│   ├── tes3mp-easy-backup.service
│   ├── tes3mp-easy-backup.timer
│   └── journald.conf.d/
│       └── tes3mp-easy.conf
├── sysctl.d/
│   └── 99-tes3mp-easy.conf
├── sudoers.d/
├── rootctl/
│   └── tes3mp-easy-rootctl
└── bootstrap.sh

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
│   │   ├── current.txt
│   │   ├── mods-init-*.tar.gz    # базовый архив, создаёт bootstrap
│   │   └── mods-*.tar.gz
│   └── state/
│       ├── state-*.tar.gz
│       ├── state-manual-*.tar.gz
│       └── state-predeploy-*.tar.gz
├── panel-data/                   # tes3mp-panel:tes3mp
│   ├── auth.json
│   ├── initial-password.txt
│   ├── tls/
│   └── uploads/                  # временные загрузки модов
├── .config/openmw/               # HOME-каталоги для tes3mp-prelaunch
├── .local/share/openmw/
├── .staging/                     # распаковка и подмена, чистится ops
└── .locks/
    └── ops.lock
```

`/opt/tes3mp-easy/bin/` не создаётся bootstrap: каталог и бинарники появляются
при установке `tes3mp-easy-ops` и `tes3mp-easy-panel`.

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
файлы. После успешного `ops mods import` панель удаляет временный файл.
Uploads и `backups/` обязаны быть на одной файловой системе: `mods import`
делает rename, а не копирование. Это проверяет `doctor`.

Системные файлы:

- `/etc/systemd/system/tes3mp.service`;
- `/etc/systemd/system/tes3mp-panel.service`;
- `/etc/systemd/system/tes3mp-easy-backup.service` и `.timer`;
- `/etc/sudoers.d/tes3mp-easy`;
- `/usr/local/libexec/tes3mp-easy-rootctl`;
- `/etc/systemd/journald.conf.d/tes3mp-easy.conf` с `SystemMaxUse=100M`,
  `RuntimeMaxUse=32M`;
- `/etc/sysctl.d/99-tes3mp-easy.conf` с `vm.swappiness=10`.

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

### Загрузка и деплой модов

```text
Браузер ──multipart upload──► panel
  panel стримит в panel-data/uploads/<uuid>.tar.gz (MaxBytesReader, 512 MiB)
  panel ──sudo -u tes3mp──► ops mods import --file ... --json
  ops проверяет архив, CRC, пути; rename в backups/mods/mods-*.tar.gz
  panel показывает список и предлагает Deploy

Браузер ──POST /admin/mods/deploy──► panel
  panel ──sudo -u tes3mp──► ops mods deploy --archive ... --json
  ops делает state-predeploy, останавливает tes3mp, распаковывает в staging,
  меняет mods/, генерирует customScripts.lua и current.txt, запускает tes3mp,
  если он работал до операции (и не указан --no-restart)
```

### State-бэкап и восстановление

```text
tes3mp-easy-backup.timer ──каждые 5 минут──► tes3mp-easy-backup.service
  ops backups create --type state --if-running
  lock → проверка диска → потоковый tar.gz во временный файл → fsync → rename
  → retention → JSON с pruned/warnings

Браузер ──POST /admin/state/deploy──► panel ──sudo -u tes3mp──► ops state deploy
  ops: lock → проверка архива → state-predeploy → stop → staging → подмена
       players/ и world/ → start, если был запущен → при ошибке вернуть старые
```

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
- **TLS.** Панель не работает по голому HTTP для админки. Целевой режим —
  сертификат Let’s Encrypt по IP; self-signed — аварийный режим с явным
  предупреждением.
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
| Панель упала | `tes3mp.service` и таймер бэкапов продолжают работать |
| Бэкап упал | Панель показывает предупреждение; ручной `backups create` работает |
| systemd недоступен | Панель отдаёт 503 `service_unavailable`, не изображает «running» |
| Кривой архив модов | `ops mods import` отклоняет до записи в `backups/mods/`; `mods deploy` — до подмены `mods/` |
| Ошибка при деплое state | Ops возвращает старые `players/` и `world/` и прежнее состояние сервера |
| Мало места | Ops проверяет диск до упаковки/распаковки и возвращает `no_space` |
| Бюджет бэкапов превышен | Retention удаляет кандидатов; месячные не трогает, возвращает `warn` |
| Одновременные операции | `.locks/ops.lock`: вторая мутация ждёт или получает `locked` |
| Питание/падение в момент записи | Временный файл + rename; недописанный архив не становится `current` |

## 9. Ресурсный бюджет

### RAM и CPU

Расчётная VM: **1 vCPU, 512–640 MiB RAM, 512 MiB swap, 7 GiB NVMe**.

| Потребитель | Обычно | Лимит |
|---|---|---|
| Базовая ОС, systemd, journald | 80–120 MiB | — |
| `tes3mp.service` | 250–380 MiB | `MemoryHigh=384M`, `MemoryMax=448M` |
| `tes3mp-panel.service` | 25–45 MiB | `MemoryHigh=48M`, `MemoryMax=64M` |
| `tes3mp-easy-backup.service` | 15–40 MiB | `MemoryHigh=48M`, `MemoryMax=64M` |
| Page cache | по остатку | вытесняется |

- `GOMAXPROCS=1` у панели и backup-сервиса: на одном vCPU больше не нужно.
- `GOMEMLIMIT=64MiB` заставляет Go-процессы агрессивнее собирать мусор, а не
  раздувать RSS.
- `tes3mp` получает `OOMScoreAdjust=-500`, панель и backup — `+500`: при
  нехватке памяти ядро убивает вспомогательные процессы, а не игру.
- `vm.swappiness=10`: swap — страховка от OOM, а не рабочая память.
- 512 MiB — минимум; 640 MiB рекомендуется, если на сервере одновременно
  работают панель, импорт модов и игроки. При 512 MiB пики панели и backup не
  должны совпадать; `MemoryHigh` их притормаживает.
- Игровые значения по умолчанию: `maximumPlayers=8`, `maxClientsPerIP=3`.
  Для 2–5 друзей этого достаточно.

Контейнерный вариант не используется: он добавляет фоновые процессы и требует
доступа к сокету управления, что расширяет модель привилегий. Нативная схема
оставляет весь бюджет RAM и диска игре, панели и бэкапам.

### Диск

| Статья | Бюджет |
|---|---|
| ОС, пакеты, runtime TES3MP | 1.35 GiB |
| Живые `players/` + `world/` | 0.5 GiB |
| Живые `mods/` (распакованные) | 1.0 GiB |
| Staging/upload (пик — больший из двух) | 1.0 GiB |
| `backups/state` | 1.0 GiB |
| `backups/mods` | 0.5 GiB |
| `panel-data` (auth, TLS) | 0.05 GiB |
| journald | 0.1 GiB |
| Свободный резерв | 0.5 GiB |
| **Пик** | **~6.0 GiB из 7** |

- Upload (512 MiB) и staging (1 GiB) не обязаны существовать одновременно:
  импорт заканчивается до деплоя, а upload удаляется после импорта.
- `doctor` предупреждает при свободном месте меньше 1 GiB и считает ошибкой
  меньше 512 MiB.
- Сжатые state-архивы крошечные (типично ~100 KB), поэтому 1 GiB — это
  несколько тысяч точек; месячные архивы не удаляются по возрасту.
- Если месячные архивы сами превысили 1 GiB, retention не удаляет их;
  `doctor` показывает `warn`, а админ копирует историю на другую машину.
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
