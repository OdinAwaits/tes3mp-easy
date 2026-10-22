# TES3MP Easy

Инструменты для развёртывания **TES3MP**-сервера и подключения к нему
игроков без ручной возни с SSH, nginx и bash-меню.

## Статус

Сейчас в `master` лежит старая схема: bash-скрипты, `nginx`, `export` и
клиент на shell. В ветке `v2` проектируется новая версия:

- сервер и панель на Go;
- HTTPS-панель с публичными бэкапами и текущим архивом модов;
- ops-CLI как единственный административный писатель;
- кроссплатформенный клиент на Go + Fyne для Windows 10/11 и Linux/Proton;
- без `nginx`, `certbot` и `export`.

## Документация

Актуальная целевая документация — [`docs/v2/README.md`](docs/v2/README.md).
Начинать чтение нужно с него, а не с корневого README.

| Документ | О чём |
|---|---|
| [01-goals](docs/v2/01-goals.md) | зачем, для кого, MVP и non-goals |
| [02-architecture](docs/v2/02-architecture.md) | компоненты, контейнеры, данные |
| [03-contracts](docs/v2/03-contracts.md) | JSON, коды, архивы, HTTP API |
| [04-ops](docs/v2/04-ops.md) | команды и алгоритмы ops |
| [05-panel](docs/v2/05-panel.md) | auth, TLS, публичные бэкапы, админка |
| [06-client](docs/v2/06-client.md) | мастер, моды, шрифты, запуск |
| [07-testing](docs/v2/07-testing.md) | тесты, фикстуры, parity, acceptance |
| [08-runbook](docs/v2/08-runbook.md) | установка, восстановление, аварии |

`docs/legacy/` — старые гайды, оставлены только для истории миграции.
`docs/reference/` — справки по TES3MP 0.8.1, они не описывают нашу
архитектуру.

## Ветки

```text
master
└── v2
    ├── v2-docs
    ├── v2-infra
    ├── v2-ops
    ├── v2-panel
    └── v2-client
```

`master` не трогаем до milestone. Работа идёт в `v2` и ветках подсистем;
подробности — в [`docs/v2/README.md`](docs/v2/README.md) §4.

## Разработка

Требования и команды появятся вместе с кодом v2. Пока источник истины —
документация и контракты. Перед реализацией любой подсистемы сначала
обновляется `docs/v2/03-contracts.md`, затем фикстуры и тесты.

## Благодарности

Спасибо командам [TES3MP](https://github.com/TES3MP) и
[OpenMW](https://github.com/OpenMW) за Morrowind в мультиплеере.
