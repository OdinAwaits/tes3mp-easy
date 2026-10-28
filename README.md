# TES3MP Easy

Сервер Morrowind-мультиплеера и desktop-клиент: установка одной командой, моды
и бэкапы.

## Быстрый старт: игрок

Клиент и сервер выпускаются независимыми релизами: клиентские помечены
`client-v<version>`, серверные — `server-v<version>`. Постоянная ссылка на
последний клиент — тег
[`client-latest`](https://github.com/Saagilat/tes3mp-easy/releases/tag/client-latest):
он переезжает на каждый клиентский релиз, а архивы в нём лежат без версии в
имени:

- **Windows 10/11** —
  [`tes3mp-easy-client-windows-amd64.zip`](https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-windows-amd64.zip),
  распакуйте и запустите `tes3mp-easy-client.exe`.
- **Linux** —
  [`tes3mp-easy-client-linux-amd64.tar.gz`](https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-linux-amd64.tar.gz),
  распакуйте и запустите `./tes3mp-easy-client`.

Рядом с каждым архивом лежит `.sha256`. Конкретную версию берите из релиза
`client-v<version>`: там те же файлы с версией в имени.

Нужен Morrowind; на Linux также Steam + Proton. Адрес, порт и пароль сервера
возьмите у админа.

## Быстрый старт: админ

Нужны Debian 12+/Ubuntu 24.04+ x86_64, публичный IPv4, `sudo` и открытые порты
`25565/udp`, `443/tcp`, `80/tcp` (80 — только на время выпуска/продления
сертификата).

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/install.sh)
```

Установщик сам находит последний серверный релиз (`server-v<version>`);
клиентские релизы его не сбивают.

Если архив нужен без установщика, постоянная ссылка на последний серверный
архив — тег
[`server-latest`](https://github.com/Saagilat/tes3mp-easy/releases/tag/server-latest):
[`tes3mp-easy-linux-amd64.tar.gz`](https://github.com/Saagilat/tes3mp-easy/releases/download/server-latest/tes3mp-easy-linux-amd64.tar.gz)
и рядом `.sha256`. Конкретная версия — в релизе `server-v<version>`, где имя
архива содержит версию.

После установки bootstrap печатает временный пароль панели в конце вывода
(`[panel] temporary password: ...`). Если он потерялся, пароль лежит рядом:

```bash
sudo cat /tes3mp-easy/panel-data/initial-password.txt
```

Обновление — той же командой.

Подробности: [runbook](docs/v2/08-runbook.md).
