# TES3MP Easy

Инструменты для развёртывания **TES3MP**-сервера на маленьком VPS и
подключения к нему игроков без ручной возни с SSH и bash-меню.

## Что в проекте

- `tes3mp-easy-ops` — CLI для всех административных операций: статус, логи,
  запуск/остановка, бэкапы, моды, state, конфиги, staff.
- `tes3mp-easy-panel` — HTTPS-панель с публичными бэкапами, текущим архивом
  модов и админскими формами. Панель вызывает только `ops`.
- `tes3mp-easy-client` — кроссплатформенный клиент на Go + Fyne для Windows
  10/11 и Linux/Proton.
- `bootstrap.sh` — подготовка Debian/Ubuntu: пользователи, каталоги, sudoers,
  rootctl, systemd-юниты, journald, sysctl, runtime TES3MP 0.8.1, базовые
  конфиги и `mods-init-*`.
- systemd-юниты для игры, панели и бэкапа каждые 5 минут.

Сервер работает нативно на systemd, без контейнерного рантайма и внешнего
прокси. Административные изменения в данных вносит только `ops`.

## Документация

Актуальная архитектурная документация — [`docs/v2/README.md`](docs/v2/README.md).
Начинать чтение нужно с него, а не с корневого README.

| Документ | О чём |
|---|---|
| [01-goals](docs/v2/01-goals.md) | зачем, для кого, границы, критерии приёмки |
| [02-architecture](docs/v2/02-architecture.md) | systemd, пользователи, данные, бюджет |
| [03-contracts](docs/v2/03-contracts.md) | JSON, коды, архивы, retention, HTTP API |
| [04-ops](docs/v2/04-ops.md) | команды и алгоритмы ops |
| [05-panel](docs/v2/05-panel.md) | auth, TLS, публичные бэкапы, админка |
| [06-client](docs/v2/06-client.md) | мастер, моды, шрифты, запуск |
| [07-testing](docs/v2/07-testing.md) | тесты, фикстуры, retention, приёмка |
| [08-runbook](docs/v2/08-runbook.md) | установка, восстановление, аварии |

`docs/reference/` — справки по TES3MP 0.8.1, они не описывают нашу
архитектуру.

## Требования

- Go 1.24+;
- для сборки клиента на Linux — `libgl1-mesa-dev` и `xorg-dev`;
- для сервера — Debian 12+/Ubuntu 24.04+ x86_64 с systemd.

## Сборка

```bash
go build -trimpath -o /tmp/tes3mp-easy-ops ./cmd/tes3mp-easy-ops
go build -trimpath -o /tmp/tes3mp-easy-panel ./cmd/tes3mp-easy-panel
go build -trimpath -o /tmp/tes3mp-easy-client ./cmd/tes3mp-easy-client
```

Серверные бинарники можно собрать на другой машине:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -o tes3mp-easy-ops ./cmd/tes3mp-easy-ops
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -o tes3mp-easy-panel ./cmd/tes3mp-easy-panel
```

Релизного пакета и установщика бинарников нет: `bootstrap.sh` готовит
хост и runtime, а `ops` и панель устанавливаются в `/opt/tes3mp-easy/bin`
вручную. Подробная последовательность — в
[`docs/v2/08-runbook.md`](docs/v2/08-runbook.md) §2.

## Проверка

```bash
bash -n bootstrap.sh
sh -n rootctl/tes3mp-easy-rootctl
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l .)"
```

Preflight bootstrap можно проверить без изменений на диске:

```bash
sudo ./bootstrap.sh --check
```

## Благодарности

Спасибо командам [TES3MP](https://github.com/TES3MP) и
[OpenMW](https://github.com/OpenMW) за Morrowind в мультиплеере.
