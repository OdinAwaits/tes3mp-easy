# TES3MP Easy

TES3MP Easy — готовый сервер Morrowind-мультиплеера (TES3MP 0.8.1) на VPS и
desktop-клиент для игроков. Админ ставит сервер из релизного архива, игрок
скачивает клиент, указывает папку Morrowind и адрес сервера. Игроку не нужны
SSH, bash-меню и ручная установка модов.

- **Сервер:** systemd, автобэкапы каждые 5 минут, HTTPS-панель, моды, state.
- **Клиент:** Windows 10/11 и Linux/Proton; сам ставит TES3MP, моды и шрифты.
- **Админ:** браузерная панель; внешняя БД, Docker и прокси не нужны.

## Быстрый старт: админ

### 1. Подготовьте VPS

Нужны Debian 12+/Ubuntu 24.04+ x86_64 с systemd, 1 vCPU, 512 MiB RAM
(640 MiB рекомендуется), 7 GiB диска и открытые порты `25565/udp`, `80/tcp`,
`443/tcp`.

```bash
sudo apt update && sudo apt install -y curl ca-certificates

# Если используете ufw:
sudo ufw allow 25565/udp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

### 2. Скачайте и установите релиз

Замените `VERSION` на актуальную версию из
[GitHub Releases](https://github.com/Saagilat/tes3mp-easy/releases).

```bash
VERSION=0.1.0
BASE="https://github.com/Saagilat/tes3mp-easy/releases/download/v${VERSION}"

curl -fsSLO "${BASE}/tes3mp-easy-${VERSION}-linux-amd64.tar.gz"
curl -fsSLO "${BASE}/tes3mp-easy-${VERSION}-linux-amd64.tar.gz.sha256"
sha256sum -c "tes3mp-easy-${VERSION}-linux-amd64.tar.gz.sha256"

tar -xzf "tes3mp-easy-${VERSION}-linux-amd64.tar.gz"
cd "tes3mp-easy-${VERSION}-linux-amd64"

sudo ./bootstrap.sh --check   # проверка без изменений на диске
sudo ./bootstrap.sh           # установка
```

`bootstrap.sh` ставит TES3MP, оба бинарника, systemd-юниты, пользователей,
каталоги и запускает игру, таймер бэкапов и панель. Go, исходники и Docker на
VPS не нужны.

### 3. Войдите в панель

```bash
sudo cat /tes3mp-easy/panel-data/initial-password.txt
```

Откройте `https://<IP-сервера>/`, войдите временным паролем и сразу смените
его. Браузер предупредит о самоподписанном сертификате — это нормально для
первого запуска.

В панели откройте **Настройки** и задайте:

- **Имя сервера** — то, что увидят игроки;
- **Пароль сервера** — пароль для входа в игру (не пароль панели).

После сохранения перезапустите игру: кнопка **Перезапустить** на **Обзоре**
или `sudo systemctl restart tes3mp`. Без перезапуска TES3MP продолжит работать
со старым конфигом.

Панель по умолчанию использует self-signed TLS. Клиент проверяет сертификат,
поэтому для игры с друзьями либо положите доверенный сертификат в
`panel-data/tls/`, либо передайте игрокам `panel-data/tls/cert.pem` для
импорта в хранилище ОС. Подробнее — в
[runbook](docs/v2/08-runbook.md#2-установка).

Игроку на Linux достаточно выполнить:

```bash
sudo cp cert.pem /usr/local/share/ca-certificates/tes3mp-easy.crt
sudo update-ca-certificates
```

На Windows: откройте `certmgr.msc` → **Доверенные корневые центры
сертификации** → **Импорт** → выберите `cert.pem`.

### 4. Проверьте сервер

```bash
sudo systemctl status tes3mp.service --no-pager
sudo systemctl status tes3mp-easy-backup.timer --no-pager
sudo systemctl status tes3mp-panel.service --no-pager

sudo -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json doctor
```

Ожидаемо: игра и панель `active (running)`, таймер `active (waiting)`,
`doctor` без ошибок.

### 5. Передайте игрокам

```text
Адрес:  <IP-сервера>
Порт:   25565
Пароль: <пароль сервера из панели>
Панель: https://<IP-сервера>/
```

## Быстрый старт: игрок

### 1. Скачайте клиент

Возьмите клиент из [GitHub Releases](https://github.com/Saagilat/tes3mp-easy/releases):

- **Windows 10/11:** `tes3mp-easy-client-<version>-windows-amd64.zip` —
  распакуйте и запустите `tes3mp-easy-client.exe`.
- **Linux:** `tes3mp-easy-client-<version>-linux-amd64.tar.gz` — распакуйте и
  запустите `./tes3mp-easy-client`. Нужны рабочий Steam + Proton, Morrowind и
  графические библиотеки X11/OpenGL (Fyne).

### 2. Пройдите первичную настройку

При первом запуске клиент откроет мастер:

1. Выберите язык.
2. Укажите папку Morrowind — ту, внутри которой лежит `Data Files/` с
   `Morrowind.esm`, `Tribunal.esm` и `Bloodmoon.esm`.
3. Оставьте рабочую папку по умолчанию или выберите свою.
4. Дождитесь установки TES3MP и шрифтов.
5. Нажмите **Готово**.

Оригинальная папка Morrowind не изменяется: клиент читает её, а моды и
локализация складываются в отдельный профиль.

### 3. Добавьте сервер и играйте

На главном экране нажмите **Добавить** и заполните:

| Поле | Значение |
|---|---|
| Название | любое, например `Друзья` |
| Адрес | IP сервера |
| Порт | `25565` |
| Пароль | пароль сервера от админа |
| URL панели | `https://<IP-сервера>` |

Затем:

1. **Проверить сервер** — клиент покажет статус и текущий мод-пак.
2. **Обновить моды** — скачает и проверит архив модов с панели.
3. **Играть** — клиент подготовит `openmw.cfg` и запустит TES3MP.

При следующих запусках достаточно нажать **Играть**. Если админ обновил моды,
клиент предложит/скачает новую версию.

## Обновление

Сервер обновляется тем же архивом: скачайте новую версию, проверьте sha256,
распакуйте и снова выполните `sudo ./bootstrap.sh`. Данные в `/tes3mp-easy`
сохраняются; bootstrap не перезаписывает существующие конфиги и state. Перед
обновлением можно сделать бэкап в панели.

Клиент обновляется заменой бинарника/архива. Настройки и профили серверов
остаются в `%APPDATA%\tes3mp-easy` (Windows) или
`~/.config/tes3mp-easy` (Linux).

## Если что-то не работает

| Симптом | Что проверить |
|---|---|
| `bootstrap.sh` падает на preflight | Запустите `sudo ./bootstrap.sh --check`; проверьте ОС, RAM, диск и systemd. |
| Панель не открывается | `systemctl status tes3mp-panel`, порты `80/443`, `journalctl -u tes3mp-panel`. |
| Клиент пишет про сертификат | Импортируйте `panel-data/tls/cert.pem` в доверенные корни ОС или используйте доверенный сертификат на панели. |
| Клиент не находит Morrowind | Укажите папку, внутри которой есть `Data Files/Morrowind.esm`. |
| Игра не подключается | Проверьте `25565/udp`, IP, пароль сервера и `systemctl status tes3mp`. |

Полный runbook, восстановление state и аварийные сценарии —
[`docs/v2/08-runbook.md`](docs/v2/08-runbook.md).

## Для разработчика

Нужны Go 1.24+ и `git`; для Linux-клиента — `libgl1-mesa-dev` и `xorg-dev`.

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...

# воспроизводимый серверный релиз + .sha256
go run ./cmd/tes3mp-easy-pack --version 0.1.0
```

Архитектура, контракты, панель, клиент и тесты —
[`docs/v2/README.md`](docs/v2/README.md). Исходники без `bin/` можно
использовать для developer/emergency-установки: bootstrap подготовит хост и
runtime, а `ops`/`panel` нужно собрать и установить вручную (см. runbook §2.2).

## Благодарности

Спасибо командам [TES3MP](https://github.com/TES3MP) и
[OpenMW](https://github.com/OpenMW) за Morrowind в мультиплеере.
