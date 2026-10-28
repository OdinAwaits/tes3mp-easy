# 08 — Runbook

Runbook для нативной установки на Debian/Ubuntu x86_64 с systemd. Все
команды — от пользователя с sudo. Если не сказано иное, `ROOT=/tes3mp-easy`,
`OPS=/opt/tes3mp-easy/bin/tes3mp-easy-ops`.

## 1. Требования

- Debian 12+ или Ubuntu 24.04+ x86_64 с systemd;
- 1 vCPU;
- 512 MiB RAM минимум, 640 MiB рекомендуется;
- 512 MiB swap; если swap меньше 480 MiB, bootstrap создаст `/swapfile` на
  512 MiB и включит его;
- 7 GiB NVMe/SSD;
- открытые `25565/udp`, `443/tcp` и `80/tcp` (только на время ACME HTTP-01);
- root/sudo и доступ по SSH;
- `curl`, `openssl`, `tar`, `gzip`, `sha256sum`;
- Go 1.24+ и `git` — только для сборки релиза или установки из исходников;
- время синхронизировано (systemd-timesyncd).

Другие ОС — best-effort и не входят в поддерживаемую конфигурацию.
Контейнерный рантайм не используется и не требуется.

Проверка:

```bash
nproc
free -m
swapon --show
df -h /
systemctl --version
timedatectl
```

Если RAM 512 MiB, не запускайте одновременно импорт модов и тяжёлые
операции; `MemoryHigh` притормозит панель/backup, но игру не спасёт от
нехватки памяти.

## 2. Установка

### 2.1. Из релизного архива

Рекомендуемый путь: на VPS не нужны Go и исходники.

Самый короткий путь — установщик скачивает последний серверный архив,
проверяет SHA256 и запускает `bootstrap.sh`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/install.sh)
```

Конкретную версию серверного релиза можно закрепить:

```bash
TES3MP_EASY_VERSION=0.2.4 bash <(curl -Ls https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/install.sh)
```

Ручной путь, если архив нужно скачать заранее:

1. Скачайте архив и checksum с GitHub Releases (или возьмите локально
   собранный `dist/tes3mp-easy-<version>-linux-amd64.tar.gz`). Серверные
   релизы помечены тегом `server-v<version>`:

   ```bash
   VERSION=0.2.4
   curl -fsSLO "https://github.com/Saagilat/tes3mp-easy/releases/download/server-v${VERSION}/tes3mp-easy-${VERSION}-linux-amd64.tar.gz"
   curl -fsSLO "https://github.com/Saagilat/tes3mp-easy/releases/download/server-v${VERSION}/tes3mp-easy-${VERSION}-linux-amd64.tar.gz.sha256"
   sha256sum -c "tes3mp-easy-${VERSION}-linux-amd64.tar.gz.sha256"
   tar -xzf "tes3mp-easy-${VERSION}-linux-amd64.tar.gz"
   cd "tes3mp-easy-${VERSION}-linux-amd64"
   ```

   Локальная сборка релиза:

   ```bash
   go run ./cmd/tes3mp-easy-pack --version 0.2.4
   ```

2. Проверьте хост без изменений:

   ```bash
   sudo ./bootstrap.sh --check
   ```

3. Запустите установку:

   ```bash
   sudo ./bootstrap.sh
   ```

   Bootstrap выполняет preflight, ставит пользователей, группы, каталоги,
   rootctl, sudoers, systemd-юниты, journald drop-in, sysctl, TES3MP 0.8.1,
   базовые конфиги и base mod-пак. Из каталога `bin/` он устанавливает
   `tes3mp-easy-ops` и `tes3mp-easy-panel` в `/opt/tes3mp-easy/bin`, затем
   включает и запускает `tes3mp.service`, `tes3mp-easy-backup.timer` и
   `tes3mp-panel.service`. Исходники и Go на VPS не нужны.

4. Войдите в панель. Bootstrap печатает временный пароль в конце вывода
   (`[panel] temporary password: ...`), а тот же пароль лежит в файле:

   ```bash
   sudo cat /tes3mp-easy/panel-data/initial-password.txt
   ```

   Откройте `https://<IP>/`, войдите с временным паролем и сразу смените его.
   После успешной смены панель удаляет `initial-password.txt`; если файл
   остался, удалите его вручную.

5. Проверьте:

   ```bash
   sudo systemctl status tes3mp.service --no-pager
   sudo systemctl status tes3mp-easy-backup.timer --no-pager
   sudo systemctl status tes3mp-panel.service --no-pager
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   curl -kfsS --noproxy '*' https://127.0.0.1/healthz
   ```

   Ожидаемо: игра и панель `active (running)`, таймер `active (waiting)`,
   `doctor` без `error`, `/healthz` возвращает `ok`.

### 2.2. Из исходников (developer/emergency)

Этот путь нужен для git checkout: bootstrap подготовит хост и runtime, но не
установит бинарники.

1. Скопируйте исходники проекта в `/opt/tes3mp-easy`:

   ```bash
   rm -rf /tmp/tes3mp-easy
   git clone https://github.com/Saagilat/tes3mp-easy.git /tmp/tes3mp-easy
   sudo mkdir -p /opt/tes3mp-easy
   sudo cp -a /tmp/tes3mp-easy/. /opt/tes3mp-easy/
   sudo rm -rf /opt/tes3mp-easy/.git
   sudo chown -R root:root /opt/tes3mp-easy
   ```

2. Запустите bootstrap:

   ```bash
   cd /opt/tes3mp-easy
   sudo ./bootstrap.sh
   ```

   Проверить хост без изменений можно отдельно:

   ```bash
   sudo ./bootstrap.sh --check
   ```

   `bootstrap.sh` выполняет preflight, ставит пользователей, группы, каталоги,
   rootctl, sudoers, systemd-юниты, journald drop-in, sysctl, TES3MP 0.8.1,
   базовые конфиги и base mod-пак, а затем включает и запускает
   `tes3mp.service` и `tes3mp-easy-backup.timer`. Каталога `bin/` в git
   checkout нет, поэтому панель остаётся выключенной до шага 3.

Source bootstrap:

- проверяет Debian/Ubuntu, x86_64, systemd, RAM/swap/диск; нехватка swap — это
  warning, после которого создаётся `/swapfile` на 512 MiB;
- ставит пакеты: `libluajit-5.1-2`, `ca-certificates`, `openssl`, `curl`,
  `tar`, `gzip`, `rsync`, `coreutils`, `iproute2`;
- создаёт пользователей `tes3mp` и `tes3mp-panel`, группы, каталоги и права;
- скачивает TES3MP 0.8.1 из `server/tes3mp-version.txt`, проверяет sha256,
  распаковывает staging, делает его `root:root`, пишет маркер
  `.tes3mp-install-info` и атомарно переименовывает в
  `/opt/tes3mp-easy/tes3mp`;
- связывает изменяемые пути runtime с `/tes3mp-easy` (конфиги, `players/`,
  `world/`, `mods/scripts`, `requiredDataFiles.json`, `banlist.json`);
- создаёт базовые `tes3mp-server-default.cfg`, `config.lua`,
  `customScripts.lua`, `banlist.json` и `requiredDataFiles.json`, не
  перезаписывая существующие;
- создаёт `mods-init-*`, канонический `current.txt` и HOME-каталоги
  `.config/openmw`, `.local/share/openmw`;
- ставит `tes3mp.service`, `tes3mp-panel.service`,
  `tes3mp-easy-backup.service`, `tes3mp-easy-backup.timer`;
- если рядом с `bootstrap.sh` есть `bin/`, ставит оба бинарника в
  `/opt/tes3mp-easy/bin` и проверяет, что они регулярные файлы;
- ставит `/usr/local/libexec/tes3mp-easy-rootctl` и
  `/etc/sudoers.d/tes3mp-easy`;
- настраивает journald (`SystemMaxUse=100M`, `RuntimeMaxUse=32M`) и
  `vm.swappiness=10`;
- включает в автозагрузку и запускает `tes3mp.service` и
  `tes3mp-easy-backup.timer`, затем проверяет
  `is-active`; при наличии установленной панели определяет публичный IPv4,
  проверяет свободный `80/tcp`, пишет `/etc/tes3mp-easy/panel.env`, включает и
  перезапускает `tes3mp-panel.service` и ждёт HTTPS `/healthz` (включая первый
  ACME-выпуск).

3. Соберите и установите серверные бинарники из каталога `/opt/tes3mp-easy`.
   Сборку можно выполнить на этой же машине или кросс-компиляцией на другой:

   ```bash
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
     go build -trimpath -o /tmp/tes3mp-easy-ops ./cmd/tes3mp-easy-ops
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
     go build -trimpath -o /tmp/tes3mp-easy-panel ./cmd/tes3mp-easy-panel

   sudo install -d -o root -g root -m 0755 /opt/tes3mp-easy/bin
   sudo install -o root -g root -m 0755 /tmp/tes3mp-easy-ops \
     /opt/tes3mp-easy/bin/tes3mp-easy-ops
   sudo install -o root -g root -m 0755 /tmp/tes3mp-easy-panel \
     /opt/tes3mp-easy/bin/tes3mp-easy-panel
   ```

   Пока `ops` не установлен, backup service пропускается по
   `ConditionPathExists` и таймер остаётся здоровым. После установки `ops`
   первый тик создаёт первый архив; можно не ждать и запустить
   `sudo systemctl start tes3mp-easy-backup.service`.

4. Настройте TLS и включите панель. Повторный bootstrap увидит установленный
   бинарник, определит публичный IPv4, проверит `80/tcp`, запишет
   `/etc/tes3mp-easy/panel.env` и запустит панель:

   ```bash
   sudo /opt/tes3mp-easy/bootstrap.sh
   sudo cat /tes3mp-easy/panel-data/initial-password.txt
   ```

   Вместо повторного bootstrap можно создать env-файл вручную:

   ```bash
   sudo install -d -o root -g tes3mp-panel -m 0750 /etc/tes3mp-easy
   printf 'TES3MP_PANEL_TLS_MODE=acme\nTES3MP_PANEL_PUBLIC_ADDRESS=%s\n' \
     '<IP>' | sudo tee /etc/tes3mp-easy/panel.env >/dev/null
   sudo chown root:tes3mp-panel /etc/tes3mp-easy/panel.env
   sudo chmod 0640 /etc/tes3mp-easy/panel.env
   sudo systemctl enable --now tes3mp-panel.service
   ```

   `initial-password.txt` создаётся при первом старте панели. Откройте
   `https://<IP>/`, войдите с временным паролем и сразу смените его. После
   успешной смены панель удаляет `initial-password.txt`; если файл остался,
   удалите его вручную.

5. Проверьте:

   ```bash
   sudo systemctl status tes3mp.service --no-pager
   sudo systemctl status tes3mp-easy-backup.timer --no-pager
   sudo systemctl list-timers tes3mp-easy-backup.timer
   sudo systemctl status tes3mp-panel.service --no-pager
   ```

   Ожидаемо: сервер `active (running)`, таймер `active (waiting)`, панель
   `active (running)` после шага 4. Если backup service был пропущен, проверьте,
   установлен ли `/opt/tes3mp-easy/bin/tes3mp-easy-ops`.

## 3. Первый запуск и проверка

Bootstrap включает и запускает `tes3mp.service` и таймер автобэкапов. При
установке из релизного архива он также запускает панель; при source-установке
панель включается отдельно после установки её бинарника. Runtime можно проверить
отдельно:

```bash
sudo cat /opt/tes3mp-easy/tes3mp/.tes3mp-install-info
readlink /opt/tes3mp-easy/tes3mp/server/data/player
readlink /opt/tes3mp-easy/tes3mp/server/data/banlist.json
cd /tes3mp-easy/backups/mods
read -r sha name < current.txt
printf '%s  %s\n' "$sha" "$name" | sha256sum -c -
```

Ожидаемо: маркер содержит `TES3MP_VERSION=0.8.1` и sha256 из
`server/tes3mp-version.txt`; symlinks ведут в `/tes3mp-easy`; `sha256sum -c`
печатает `OK`. Статическая проверка юнитов:

```bash
systemd-analyze verify /etc/systemd/system/tes3mp.service
systemd-analyze verify /etc/systemd/system/tes3mp-panel.service
systemd-analyze verify /etc/systemd/system/tes3mp-easy-backup.service
systemd-analyze verify /etc/systemd/system/tes3mp-easy-backup.timer
systemctl cat tes3mp-easy-backup.timer
```

`tes3mp.service`, backup service и панель проходят verify без ошибки об
`ExecStart` после установки соответствующих бинарников. Если бинарник ещё не
установлен, сообщение об отсутствующем `ExecStart` ожидаемо. Любые
синтаксические, dependency- или path-ошибки недопустимы. Ручной запуск, если
сервер или таймер был остановлен:

```bash
sudo systemctl start tes3mp.service
sudo systemctl enable --now tes3mp-easy-backup.timer
```

Проверка текущего состояния:

```bash
systemctl is-active tes3mp.service
systemctl is-enabled tes3mp.service
systemctl is-active tes3mp-easy-backup.timer
systemctl is-enabled tes3mp-easy-backup.timer
journalctl -u tes3mp.service -n 50 --no-pager
```

Ожидаемо:

- `tes3mp.service` — `active` и `enabled`;
- `tes3mp-easy-backup.timer` — `active` и `enabled`, следующий запуск через
  15 минут;
- `journalctl -u tes3mp.service -n 50` показывает запуск сервера;
- без установленного `ops` backup service пропускается по
  `ConditionPathExists`, без `failed`;
- `ops status`, публичный `/healthz` и `/api/v1/server` доступны после
  установки ops и запуска панели.

Проверка ресурсов:

```bash
systemctl show tes3mp.service -p MemoryCurrent -p MemoryMax
free -m
systemd-cgtop -1
```

`systemctl show tes3mp-panel.service` имеет смысл после включения панели.
Дашборд `/admin` показывает текущие RAM/CPU игры, RAM/swap системы, свободный
диск и графики за 1 ч/6 ч/24 ч. Первые точки появляются через 15 секунд после
старта панели; `available=false` означает недоступный cgroup: линии игры
пусты, а системные RAM/CPU/диск продолжают идти.

## 4. Сброс пароля панели

```bash
sudo systemctl stop tes3mp-panel.service
sudo -u tes3mp-panel /opt/tes3mp-easy/bin/tes3mp-easy-panel reset-password
sudo systemctl start tes3mp-panel.service
sudo cat /tes3mp-easy/panel-data/initial-password.txt
```

Панель останавливается, чтобы не было двух писателей `auth.json`. Игра и
таймер бэкапов при этом продолжают работать.

## 5. Обновление

### 5.1. Из релизного архива

Для обычного обновления достаточно команды из §2.1: установщик скачает
последний серверный релиз и запустит `bootstrap.sh`; клиентские релизы
(`client-v<version>`) на этот выбор не влияют. Ручной путь:

1. Скачайте и проверьте новый архив по §2.1 и распакуйте его во временный
   каталог.

2. Запустите bootstrap из нового каталога:

   ```bash
   cd /tmp/tes3mp-easy-<new-version>-linux-amd64
   sudo ./bootstrap.sh
   ```

   Bootstrap заменит `ops`/`panel`, unit-файлы, rootctl и sudoers, сохранит
   `/tes3mp-easy` и совпадающий runtime, перезапустит панель и запустит игру,
   если она была остановлена. Повторный запуск использует `start`, поэтому
   игроки не отключаются.

   Bootstrap перепишет `/etc/tes3mp-easy/panel.env` (режим ACME и публичный
   IP), проверит `80/tcp` и перезапустит панель. Уже выпущенный сертификат
   используется, пока покрывает IP и не в окне продления; иначе панель выпустит
   новый. Первый выпуск может занять несколько минут; при ошибке bootstrap
   завершится с подсказкой `journalctl`. Повторный запуск безопасен.

3. Проверьте:

   ```bash
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   curl -kfsS --noproxy '*' https://127.0.0.1/healthz
   ```

   В namespace панели `.staging` и `mods/` должны быть на одном mount:

   ```bash
   panel_pid=$(systemctl show -p MainPID --value tes3mp-panel.service)
   sudo nsenter -t "$panel_pid" -m -- findmnt -no TARGET -T /tes3mp-easy/mods
   sudo nsenter -t "$panel_pid" -m -- findmnt -no TARGET -T /tes3mp-easy/.staging
   ```

   Обе команды должны вывести `/tes3mp-easy`. Если выводятся подкаталоги,
   в unit остались granular-пути `ReadWritePaths`, и deploy снова упадёт с
   `EXDEV`.

### 5.2. Из исходников (developer/emergency)

1. Скопируйте новую версию исходников в `/opt/tes3mp-easy`, не трогая
   `/tes3mp-easy` и установленный runtime:

   ```bash
   rm -rf /tmp/tes3mp-easy-new
   git clone https://github.com/Saagilat/tes3mp-easy.git /tmp/tes3mp-easy-new
   sudo cp -a /tmp/tes3mp-easy-new/. /opt/tes3mp-easy/
   sudo rm -rf /opt/tes3mp-easy/.git
   sudo chown -R root:root /opt/tes3mp-easy
   ```

2. Соберите новые серверные бинарники из каталога новой версии:

   ```bash
   cd /tmp/tes3mp-easy-new
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
     go build -trimpath -o /tmp/tes3mp-easy-ops ./cmd/tes3mp-easy-ops
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
     go build -trimpath -o /tmp/tes3mp-easy-panel ./cmd/tes3mp-easy-panel
   ```

3. Остановите панель и таймеры, затем замените бинарники:

   ```bash
   sudo systemctl stop tes3mp-panel.service tes3mp-easy-backup.timer
   sudo install -o root -g root -m 0755 /tmp/tes3mp-easy-ops \
     /opt/tes3mp-easy/bin/tes3mp-easy-ops
   sudo install -o root -g root -m 0755 /tmp/tes3mp-easy-panel \
     /opt/tes3mp-easy/bin/tes3mp-easy-panel
   ```

4. Обновите unit-файлы и rootctl, если они изменились:

   ```bash
   sudo install -m 0644 /opt/tes3mp-easy/systemd/*.service /etc/systemd/system/
   sudo install -m 0644 /opt/tes3mp-easy/systemd/*.timer /etc/systemd/system/
   sudo install -o root -g tes3mp -m 0750 \
     /opt/tes3mp-easy/rootctl/tes3mp-easy-rootctl \
     /usr/local/libexec/tes3mp-easy-rootctl
   sudo systemctl daemon-reload
   ```

5. Запустите панель и таймеры, проверьте:

   ```bash
   sudo systemctl start tes3mp-panel.service tes3mp-easy-backup.timer
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

Игру можно не останавливать, если обновление не меняет ABI TES3MP.

### Обновление TES3MP runtime

Runtime не перезаписывается автоматически: при наличии каталога bootstrap
сверяет `.tes3mp-install-info` и останавливается при несовпадении. Плановая
замена:

```bash
sudo systemctl stop tes3mp.service
sudo mv /opt/tes3mp-easy/tes3mp /opt/tes3mp-easy/tes3mp.previous
```

Обновите `server/tes3mp-version.txt` (версия, URL, sha256) из новой версии
(в релизном архиве он уже новый), затем запустите `bootstrap.sh` из нового
релизного архива. Для source-установки — из `/opt/tes3mp-easy`. Если загрузка
или проверка не удалась, верните старый runtime:

```bash
sudo rm -rf /opt/tes3mp-easy/tes3mp
sudo mv /opt/tes3mp-easy/tes3mp.previous /opt/tes3mp-easy/tes3mp
sudo systemctl start tes3mp.service
```

После успешного bootstrap проверьте маркер и symlinks из §3. Bootstrap сам
запускает новый runtime; убедитесь, что `systemctl is-active tes3mp.service`
возвращает `active`, и удалите `tes3mp.previous`. `/tes3mp-easy` при замене
runtime не меняется: state, конфиги и моды остаются на месте.

## 6. Моды

### Через панель

`/admin/mods` → «Загрузить архив» → выбрать `.tar.gz` (до 512 MiB) → дождаться
импорта → «Deploy». Панель сама вызовет ops и перезапустит сервер, если он
был запущен.

### Через CLI

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods import \
  --file /tes3mp-easy/panel-data/uploads/<uuid>.tar.gz

sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods deploy --latest
```

Проверка:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type mods --limit 10
sudo cat /tes3mp-easy/backups/mods/current.txt
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json restart
```

`mods import` не включает моды, только кладёт архив. `mods deploy` делает
`state-predeploy-*`, останавливает сервер, распаковывает в staging, меняет
`mods/`, генерирует `customScripts.lua`, пишет `current.txt`, запускает сервер,
если он работал до операции, и выполняет full retention. При ошибке возвращает
старые файлы.

Если `mods import` вернул `archive_invalid`, архив не попал в `backups/mods`:
проверьте `plugins/requiredDataFiles.json`, CRC, пути и отсутствие базовых ESM
в архиве, затем повторите импорт. Если упал `mods deploy`, ops вернул старые
`mods/` и `current.txt`; сервер остаётся в прежнем состоянии.

## 7. Состояния: создать и восстановить

State содержит игроков, мир и `config.lua`; моды — отдельный неизменяемый пакет,
на который state ссылается через `current.txt`. Пароль сервера
(`tes3mp-server-default.cfg`) и `banlist.json` в state не входят.

Создать вручную:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups create --type state --manual
```

Восстановить скачанный архив через панель:

1. `/admin/state` → «Восстановить из скачанного архива».
2. Выберите ZIP «состояние + моды» (с `/backups`) или отдельный state
   `.tar.gz`.
3. Дождитесь redirect с сообщением «Состояние и моды восстановлены…».

Панель для ZIP сама импортирует моды под их исходным именем, задеплоит их,
импортирует state и выполнит `state deploy`. Отдельный state принимается
только на уже активном пакете модов. Временные загрузки удаляются.

То же через CLI:

```bash
# Моды: сохранить имя из state, затем активировать
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods import \
  --file /tes3mp-easy/panel-data/uploads/mods.tar.gz \
  --name mods-2026-10-22_12-00-00.tar.gz
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods deploy \
  --archive mods-2026-10-22_12-00-00.tar.gz

# State: положить в backups/state, затем восстановить
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json state import \
  --file /tes3mp-easy/panel-data/uploads/state.tar.gz \
  --name state-2026-10-22_12-00-00.tar.gz
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json state deploy \
  --archive state-2026-10-22_12-00-00.tar.gz
```

`state import` проверяет state против активного `backups/mods/current.txt`,
поэтому порядок обязателен. Коллизия имени возвращает `invalid_argument`;
существующие архивы не перезаписываются. Перед импортом распакуйте ZIP из
`/backups`, если работаете вручную: внутри лежат ровно state и mods `.tar.gz`.

Список и текущий пакет:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type state --limit 20
sudo cat /tes3mp-easy/backups/mods/current.txt
```

Восстановить:

```bash
# 1. Активировать пакет, который записан в state
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods deploy --archive mods-2026-10-22_12-00-00.tar.gz
# 2. Восстановить state
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json state deploy \
  --archive state-2026-10-22_12-00-00.tar.gz
```

`state deploy --latest` берёт самый новый state. Перед восстановлением ops
создаёт `state-predeploy-*` и останавливает сервер. После успеха сервер
запускается, если работал до операции. При ошибке возвращаются старые
`players/`, `world/`, `config.lua`; `tes3mp-server-default.cfg`, `banlist.json`
и `customScripts.lua` не затрагиваются.

Если `current.txt` состояния не совпадает с активным пакетом, ops вернёт
`archive_invalid`. Сначала выполните `mods deploy --archive <mods_archive>`,
затем повторите `state deploy`. Автоматической подстановки другого пакета нет.

State без `configs/config.lua` или с `tes3mp-server-default.cfg`/`banlist.json`
не принимается: `archive_invalid`. Создайте новый state текущей версией.

## 8. Retention и диск

Проверить размеры:

```bash
du -sh /tes3mp-easy/backups/state /tes3mp-easy/backups/mods \
       /tes3mp-easy/players /tes3mp-easy/world /tes3mp-easy/mods
df -h /tes3mp-easy
```

Бюджеты: `state` 1.0 GiB, `mods` 512 MiB, свободный резерв 512 MiB. `doctor`
даёт `warn` при свободных 512 MiB–1 GiB и `error` меньше 512 MiB.

State-only retention работает после каждого state backup:

- 0–7 дней — все;
- 7–365 дней — последний за UTC-день;
- 1–3 года — последний за ISO-неделю;
- старше 3 лет — последний за месяц, навсегда;
- ручные — 5 новейших, predeploy — 2 новейших.

Full retention работает на `mods import`, `mods deploy`, `backups prune` и при
нехватке места:

- state-политика выше;
- `mods` — 5 новейших, `mods-init` — 2 новейших и текущий;
- все пакеты, указанные в `current.txt` любого state, защищены.

Пакет годовалой давности остаётся восстановимым, пока на него ссылается state.
После удаления state его пакет может быть удалён при следующем full retention.

Месячные представители state не удаляются автоматически никогда. Если бюджеты
всё равно превышены, `doctor` показывает `warn`; освобождайте место через
`backups prune`, а не удалением файлов вслепую.

Посмотреть, что съедает место:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type state --limit 20
sudo du -h /tes3mp-easy/backups/state/*.tar.gz | sort -h | tail -20
sudo du -h /tes3mp-easy/backups/mods/*.tar.gz | sort -h | tail -20
```

## 9. Мало места

`doctor` проверяет свободное место:

- ≥1 GiB — `ok`;
- 512 MiB–1 GiB — `warn`: значение видно в метрике «свободно на диске», игра работает;
- <512 MiB — `error`: мутации в панели заблокированы, `backups create` и
  `mods deploy` возвращают `no_space` (exit 5), игра продолжает работать.

Автоматической остановки сервера по диску нет.

1. Проверьте:

   ```bash
   df -h /tes3mp-easy
   du -sh /tes3mp-easy/* | sort -h
   sudo journalctl --disk-usage
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

2. Остановите таймер, чтобы новые архивы не усугубляли ситуацию:

   ```bash
   sudo systemctl stop tes3mp-easy-backup.timer
   ```

3. Освободите безопасное:

   ```bash
   sudo find /tes3mp-easy/.staging -mindepth 1 -maxdepth 1 -type d -print
   sudo journalctl --vacuum-size=50M
   sudo apt clean
   ```

   `.staging` можно чистить, только если нет активного `ops` (проверьте
   `pgrep -a tes3mp-easy-ops`). Незавершённые `.part` удаляйте только после
   проверки, что `ops` не работает.

4. Если места всё ещё нет, запустите полный retention с планом:

   ```bash
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json backups prune --dry-run
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json backups prune
   ```

   Full retention защищает пакеты, указанные в `current.txt` любого state.
   State, на который вы рассчитываете, не удалится, если он новее/защищён
   политикой. Месячные представители state остаются.

5. Запустите таймер и проверьте `doctor`:

   ```bash
   sudo systemctl start tes3mp-easy-backup.timer
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

   Если вы останавливали игру вручную, запустите её отдельно:
   `sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops --root /tes3mp-easy
   --json start`.

`no_space` от ops — это не повод удалять архивы вслепую. Сначала копия или
`--dry-run`, потом удаление.


## 10. Игра не запускается или падает

```bash
sudo systemctl status tes3mp.service --no-pager
sudo journalctl -u tes3mp.service -n 200 --no-pager
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json doctor
```

Типичные причины:

- runtime отсутствует или неполный →
  `cat /opt/tes3mp-easy/tes3mp/.tes3mp-install-info` и
  `readlink /opt/tes3mp-easy/tes3mp/server/data/player`; если маркер или symlink
  потерян, переустановите runtime по §5 и не удаляйте `/tes3mp-easy`;
- нет `libluajit-5.1.so.2` → `ldconfig -p | grep luajit`, переустановите пакет;
- битый `customScripts.lua` → проверьте `mods/scripts/` и пересоберите деплоем;
- плагин/ESM, который не подходит к текущей сборке → верните предыдущий
  `mods-*` и `state-predeploy-*`;
- нехватка памяти → `dmesg | tail`, `systemctl show tes3mp.service -p MemoryCurrent`;
- занят порт → `ss -lunp | grep 25565`.

Если сервер циклически падает после деплоя:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods deploy --archive <previous-mods-archive>
```

Если состояние мира повреждено:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json state deploy --latest
```

Перед восстановлением ops сам делает `state-predeploy-*`, так что текущее
состояние не теряется. Если state ссылается на другой пакет модов, сначала
задеплойте его: `mods deploy --archive <mods_archive>`.

### 10.1. Клиент не подключается

Сначала посмотрите код `T3E-…` в диалоге клиента и строку с `code=` в
`logs/client.log`:

```text
Windows: %LOCALAPPDATA%\tes3mp-easy\logs\client.log
Linux:   ~/.local/share/tes3mp-easy/logs/client.log
```

Частые коды:

- `T3E-NET-01/02` — панель или интернет недоступны; проверьте `https://<IP>/healthz`
  и firewall;
- `T3E-NET-03` — сертификат панели не доверен; не отключайте проверку,
  проверьте дату и `--tls-mode`;
- `T3E-API-01/02` — панель отвечает ошибкой или не тем JSON; сравните
  `curl -fsS https://<IP>/api/v1/server`;
- `T3E-API-03` — на панели не опубликован мод-пак;
- `T3E-GAME-03` — не удалось записать `openmw.cfg` или
  `tes3mp/tes3mp-client-default.cfg`; проверьте права на рабочую папку;
- `T3E-GAME-05` — TES3MP завершился; смотрите его лог в `tes3mp/` и клиентский
  лог.

Проверьте, что адрес и порт действительно попали в клиентский конфиг:

```bash
grep -E '^(destinationAddress|port|password) =' \
  <work>/tes3mp/tes3mp-client-default.cfg
```

Пароль в выводе не публикуйте. Клиент делает резервную копию
`tes3mp-client-default.cfg.bak-<timestamp>` и пишет файл с правами `0600`.
Кнопка `Copy report` в диалоге ошибки копирует отчёт без пароля: пути, версии,
список серверов и последние строки лога.

## 11. Панель недоступна

Игра и таймер бэкапов от панели не зависят:

```bash
sudo systemctl status tes3mp-panel.service --no-pager
sudo journalctl -u tes3mp-panel.service -n 200 --no-pager
sudo systemctl restart tes3mp-panel.service
```

Если панель не поднимается:

- проверьте порты `80/443` (постоянно нужен только 443, 80 — на время ACME):
  `ss -ltnp | grep -E ':(80|443)([[:space:]]|$)'`;
- проверьте права `panel-data`: `ls -la /tes3mp-easy/panel-data`;
- проверьте TLS: `sudo openssl x509 -in /tes3mp-easy/panel-data/tls/cert.pem -noout -dates`;
- проверьте, что `tes3mp-panel` входит в группу `tes3mp` и читает бэкапы;
- проверьте sudo: `sudo -u tes3mp-panel sudo -n -u tes3mp \
  /opt/tes3mp-easy/bin/tes3mp-easy-ops --root /tes3mp-easy --json status`.
- если `systemctl restart` отвечает `Start request repeated too quickly` или в
  journald `start-limit-hit`, сработал `StartLimitBurst=2` (защита Let's Encrypt
  от шквала ACME-запросов: лимит считает и ручные рестарты). Сбросьте состояние и
  запустите панель: `sudo systemctl reset-failed tes3mp-panel.service` и
  `sudo systemctl start tes3mp-panel.service`. `bootstrap.sh` делает этот сброс
  сам.

Пока панель лежит, бэкапы и игру можно обслуживать через CLI.

### График ресурсов пуст

Блок «Ресурсы» не пишет на диск и не зависит от ops. Проверяйте по порядку:

- cgroup v2: `stat -fc %T /sys/fs/cgroup` возвращает `cgroup2fs`; на cgroup v1
  метрики игры недоступны (`available=false`), системные RAM/CPU/диск
  продолжают идти;
- файлы игры: `memory.current` и `cpu.stat` в
  `/sys/fs/cgroup/system.slice/tes3mp.service/`; `No such file` при
  остановленном сервере — норма, значения `null`;
- `/proc/meminfo` доступен без привилегий; `df -h /tes3mp-easy` показывает ту
  же ФС, что читает панель; ошибка `panel: metrics` в journald означает проблему
  окружения, а не потерю данных;
- после рестарта панели история пуста по определению: 24 часа хранятся только
  в памяти, первые точки — через 15 секунд;
- если `available=true`, но линия не двигается, обновите страницу: `admin.js`
  добавляет сэмплы раз в 15 секунд.

## 12. systemd/rootctl

```bash
systemctl cat tes3mp.service tes3mp-panel.service \
  tes3mp-easy-backup.service tes3mp-easy-backup.timer
sudo systemd-analyze verify /etc/systemd/system/tes3mp*.service \
  /etc/systemd/system/tes3mp-easy-*.service
sudo /usr/local/libexec/tes3mp-easy-rootctl restart
```

`rootctl` принимает только `start`, `stop`, `restart` и всегда работает с
`tes3mp.service`. Если ops возвращает `service_unavailable`:

- проверьте, что `/usr/local/libexec/tes3mp-easy-rootctl` существует,
  принадлежит `root:tes3mp` и имеет режим `0750`;
- проверьте `/etc/sudoers.d/tes3mp-easy` через `sudo visudo -c`;
- проверьте, что `systemctl` отвечает: `systemctl is-system-running`.


## 13. TLS

```bash
curl -I https://<IP>/         # ожидаем 200
openssl s_client -connect <IP>:443 </dev/null 2>/dev/null | \
  openssl x509 -noout -subject -dates -ext subjectAltName
sudo journalctl -u tes3mp-panel.service -n 100 --no-pager
```

Панель постоянно слушает только `443/tcp`. Сертификат Let's Encrypt для
публичного IP выпускается и продлевается встроенным ACME-клиентом. На время
HTTP-01 challenge панель занимает `80/tcp`; после проверки порт освобождается.

Если сертификата нет или он не продлился:

- проверьте, что `TES3MP_PANEL_PUBLIC_ADDRESS` в
  `/etc/tes3mp-easy/panel.env` совпадает с публичным IPv4;
- проверьте firewall/NAT: `80/tcp` должен быть доступен из интернета на время
  выпуска; `443/tcp` — постоянно;
- проверьте, что `80/tcp` не занят: `ss -ltnp | grep ':80 '`;
- если в journald есть `too many failed authorizations`, Let's Encrypt временно
  заблокировал повторные проверки для этого IP: исправьте причину и подождите
  около часа либо используйте staging;
- если в journald есть `too many certificates (5) ... for this exact set of
  identifiers`, исчерпан недельный лимит Let's Encrypt на сертификаты для
  этого IP. В сообщении указано `retry after <время UTC>`; до этого момента
  повторные запуски установки и рестарты панели не помогут. `bootstrap.sh`
  печатает это время и последние строки journald. Дождитесь сброса
  лимита и повторите установку; смена публичного IPv4 у провайдера обходит
  лимит, потому что он считается для точного набора идентификаторов;
- проверьте системное время: `timedatectl`;
- посмотрите journald: панель пишет причину отказа ACME;
- при ошибке выпуска `bootstrap.sh` завершается с последними строками журнала
  панели и подсказкой `journalctl`; повторный запуск безопасен.

Для отладки используйте staging, чтобы не расходовать production-лимиты:

```bash
sudo systemctl edit tes3mp-panel.service
# [Service]
# Environment=TES3MP_PANEL_ACME_DIRECTORY=https://acme-staging-v02.api.letsencrypt.org/directory
sudo systemctl restart tes3mp-panel.service
```

Режим готовой пары (`--tls-mode files`) оставлен для нестандартных установок:
положите `cert.pem` и `key.pem` в `/tes3mp-easy/panel-data/tls/` с правами
`0600` и укажите `TES3MP_PANEL_TLS_MODE=files`. Штатная установка этого не
требует.

Так поднимают панель на стенде без доступа к `80/tcp` или когда лимит для IP
исчерпан (самоподписанный сертификат, браузер покажет предупреждение):

```bash
sudo openssl req -x509 -newkey rsa:2048 -days 365 -nodes \
  -keyout /tmp/key.pem -out /tmp/cert.pem \
  -subj "/CN=<IP>" -addext "subjectAltName=IP:<IP>"
sudo install -m 0600 -o tes3mp-panel -g tes3mp-panel /tmp/cert.pem \
  /tes3mp-easy/panel-data/tls/cert.pem
sudo install -m 0600 -o tes3mp-panel -g tes3mp-panel /tmp/key.pem \
  /tes3mp-easy/panel-data/tls/key.pem
sudo sed -i 's/^TES3MP_PANEL_TLS_MODE=.*/TES3MP_PANEL_TLS_MODE=files/' \
  /etc/tes3mp-easy/panel.env
sudo systemctl reset-failed tes3mp-panel.service
sudo systemctl start tes3mp-panel.service
```

Повторный запуск `bootstrap.sh` перепишет `panel.env` обратно на `acme`; для
стенда правьте файл после установки.

## 14. Копия бэкапов на другую машину

Копируйте весь `backups/`, кроме недописанных файлов:

```bash
rsync -a --exclude "*.part" \
  /tes3mp-easy/backups/ backup@offsite:/srv/tes3mp-easy-backups/
```

Проверка копии:

```bash
ssh backup@offsite 'du -sh /srv/tes3mp-easy-backups/*'
ssh backup@offsite 'tar -tzf /srv/tes3mp-easy-backups/state/<archive>.tar.gz | head'
ssh backup@offsite 'tar -tzf /srv/tes3mp-easy-backups/mods/<archive>.tar.gz | head'
```

State ссылается на пакет модов через свой `current.txt`, поэтому `state/` и
`mods/` копируются только вместе. `backups/mods/current.txt` и текущий
`mods-*.tar.gz` обязательны: по ним определяется активный мод-пак.

## 15. Восстановление на новой VPS

1. Выполните §2.1 или §2.2, затем сразу остановите игру и таймеры шагом 2.
   Панель можно оставить запущенной: до восстановления она показывает пустой
   список, а после `state deploy` данные появятся без перезапуска.
2. Остановите игру и таймеры:

   ```bash
   sudo systemctl stop tes3mp.service tes3mp-easy-backup.timer
   ```

3. Скопируйте `backups/` с копии, сохранив `current.txt`:

   ```bash
   sudo rsync -a backup@offsite:/srv/tes3mp-easy-backups/ \
     /tes3mp-easy/backups/
   sudo chown -R tes3mp:tes3mp /tes3mp-easy/backups
   ```

4. Активируйте пакет модов, затем восстановите state:

   ```bash
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json mods deploy --current
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json state deploy --latest
   ```

   Если `state deploy` вернул `archive_invalid` из-за несовпадения пакета,
   посмотрите `mods_archive` в `backups list --type state` и задеплойте именно
   этот пакет:

   ```bash
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json mods deploy --archive <mods_archive>
   ```

   Затем повторите `state deploy`. State без `configs/config.lua` или с
   cfg/banlist отклоняется как `archive_invalid`.

5. Запустите игру и таймеры, проверьте:

   ```bash
   sudo systemctl start tes3mp.service
   sudo systemctl start tes3mp-easy-backup.timer
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

Если нужного пакета модов нет в копии, восстановить state нельзя: ops не
подставляет другой пакет. Верните `mods-*.tar.gz` из офсайта и повторите
`mods deploy`.

## 16. Финальная проверка

```bash
sudo systemctl is-active tes3mp.service tes3mp-panel.service \
  tes3mp-easy-backup.timer
sudo systemctl list-timers tes3mp-easy-backup.timer --no-pager
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json doctor
curl -fsS https://<IP>/healthz
curl -fsS https://<IP>/api/v1/server
sudo journalctl -u tes3mp.service -n 20 --no-pager
```

Ожидаемо: все четыре unit активны, оба таймера запланированы, `doctor` без
`error`, `/healthz` = `ok`, публичный API отдаёт `mods_sha256`, в логах нет
panic.

## 17. Выпуск релиза

Сервер и клиент выпускаются независимо: правка клиента не создаёт серверный
релиз и наоборот.

| Канал | Тег | Артефакты |
|---|---|---|
| server | `server-v<version>` | `tes3mp-easy-<version>-linux-amd64.tar.gz` (+ `.sha256`) |
| client | `client-v<version>` | `tes3mp-easy-client-<version>-linux-amd64.tar.gz`, `tes3mp-easy-client-<version>-windows-amd64.zip` (+ `.sha256`) |

```bash
# серверный релиз
git tag -a server-v0.3.0 -m "tes3mp-easy server 0.3.0"
git push origin server-v0.3.0

# клиентский релиз
git tag -a client-v0.3.0 -m "tes3mp-easy client 0.3.0"
git push origin client-v0.3.0
```

- Версию подставляет workflow через `-ldflags -X
  github.com/Saagilat/tes3mp-easy/internal/version.Server` (server) и
  `.Client` (client), поэтому коммит «bump version» не нужен. Значения в
  `internal/version` — фолбэки для локальных сборок.
- `install.sh` читает список релизов и берёт серверный архив с наибольшей
  версией, поэтому новый клиентский релиз не подменяет серверный.
  `TES3MP_EASY_VERSION=0.3.0` прибивает версию серверного релиза;
  `--version 0.3.0` у `tes3mp-easy-pack` задаёт её для локальной сборки.
- `internal/contract` и `internal/serverapi` описывают протокол
  клиент↔панель: несовместимые изменения выпускаются в обоих каналах.
- Панель отдаёт свою версию в `/api/v1/server` как `panel_version` — это
  версия серверного релиза.

