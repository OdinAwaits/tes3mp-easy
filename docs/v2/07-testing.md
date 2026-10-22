# 07 — Тестирование

**Статус:** draft.

**Обновлено:** 2026-10-22.

Стратегия проверяет не «счастливый путь», а контракты и отказы. Каждый пункт
`03-contracts.md` должен иметь тест; каждый failure mode из `02-architecture.md`
— воспроизводимый сценарий.

## 1. Принципы

- **Контракт — источник тестов.** JSON-поля, коды выхода и форматы архивов
  проверяются по `03-contracts.md`, а не по текущей реализации.
- **Без Morrowind в репозитории.** Фикстуры — пустые файлы с правильными
  именами и CRC. Никаких копий `Morrowind.esm`, BSA, текстур и звуков.
- **Docker не нужен для большинства тестов.** Ops принимает `--root` и
  `Runner`; panel принимает `Ops`; client принимает HTTP-клиент и файловую
  систему.
- **Один и тот же бинарник в тестах и в проде.** Contract-тесты запускают
  `tes3mp-easy-ops`, а не только внутренние функции.
- **Failure injection — часть Definition of Done.** Если операция может
  упасть, тест должен уметь это сымитировать.

## 2. Уровни

| Уровень | Что проверяет | Docker | Где живёт |
|---|---|---|---|
| Unit | tar, CRC, `requiredDataFiles.json`, `current.txt`, config, pathguard, lock, auth | нет | `internal/**/*_test.go` |
| Contract (ops) | запуск бинарника, JSON-envelope, exit codes, stdout/stderr | fake | `test/contract/ops` |
| Contract (panel) | HTTP-коды, заголовки, cookie, JSON публичного API | нет | `internal/panel/**/*_test.go` |
| Contract (client) | разбор ответов API, профиль, установка модов | нет | `internal/client/**/*_test.go` |
| Integration | lifecycle compose, mods/state deploy на temp-root | да, отдельный проект | `test/integration` |
| Parity | ops vs `package.sh`/`deploy_*.sh` | нет | `test/parity` |
| Fault | битый архив, path traversal, lock, нет места, Docker error, rollback | fake | `test/fault` |
| E2E | чистая VM: установка, вход, бэкап, восстановление, клиент | да | ручной/nightly |
| Docs | ссылки, отсутствие дублированных схем | нет | `test/docs` |

## 3. Фикстуры

```text
testdata/
├── mods/
│   ├── minimal/          # один esp, requiredDataFiles.json
│   ├── scripts/          # esp + два lua
│   ├── no-crc/           # requiredDataFiles.json с пустыми массивами
│   ├── bad-crc/          # CRC не совпадает
│   ├── root-json/        # requiredDataFiles.json в корне (должен падать)
│   ├── traversal/        # ../evil
│   ├── symlink/          # symlink внутри tar
│   └── duplicate/        # два файла с разным регистром
├── state/
│   ├── empty/            # пустые каталоги + current.txt
│   ├── full/             # players + все пять world-каталогов
│   ├── legacy/           # current.txt только sha256
│   └── no-current/       # без current.txt
├── configs/
│   ├── server.cfg
│   ├── config.lua
│   └── banlist.json
└── api/
    ├── server.json
    ├── mods-current.json
    └── errors.json
```

Правила фикстур:

- имена и CRC детерминированы;
- tar-фикстуры собираются в тесте, а не хранятся бинарниками, если это
  возможно;
- для «плохих» архивов можно хранить байты, но с комментарием, что именно
  ломается;
- `testdata` не содержит пользовательских данных.

## 4. Ops

### 4.1. Архивы

- `mods import` принимает `minimal`, `scripts`, `no-crc`;
- `root-json`, `traversal`, `symlink`, `duplicate`, `bad-crc` → конкретный
  `error.code`;
- лимиты: 2 GiB сжатого, 100 000 записей, 8 GiB распакованного;
- tar с абсолютным путём, `..`, hardlink, device, FIFO;
- gzip с мусором после tar;
- пустой tar;
- архив без `plugins/`;
- `scripts/` с вложенным каталогом;
- скрипт с пробелом/точкой в имени.

### 4.2. `current.txt`

- новый формат `<sha256> <filename>`;
- legacy `<sha256>`;
- лишние пробелы, CRLF, пустая строка;
- неверная длина хэша, не hex;
- имя с `/`, `..`, не `.tar.gz`;
- файл ссылается на отсутствующий архив;
- `state deploy` с legacy-хэшем, который есть/нет в `backups/mods/`;
- `state deploy` с чужим текущим мод-паком → `archive_invalid`.

### 4.3. Lock и атомарность

- две параллельные мутации: одна проходит, вторая получает `locked` (exit 4);
- `--lock-timeout 0` не ждёт;
- `mods deploy` внутри себя не вызывает `backups create` повторно;
- `backup loop` не блокирует сам себя;
- `--dry-run` не меняет ни одного байта;
- kill -9 в момент записи `current.txt`/архива: старый файл цел, `.part`
  остаётся, `doctor` предупреждает;
- запись конфига: старый файл цел при ошибке fsync/rename (fake writer).

### 4.4. Команды

| Команда | Обязательные случаи |
|---|---|
| `doctor` | ok/warn/error, exit 0/0/10, нет Docker, нет места, битый `current.txt` |
| `status` | running/stopped/unhealthy, Docker недоступен, оба формата `compose ps` |
| `logs` | tail, пустой лог, JSON `lines`, не-JSON текст |
| `backups list` | пусто, сортировка, `current`, `--sha256`, битый `current.txt` |
| `backups create` | успех, `--if-running` + stopped, нет места, обрыв, имя `manual-*` |
| `mods import` | успех, все ошибки архива, upload не удалён при ошибке |
| `mods deploy` | первый деплой, rollback, `--no-restart`, сбой на каждом шаге, state-бэкап |
| `state deploy` | успех, legacy, нет `current.txt`, несовпадение модов, сбой распаковки, rollback |
| `staff set` | 0..3, неверный ранг, имя с `/`, отсутствующий игрок, `.bak` |
| `config get/set` | типы, диапазоны, комментарии, read-only, `.bak`, `config_invalid` |
| `backup loop` | тик, lock busy, SIGTERM, интервал < 1m |

## 5. Панель

- логин: верный/неверный пароль, 5 неудач/15 минут, 429, cookie-флаги;
- pre-session CSRF на форме логина и CSRF на каждой админской форме;
- сессия: idle 12 ч, absolute 7 дней, рестарт панели = разлогин;
- публичные страницы: `/`, `/backups`, `/api/v1/server`,
  `/api/v1/mods/current`, `/api/v1/mods/current/archive`, `/healthz`;
- скачивание: `..`, абсолютный путь, не `.tar.gz`, отсутствующий файл,
  `Content-Disposition`, `ETag`/304;
- загрузка: `MaxBytesReader`, 2 GiB, UUID-имя, вызов fake ops, очистка upload;
- ошибки ops: каждый код из таблицы `03-contracts.md` §3 → HTTP и текст;
- конфиги: валидация, повторное чтение, `config_invalid` рядом с полем;
- staff: чтение `players/*.json`, вызов `ops staff set`, баннер рестарта;
- шаблоны: нет неэкранированного пользовательского ввода, нет stack trace;
- заголовки безопасности на всех ответах.

Панель не тестирует tar и CRC: это тесты ops. Она тестирует, что правильно
вызывает ops и правильно показывает результат.

## 6. Клиент

- поиск Morrowind: реестр, Steam-библиотеки, GOG, Flatpak, ручной выбор;
- проверка ESM, регистр имён, отсутствующий файл;
- `openmw.cfg`: профиль → активный конфиг, сохранение пользовательских строк,
  `.bak`, переключение серверов;
- моды: `no_mods`, совпадение sha256, скачивание, обрыв, битый архив,
  path traversal, отсутствующий плагин;
- шрифты: успех, 404, битый файл, fallback без шрифтов;
- локализация: overlay, `encoding=win1251`, отказ от установки в Morrowind;
- запуск: Windows env, Linux Proton env, отсутствие Proton;
- TLS: self-signed → понятная ошибка, без отключения проверки;
- GUI не тестируется в CI; `app` тестируется без Fyne.

## 7. Parity со старыми скриптами

| Что сравниваем | Критерий |
|---|---|
| `package.sh` vs `internal/archive` | одинаковый набор файлов, `requiredDataFiles.json`, CRC, имена |
| `package_state.sh` vs `backups create` | одинаковые каталоги и метаданные после распаковки |
| `deploy_mods.sh` vs `mods deploy` | одинаковый итоговый `mods/`, `customScripts.lua`, `current.txt` |
| `deploy_state.sh` vs `state deploy` | одинаковый итоговый state; отличие — staging и отсутствие удаления до успеха |
| `set-staff-rank.sh` vs `staff set` | одинаковый `settings.staffRank`, `.bak` |

Parity-тест сравнивает **содержимое**, а не байты tar: порядок и заголовки
могут отличаться. Различия в поведении должны быть перечислены в
`03-contracts.md` §9 и в тесте.

## 8. Failure injection

- **Нет места.** Fake writer возвращает `ENOSPC`; ops — `no_space`, exit 5,
  старые файлы целы.
- **Docker недоступен.** Runner возвращает ошибку; `status` — exit 6, панель —
  503, без «running».
- **Битый gzip/tar.** `archive_invalid`, staging очищен, текущее состояние не
  тронуто.
- **Обрыв на середине.** Тест отменяет context; проверяем `.part`, `.old-*`,
  `current.txt` и `doctor`.
- **Lock.** Два процесса; второй — `locked`.
- **Падение панели.** Сессии в памяти, после рестарта вход заново; публичные
  бэкапы и ops работают.
- **Падение backup-контейнера.** Панель показывает предупреждение; ручной
  `backups create` работает.

## 9. Acceptance

Сценарии с чистой VM:

1. **Установка.** Docker + compose + `.env` → панель по HTTPS → первый пароль
   → смена пароля → `doctor` без ошибок.
2. **Моды.** Загрузить архив → deploy → сервер перезапустился → клиент видит
   `mods_sha256` и скачивает архив.
3. **State.** Создать бэкап → изменить мир → восстановить → состояние
   вернулось, сервер запущен.
4. **Публичные бэкапы.** Скачать mods/state без логина; список не раскрывает
   пути и логи.
5. **TLS.** Сертификат выпущен по IP; HTTP редиректит на HTTPS; после
   перевыпуска клиент не ругается.
6. **Отказ.** Остановить Docker → панель показывает 503; поднять → статус
   возвращается.
7. **Клиент.** Windows 10/11: бинарник + Morrowind → мастер → игра. Linux:
   Proton найден автоматически.
8. **Без модов.** `no_mods` → клиент запускает vanilla-сервер, без ложной
   ошибки.

Критерий приёмки — не только «получилось», но и понятное сообщение при
неудаче, путь к логу и отсутствие потери данных.

## 10. CI

Обязательные шаги на каждый PR:

```bash
go test ./...
go test -race ./...
go vet ./...
GOOS=windows GOARCH=amd64 go build ./cmd/tes3mp-easy-client
GOOS=linux   GOARCH=amd64 go build ./cmd/tes3mp-easy-client
GOOS=linux   GOARCH=amd64 go build ./cmd/tes3mp-easy-ops
GOOS=linux   GOARCH=amd64 go build ./cmd/tes3mp-easy-panel
```

- Integration с Docker — nightly или на merge в `v2`, не на каждый PR.
- Fyne GUI не запускается в CI; собирается бинарник.
- Contract-тесты ops запускают собранный бинарник.
- `test/docs` проверяет, что ссылки в `docs/v2/` не битые и что схемы не
  дублируются (например, нет второго блока `/api/v1/server`).

## 11. Definition of Done

Изменение считается готовым, если:

- есть тест на новый контракт и на его failure mode;
- contract-тест ops проверяет JSON и exit code;
- panel-тест проверяет HTTP-код и пользовательское сообщение;
- client-тест проверяет поведение без сети/с битым ответом;
- parity-тест обновлён, если меняется формат архива;
- `03-contracts.md` обновлён первым;
- `07-testing.md` обновлён, если появился новый класс проверок.
