# 08 — Runbook

**Статус:** draft.

**Обновлено:** 2026-10-22.

Runbook для админа: установка, повседневные операции и аварийные действия.
Панель — основной интерфейс; SSH нужен для bootstrap, диагностики и
восстановления, когда панель недоступна.

## 1. Требования

- VPS с Linux x86_64, 2+ ГБ RAM, 20+ ГБ диска (лучше больше под state).
- Docker Engine 24+ и Compose v2.
- Открытые порты:
  - `80/tcp` — ACME HTTP-01 и redirect;
  - `443/tcp` — панель;
  - `25565/udp` — TES3MP;
  - `22/tcp` — SSH, желательно ограниченный по IP.
- Домен не обязателен: целевой режим — сертификат Let's Encrypt по IP.
  Если Let's Encrypt по IP недоступен в вашей среде, используйте домен;
  self-signed — только временный аварийный режим.

## 2. Первая установка

1. Установить Docker (пример для Debian/Ubuntu; для другого дистрибутива —
   официальная инструкция Docker):

   ```bash
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker "$USER"
   ```

   Перелогиниться, чтобы группа применилась.

2. Скачать релиз в `/tes3mp-easy` (архив содержит `docker-compose.yml`,
   образы/теги и `.env.example`):

   ```bash
   sudo mkdir -p /tes3mp-easy
   sudo chown "$USER":"$USER" /tes3mp-easy
   cd /tes3mp-easy
   # распаковать релизный архив сюда
   cp .env.example .env
   ```

3. Заполнить `.env`:

   ```dotenv
   TES3MP_EASY_ROOT=/tes3mp-easy
   TES3MP_PUBLIC_HOST=203.0.113.10
   TES3MP_PUBLIC_PORT=25565
   TES3MP_PANEL_URL=https://203.0.113.10
   TES3MP_SERVER_NAME=Morrowind with friends
   ```

   `TES3MP_PUBLIC_HOST` — адрес, который увидят игроки. Это не обязательно IP
   клиента. Пароль сервера задаётся в панели, не в `.env`.

4. Запустить:

   ```bash
   docker compose up -d
   docker compose ps
   docker compose logs -f panel
   ```

5. Взять первый пароль:

   ```bash
   cat /tes3mp-easy/panel-data/initial-password.txt
   ```

   Файл имеет права `0600`, создаётся только при первом запуске и исчезает
   после смены пароля.

6. Открыть `https://<IP>/`, войти, сменить пароль.

7. Проверить:

   ```bash
   docker compose exec panel tes3mp-easy-ops \
     --root /tes3mp-easy --json doctor
   ```

   `status` должен быть `ok` или `warn`; `error` — повод не запускать игроков.

## 3. Первый вход и базовая настройка

- Сразу сменить пароль: `/admin/password`.
- Проверить имя сервера, порт, `maximumPlayers`, пароль сервера в
  `/admin/config/server`.
- Загрузить мод-пак в `/admin/mods` и нажать Deploy.
- Создать первый state-бэкап в `/admin/state`.
- Убедиться, что `backup` работает: `docker compose ps` и
  `docker compose logs backup`.
- Проверить публичную страницу `/backups` без логина.
- Проверить, что порт `25565/udp` доступен снаружи.

## 4. Повседневные операции

| Задача | Где | Команда/действие |
|---|---|---|
| Статус | панель | дашборд, обновление каждые 5 с |
| Логи | панель | `/admin/logs`, tail 200 |
| Старт/стоп/рестарт | панель | кнопки на дашборде |
| Ручной state-бэкап | панель | `/admin/state` → «Создать бэкап» |
| Загрузить/задеплоить моды | панель | `/admin/mods` |
| Откатить моды | панель | Rollback (деплой предыдущего) |
| Восстановить state | панель | `/admin/state` → «Восстановить» |
| Сменить ранг | панель | `/admin/staff` |
| Сменить конфиг | панель | `/admin/config` |

SSH-эквиваленты:

```bash
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json status
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json logs --tail 200
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json backups list --type state
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json backups create --type state
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json mods deploy --latest
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json state deploy --archive export-2026-10-22_12-00-00.tar.gz
```

Мутации не запускать параллельно: второй процесс получит `locked` и код 4.

## 5. TLS

- Панель слушает `:80` и `:443`.
- `:80` нужен для ACME HTTP-01 и редиректа на HTTPS. Не закрывайте его
  файрволом, иначе выпуск сертификата не пройдёт.
- Сертификаты: `/tes3mp-easy/panel-data/tls/cert.pem` и `key.pem`, права
  `0600`.
- Проверка:

  ```bash
  openssl x509 -in /tes3mp-easy/panel-data/tls/cert.pem -noout -subject -dates
  curl -Iv https://203.0.113.10/healthz
  ```

- Логи ACME:

  ```bash
  docker compose logs panel | grep -i -E 'acme|tls|certificate'
  ```

- Если выпуск не удался, панель стартует с self-signed. Это аварийный режим:
  браузер покажет предупреждение, клиент — ошибку «сертификат не доверен».
  Не отключайте проверку TLS в клиенте; импортируйте `cert.pem` в хранилище
  ОС или почините выпуск.
- Для IP-сертификата Let's Encrypt может требовать короткоживущий сертификат
  и перевыпуск. Панель обновляет его автоматически; после смены сертификата
  клиенту не нужно ничего делать, кроме доверия цепочке.

## 6. Бэкапы

### Что где лежит

```text
/tes3mp-easy/backups/
├── mods/
│   ├── current.txt
│   ├── import-*.tar.gz
│   └── init-*.tar.gz
└── state/
    ├── export-*.tar.gz       # автоматические
    └── manual-*.tar.gz       # кнопка в панели
```

- `export-*` создаёт `backup` каждые 5 минут, если сервер запущен.
- `manual-*` создаёт админ.
- `current.txt` в `backups/mods/` указывает, какой мод-пак активен.
- Архивы state содержат копию `current.txt` и `requiredDataFiles.json` на
  момент бэкапа.

### Проверить бэкапы

```bash
docker compose exec panel tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type state
docker compose exec panel tes3mp-easy-ops \
  --root /tes3mp-easy --json backups list --type mods
```

### Скачать

Публично: `https://<IP>/backups`, без логина. Это осознанное решение; не
храните там ничего, кроме игровых данных.

### Восстановить state

1. Убедиться, что активный мод-пак совпадает с тем, с которым сделан state.
   Если нет — сначала задеплоить нужные моды.
2. В панели: `/admin/state` → «Восстановить» → подтвердить.
3. Или через SSH:

   ```bash
   docker compose exec panel tes3mp-easy-ops \
     --root /tes3mp-easy --json state deploy \
     --archive export-2026-10-22_12-00-00.tar.gz
   ```

4. Проверить `status`, `doctor`, зайти игроком.

`state deploy` не удаляет текущее состояние до успешной распаковки. Если
операция упала, ops возвращает старые `players/` и `world/` и запускает
сервер. Если сервер не поднялся — смотрите `panel.log` и
`docker compose logs tes3mp`.

### Правило 3-2-1

Один VPS с бэкапами — это не бэкап. Раз в неделю копируйте `backups/` на
другую машину:

```bash
rsync -av --delete /tes3mp-easy/backups/ user@backup-host:/srv/tes3mp-easy-backups/
```

Публичные ссылки на бэкапы не заменяют офсайт-копию: если VPS умер, панель
тоже недоступна.

## 7. Аварийные сценарии

### 7.1. Панель не открывается

```bash
docker compose ps
docker compose logs --tail 200 panel
curl -vk https://127.0.0.1/healthz
```

- контейнер `panel` не запущен → `docker compose up -d panel`;
- порт 443 занят → `ss -lntp | grep -E ':80|:443'`;
- сертификат сломан → смотрите §5, временно можно зайти по HTTP на `:80`
  (редирект), но не оставляйте так;
- панель падает при старте → `docker compose logs panel`, проверьте права на
  `panel-data/` и валидность `auth.json`.

### 7.2. Забыт пароль

Канонический способ:

```bash
docker compose exec panel tes3mp-easy-panel reset-password
cat /tes3mp-easy/panel-data/initial-password.txt
```

Если панель не запущена:

```bash
docker compose run --rm panel tes3mp-easy-panel reset-password
```

Fallback, если команда недоступна в вашей сборке:

```bash
docker compose stop panel
rm -f /tes3mp-easy/panel-data/auth.json
rm -f /tes3mp-easy/panel-data/initial-password.txt
docker compose up -d panel
cat /tes3mp-easy/panel-data/initial-password.txt
```

Панель сгенерирует новый временный пароль и потребует смену при входе.
TLS-сертификаты, загрузки и бэкапы не затрагиваются.

### 7.3. Docker не отвечает

```bash
systemctl status docker
journalctl -u docker --no-pager -n 200
docker info
```

- Docker не запущен → `systemctl start docker`, затем `docker compose up -d`;
- нет места → §7.5;
- socket недоступен пользователю → проверьте группу `docker` и права;
- после перезагрузки хоста compose-сервисы должны подняться сами
  (`restart: unless-stopped`).

### 7.4. Автобэкапы не идут

```bash
docker compose ps backup
docker compose logs --tail 200 backup
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json doctor
```

- `backup` не запущен → `docker compose up -d backup`;
- `tes3mp` остановлен, а `--if-running` включён → это нормально, бэкапов нет;
- lock занят → в логе `locked`, следующий тик пройдёт;
- нет места → §7.5;
- контейнер в цикле перезапуска → смотрите `docker compose logs backup`.

### 7.5. Мало места

```bash
df -h /tes3mp-easy
du -sh /tes3mp-easy/backups/mods /tes3mp-easy/backups/state
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json doctor
```

- `doctor` покажет `warn`/`error` и свободное место;
- удалять старые архивы можно только вручную по SSH; в MVP панель не умеет
  удалять бэкапы;
- сначала скопируйте архивы на другую машину, потом удаляйте самые старые;
- не удаляйте `backups/mods/current.txt` и архив, на который он указывает;
- не удаляйте `players/`, `world/`, `configs/`, `mods/` — это живые данные.

Пример безопасной очистки (оставить 10 самых новых state-архивов):

```bash
cd /tes3mp-easy/backups/state
ls -1t *.tar.gz | tail -n +11 | xargs -r rm --
```

Перед `rm` проверьте список без удаления (`ls -1t *.tar.gz | tail -n +11`).

### 7.6. Битый архив или неудачный deploy

- `mods import` вернул `archive_invalid` → архив не попал в `backups/mods/`,
  текущие моды не тронуты; проверьте CRC и структуру.
- `mods deploy` упал → ops вернул старые `mods/`, `current.txt` и
  `customScripts.lua`; проверьте `doctor` и `docker compose logs tes3mp`.
- `state deploy` упал → ops вернул старые `players/` и `world/`; проверьте
  `.staging/` и `.old-*` (ops чистит их сам, но после kill -9 могут остаться).
- Если остались `.old-*`, не удаляйте их до проверки, что сервер работает.

### 7.7. Сервер не стартует

```bash
docker compose ps tes3mp
docker compose logs --tail 200 tes3mp
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json doctor
```

- проверьте `configs/customScripts.lua` и наличие скриптов в `mods/scripts/`;
- проверьте, что `requiredDataFiles.json` соответствует `mods/plugins/`;
- проверьте `configs/tes3mp-server-default.cfg` на валидный порт и пароль;
- если недавно меняли state — восстановите предыдущий архив;
- если недавно меняли моды — сделайте rollback.

### 7.8. Потеря VPS / переезд

1. На новом VPS установить Docker и распаковать релиз в `/tes3mp-easy`.
2. Скопировать с офсайт-копии:
   - `backups/` (обязательно);
   - `configs/` (если не хотите настраивать заново);
   - `panel-data/` (если хотите сохранить пароль и TLS; иначе будет новый
     пароль и новый сертификат).
3. `docker compose up -d`.
4. Проверить `doctor`.
5. Восстановить моды:
   - `mods import --file <последний import-*.tar.gz>`;
   - `mods deploy --archive <имя>`;
   - если архивы уже лежат в `backups/mods/`, импорт не нужен, достаточно
     `mods deploy`.
6. Восстановить state:
   - `state deploy --archive <последний export-*.tar.gz>`;
   - если `current.txt` внутри архива указывает на моды, которых нет, ops
     откажется; сначала деплойте нужные моды.
7. Проверить `status`, `doctor`, публичные бэкапы и вход игрока.

### 7.9. Аварийный доступ без панели

```bash
cd /tes3mp-easy
docker compose run --rm panel \
  tes3mp-easy-ops --root /tes3mp-easy --json doctor
```

`run --rm` поднимает одноразовый контейнер из того же образа. Если панель
жива, используйте `exec` — это быстрее.

## 8. Обновление и откат

- Обновление: скачать новый релиз, остановить `panel` и `backup`, поднять
  `docker compose up -d`, проверить `doctor`.
- Перед обновлением сделать `manual-*` state-бэкап и скопировать
  `backups/mods/current.txt`.
- Откат: вернуть предыдущий тег образа в `.env`/compose и `docker compose up -d`.
- Схема данных обратно совместима в пределах v2; если релиз меняет формат
  архива, это указано в release notes и в `03-contracts.md`.
- Не откатывайтесь на старый bash-стек после того, как новый ops записал
  `current.txt` в формате `<sha256> <filename>`: старые скрипты могут не
  понять второй токен. Для отката используйте предыдущий релиз v2.

## 9. Переход со старой схемы

1. Сделать полный бэкап старого `/tes3mp-easy` (rsync на другую машину).
2. Остановить старые `nginx`, `export`, cron-задачи и контейнеры.
3. Оставить данные на местах: `configs/`, `players/`, `world/`, `mods/`,
   `backups/`. Не пересоздавать их.
4. Развернуть новый compose; панель читает старые архивы без конвертации.
5. Проверить `doctor`: он найдёт legacy `current.txt` и подскажет, если
   архив модов не найден.
6. Проверить публичные бэкапы и клиент.
7. Старые URL (`/list-backups/`, `/download/`) больше не работают; обновите
   ссылки у игроков.

## 10. Шпаргалка команд

```bash
cd /tes3mp-easy

docker compose ps
docker compose logs -f panel
docker compose logs --tail 200 tes3mp
docker compose restart tes3mp

docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json doctor
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json status
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json logs --tail 200
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json backups list --type state
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json backups create --type state
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json mods deploy --latest
docker compose exec panel tes3mp-easy-ops --root /tes3mp-easy --json state deploy --archive <name>
```

## 11. Чек-лист безопасности

- [ ] SSH ограничен по IP и ключам, пароль root отключён.
- [ ] Открыты только 22/tcp, 80/tcp, 443/tcp, 25565/udp.
- [ ] Docker socket не проброшен наружу и не доступен игрокам.
- [ ] Пароль панели сменён с initial, хранится в менеджере паролей.
- [ ] `panel-data/` и `backups/` не отдаются nginx/apache (их нет).
- [ ] Публичные бэкапы — осознанное решение; ссылки не индексируются.
- [ ] TLS-сертификат валиден, self-signed не используется постоянно.
- [ ] `doctor` без `error`, автобэкапы идут, офсайт-копия настроена.
