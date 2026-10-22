# 08 — Runbook

**Статус:** draft.

**Обновлено:** 2026-10-22.

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
- время синхронизировано (systemd-timesyncd).

Это бюджет нативной схемы. Docker-вариант технически влезает в 512 MiB
(замер — в `02-architecture.md` §9), но под нагрузкой оставляет меньше запаса:
для него практический минимум — 640 MiB, комфортно — 1 GiB. Другие ОС —
best-effort, не MVP.

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

1. Распакуйте релиз в `/opt/tes3mp-easy`:

   ```bash
   sudo mkdir -p /opt/tes3mp-easy
   sudo tar -xzf tes3mp-easy-v2.tar.gz -C /opt/tes3mp-easy --strip-components=1
   sudo chown -R root:root /opt/tes3mp-easy
   ```

2. Запустите bootstrap:

   ```bash
   cd /opt/tes3mp-easy
   sudo ./bootstrap.sh
   ```

Bootstrap:

- проверяет Debian/Ubuntu, x86_64, systemd, RAM/swap/диск;
- ставит пакеты: `libluajit-5.1-2`, `ca-certificates`, `curl`, `tar`,
  `gzip`, `rsync`;
- создаёт пользователей `tes3mp` и `tes3mp-panel`, группы, каталоги и права;
- ставит `tes3mp.service`, `tes3mp-panel.service`,
  `tes3mp-easy-backup.service`, `tes3mp-easy-backup.timer`;
- ставит `/usr/local/libexec/tes3mp-easy-rootctl` и
  `/etc/sudoers.d/tes3mp-easy`;
- настраивает journald (`SystemMaxUse=100M`, `RuntimeMaxUse=32M`) и
  `vm.swappiness=10`;
- создаёт `mods-init-*`, `current.txt`, `customScripts.lua`;
- генерирует первый пароль панели;
- запускает `tes3mp.service`, `tes3mp-panel.service`,
  `tes3mp-easy-backup.timer`.

3. Проверьте:

   ```bash
   sudo systemctl status tes3mp.service --no-pager
   sudo systemctl status tes3mp-panel.service --no-pager
   sudo systemctl status tes3mp-easy-backup.timer --no-pager
   sudo systemctl list-timers tes3mp-easy-backup.timer
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

`doctor` должен вернуть `status=ok` (допустим `warn` по месту) и exit 0.

4. Откройте `https://<IP>/`, войдите с временным паролем:

   ```bash
   sudo cat /tes3mp-easy/panel-data/initial-password.txt
   ```

Смените пароль. После успешной смены панель удаляет `initial-password.txt`;
если файл остался, удалите его вручную.

## 3. Первый запуск и проверка

```bash
sudo systemctl start tes3mp.service
sudo systemctl start tes3mp-panel.service
sudo systemctl enable --now tes3mp-easy-backup.timer

sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json status
sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json status --unit tes3mp-easy-backup.timer
```

Ожидаемо:

- `status` для `tes3mp.service` — `running`, `healthy`;
- таймер — `running`, следующий запуск через 5 минут;
- `journalctl -u tes3mp.service -n 50` показывает запуск сервера;
- публичный `/healthz` отвечает `ok`;
- `/api/v1/server` отдаёт имя, адрес, порт и `mods_sha256`.

Проверка ресурсов:

```bash
systemctl show tes3mp.service -p MemoryCurrent -p MemoryMax
systemctl show tes3mp-panel.service -p MemoryCurrent -p MemoryMax
free -m
systemd-cgtop -1
```

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

1. Скачайте новый релиз и распакуйте в отдельный каталог.
2. Остановите панель:

   ```bash
   sudo systemctl stop tes3mp-panel.service
   ```

3. Замените `/opt/tes3mp-easy/bin` и `/opt/tes3mp-easy/tes3mp`, не трогая
   `/tes3mp-easy`:

   ```bash
   sudo rsync -a --delete new-release/bin/ /opt/tes3mp-easy/bin/
   sudo rsync -a --delete new-release/tes3mp/ /opt/tes3mp-easy/tes3mp/
   ```

4. Обновите unit-файлы и rootctl, если они изменились:

   ```bash
   sudo install -m 0644 new-release/systemd/*.service /etc/systemd/system/
   sudo install -m 0644 new-release/systemd/*.timer /etc/systemd/system/
   sudo install -m 0750 new-release/rootctl/tes3mp-easy-rootctl \
     /usr/local/libexec/tes3mp-easy-rootctl
   sudo systemctl daemon-reload
   ```

5. Запустите панель и проверьте:

   ```bash
   sudo systemctl start tes3mp-panel.service
   sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

Игру можно не останавливать, если релиз не меняет ABI TES3MP. Если меняет —
остановите `tes3mp.service`, обновите runtime и запустите снова; state при
этом не трогается.

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

- проверьте, что `/usr/local/libexec/tes3mp-easy-rootctl` существует и
  исполняется;
- проверьте `/etc/sudoers.d/tes3mp-easy` через `sudo visudo -c`;
- проверьте, что `systemctl` отвечает: `systemctl is-system-running`.


## 13. TLS

```bash
curl -I http://<IP>/          # ожидаем 301/308 на https
curl -I https://<IP>/         # ожидаем 200
openssl s_client -connect <IP>:443 -servername <IP> </dev/null \
  2>/dev/null | openssl x509 -noout -subject -dates
```

Если сертификат self-signed:

- панель работает, но браузер и клиент могут ругаться;
- проверьте, что порт 80 доступен для ACME;
- проверьте системное время: `timedatectl`;
- перезапустите панель после выпуска:
  `sudo systemctl restart tes3mp-panel.service`.

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

1. Установите v2 по §2; сразу после `bootstrap` остановите игру и таймер
   (шаг 2), пока не скопирован state.
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
