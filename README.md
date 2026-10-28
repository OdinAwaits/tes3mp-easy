# TES3MP Easy

Сервер Morrowind-мультиплеера и desktop-клиент: установка одной командой, моды
и бэкапы.

## Быстрый старт: игрок

Клиент и сервер выпускаются независимыми релизами: клиентские помечены
`client-v<version>`, серверные — `server-v<version>`. Клиент лежит в
[GitHub Releases](https://github.com/Saagilat/tes3mp-easy/releases) под тегом
`client-v<version>`:

- **Windows 10/11** — `tes3mp-easy-client-<version>-windows-amd64.zip`, распакуйте
  и запустите `tes3mp-easy-client.exe`.
- **Linux** — `tes3mp-easy-client-<version>-linux-amd64.tar.gz`, распакуйте и
  запустите `./tes3mp-easy-client`.

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

После установки bootstrap печатает временный пароль панели в конце вывода
(`[panel] temporary password: ...`). Если он потерялся, пароль лежит рядом:

```bash
sudo cat /tes3mp-easy/panel-data/initial-password.txt
```

Обновление — той же командой.

Подробности: [runbook](docs/v2/08-runbook.md).
