# 08 — Runbook

Runbook для нативной установки на Debian/Ubuntu x86_64 с systemd. Все
команды — от пользователя с sudo. Если не сказано иное, `ROOT=/tes3mp-easy`,
`OPS=/opt/tes3mp-easy/bin/tes3mp-easy-ops`.

## 1. Требования

- Debian 12+ или Ubuntu 24.04+ x86_64 с systemd;
- 1 vCPU;
- 512 MiB RAM минимум, 640 MiB рекомендуется;
- 512 MiB swap;
- 7 GiB NVMe/SSD;
- открытые `25565/udp`, `80/tcp`, `443/tcp`;
- root/sudo и доступ по SSH;
- `curl`, `tar`, `gzip`, `sha256sum`;
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

Конкретную версию можно закрепить:

```bash
TES3MP_EASY_VERSION=0.1.0 bash <(curl -Ls https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/install.sh)
```

Ручной путь, если архив нужно скачать заранее:

1. Скачайте архив и checksum с GitHub Releases (или возьмите локально
   собранный `dist/tes3mp-easy-<version>-linux-amd64.tar.gz`):

   ```bash
   VERSION=0.1.0
   curl -fsSLO "https://github.com/Saagilat/tes3mp-easy/releases/download/v${VERSION}/tes3mp-easy-${VERSION}-linux-amd64.tar.gz"
   curl -fsSLO "https://github.com/Saagilat/tes3mp-easy/releases/download/v${VERSION}/tes3mp-easy-${VERSION}-linux-amd64.tar.gz.sha256"
   sha256sum -c "tes3mp-easy-${VERSION}-linux-amd64.tar.gz.sha256"
   tar -xzf "tes3mp-easy-${VERSION}-linux-amd64.tar.gz"
   cd "tes3mp-easy-${VERSION}-linux-amd64"
   ```

   Локальная сборка релиза:

   ```bash
   go run ./cmd/tes3mp-easy-pack --version 0.1.0
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

4. Прочитайте временный пароль панели и войдите:

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
   curl -kfsS https://127.0.0.1/healthz
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

- проверяет Debian/Ubuntu, x86_64, systemd, RAM/swap/диск;
- ставит пакеты: `libluajit-5.1-2`, `ca-certificates`, `curl`, `tar`,
  `gzip`, `rsync`, `coreutils`;
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
  `tes3mp-easy-backup.timer`, затем проверяет `is-active`; при наличии
  установленной панели включает и перезапускает `tes3mp-panel.service`.

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

4. Включите панель после установки её бинарника:

   ```bash
   sudo systemctl enable --now tes3mp-panel.service
   sudo cat /tes3mp-easy/panel-data/initial-password.txt
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

Bootstrap включает и запускает `tes3mp.service` и backup timer. При установке
из релизного архива он также запускает панель; при source-установке панель
включается отдельно после установки её бинарника. Runtime можно проверить
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
сервер или таймер были остановлены:

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
  5 минут;
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

Для обычного обновления достаточно команды из §2.1: установщик скачает свежий
архив и запустит `bootstrap.sh`. Ручной путь:

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

3. Проверьте:

   ```bash
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   curl -kfsS https://127.0.0.1/healthz
   ```

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

3. Остановите панель и таймер, затем замените бинарники:

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

5. Запустите панель и таймер, проверьте:

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
`mods/`, генерирует `customScripts.lua`, пишет `current.txt` и запускает
сервер, если он работал до операции. При ошибке возвращает старые файлы.

Если `mods import` вернул `archive_invalid`, архив не попал в `backups/mods`:
проверьте `plugins/requiredDataFiles.json`, CRC, пути и отсутствие базовых ESM
в архиве, затем повторите импорт. Если упал `mods deploy`, ops вернул старые
`mods/` и `current.txt`; сервер остаётся в прежнем состоянии.

## 7. State: создать и восстановить

Создать вручную:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups create --type state --manual
```

Список:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type state --limit 20
```

Восстановить:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json state deploy --archive state-manual-2026-10-22_12-00-00.tar.gz
```

Или `--latest`. Перед восстановлением ops создаёт `state-predeploy-*` и
останавливает сервер. После успеха сервер запускается, если работал до
операции; при ошибке старые `players/` и `world/` и прежнее состояние сервера
возвращаются.

Если state-архив ссылается на другой мод-пак:

```text
error.code = archive_invalid
message: current.txt points to mods-..., current is mods-...
```

Сначала задеплойте нужный мод-пак, затем повторите `state deploy`.

## 8. Retention и диск

Проверить размеры:

```bash
du -sh /tes3mp-easy/backups/state /tes3mp-easy/backups/mods \
       /tes3mp-easy/players /tes3mp-easy/world /tes3mp-easy/mods
df -h /tes3mp-easy
```

Бюджеты: `backups/state` 1 GiB, `backups/mods` 512 MiB, свободный резерв
512 MiB. `doctor` предупреждает, если свободно меньше 1 GiB, и возвращает
`error`, если меньше 512 MiB.

Retention работает автоматически после `backups create`, `mods import` и
создания `state-predeploy-*`:

- 0–7 дней — все автоматические 5-минутные;
- 7–365 дней — последний за день;
- 1–3 года — последний за ISO-неделю;
- старше 3 лет — последний за месяц, навсегда;
- ручные — 5, predeploy — 2, моды — 5, `mods-init` — 2 новейших и текущий,
  если он старше.

Месячные архивы не удаляются автоматически никогда. Если они сами превышают
1 GiB, `doctor` показывает `warn`; освободите место или увеличьте диск.
Удалять месячные вручную можно только после копирования на другую машину и
проверки, что копия читается.

Посмотреть, что съедает место:

```bash
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type state --limit 20
sudo du -h /tes3mp-easy/backups/state/*.tar.gz | sort -h | tail -20
```

## 9. Мало места

1. Проверьте:

   ```bash
   df -h /tes3mp-easy
   du -sh /tes3mp-easy/* | sort -h
   sudo journalctl --disk-usage
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

4. Если места всё ещё нет:

   - скопируйте `backups/state` и `backups/mods` на другую машину
     (`rsync -a --exclude "*.part"`);
   - проверьте копию (`tar -tzf` для нескольких архивов);
   - только после этого удаляйте старые архивы вручную;
   - месячные удаляйте последними и только при крайней необходимости;
   - `current.txt` и текущий `mods-*` не удаляйте никогда.

5. Запустите таймер снова и проверьте `doctor`:

   ```bash
   sudo systemctl start tes3mp-easy-backup.timer
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

`no_space` от ops — это не повод удалять state вслепую. Сначала копия, потом
удаление.


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
  `state-predeploy-*` или предыдущий `mods-*`;
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
состояние не теряется.

## 11. Панель недоступна

Игра и таймер бэкапов от панели не зависят:

```bash
sudo systemctl status tes3mp-panel.service --no-pager
sudo journalctl -u tes3mp-panel.service -n 200 --no-pager
sudo systemctl restart tes3mp-panel.service
```

Если панель не поднимается:

- проверьте порты `80/443`: `ss -ltnp | grep -E ":80|:443"`;
- проверьте права `panel-data`: `ls -la /tes3mp-easy/panel-data`;
- проверьте TLS: `openssl x509 -in /tes3mp-easy/panel-data/tls/cert.pem -noout -dates`;
- проверьте, что `tes3mp-panel` входит в группу `tes3mp` и читает бэкапы;
- проверьте sudo: `sudo -u tes3mp-panel sudo -n -u tes3mp \
  /opt/tes3mp-easy/bin/tes3mp-easy-ops --root /tes3mp-easy --json status`.

Пока панель лежит, бэкапы и игру можно обслуживать через CLI.

## 12. systemd/rootctl

```bash
systemctl cat tes3mp.service tes3mp-panel.service \
  tes3mp-easy-backup.service tes3mp-easy-backup.timer
systemd-analyze verify /etc/systemd/system/tes3mp.service
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
curl -I http://<IP>/          # ожидаем 308 на https
curl -I https://<IP>/         # ожидаем 200
openssl s_client -connect <IP>:443 -servername <IP> </dev/null \
  2>/dev/null | openssl x509 -noout -subject -dates
```

Если сертификат self-signed:

- панель работает, но браузер и клиент могут ругаться;
- ACME-выпуск по IP не реализован: панель использует готовую пару или
  создаёт self-signed при первом старте;
- чтобы использовать доверенный сертификат, положите `cert.pem` и `key.pem` в
  `/tes3mp-easy/panel-data/tls/` с правами `0600` и перезапустите панель;
- проверьте системное время: `timedatectl`.

Не отключайте проверку TLS в клиенте ради self-signed сертификата. Это
превращает MITM в норму.

## 14. Копия бэкапов на другую машину

Минимум — state и mods, включая `current.txt`:

```bash
rsync -a --exclude "*.part" --exclude "*.old-*" \
  /tes3mp-easy/backups/ backup@offsite:/srv/tes3mp-easy-backups/
```

Проверка копии:

```bash
ssh backup@offsite 'ls -lh /srv/tes3mp-easy-backups/state | tail'
ssh backup@offsite 'tar -tzf /srv/tes3mp-easy-backups/state/<archive>.tar.gz | head'
```

Копируйте не только state, но и `backups/mods/current.txt` и текущий
`mods-*.tar.gz`: state-архив ссылается на мод-пак. Без него восстановление
не пройдёт проверку.

## 15. Восстановление на новой VPS

1. Выполните §2.1 или §2.2, затем сразу остановите игру и таймер шагом 2.
   Панель можно оставить запущенной: до восстановления она показывает пустой
   список, а после `state deploy` данные появятся без перезапуска.
2. Остановите игру и таймер:

   ```bash
   sudo systemctl stop tes3mp.service tes3mp-easy-backup.timer
   ```

3. Скопируйте `backups/` с копии, сохранив `current.txt`:

   ```bash
   sudo rsync -a backup@offsite:/srv/tes3mp-easy-backups/ \
     /tes3mp-easy/backups/
   sudo chown -R tes3mp:tes3mp /tes3mp-easy/backups
   ```

4. Задеплойте текущий мод-пак, затем state:

   ```bash
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json mods deploy --current
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json state deploy --latest
   ```

5. Запустите игру и таймер, проверьте:

   ```bash
   sudo systemctl start tes3mp.service
   sudo systemctl start tes3mp-easy-backup.timer
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

Если `state deploy` говорит `archive_invalid` из-за несовпадения модов,
сначала задеплойте тот `mods-*`, на который указывает `current.txt` внутри
state-архива, и повторите.

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

Ожидаемо: все три unit активны, таймер запланирован, `doctor` без `error`,
`/healthz` = `ok`, публичный API отдаёт `mods_sha256`, в логах нет panic.
