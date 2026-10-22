# 04 — Серверный операционный слой (ops)

**Статус:** draft.

**Обновлено:** 2026-10-22.

Документ описывает команды и алгоритмы ops. Нормативные JSON-формы, коды
выхода, форматы архивов и retention — в `03-contracts.md`; при расхождении
прав 03. Ops заменяет ad-hoc bash-скрипты.

## 1. Зачем отдельный CLI

Ops — это один бинарник `tes3mp-easy-ops`, который умеет всё, что админ делал
руками: смотреть статус, запускать и останавливать сервер, делать и
восстанавливать бэкапы, импортировать и деплоить моды, менять конфиги и
staff rank.

Почему CLI, а не библиотека внутри панели:

- **Тестируемость.** Ops можно запустить с `--root` на временной папке и
  fake systemd/journalctl, без HTTP и без настоящего сервера.
- **Аварийный доступ.** Если панель не поднимается, админ заходит по SSH и
  делает `tes3mp-easy-ops --root /tes3mp-easy --json status`.
- **Один писатель.** Панель, таймер и человек используют один и тот же код и
  один lock.
- **Контракт.** JSON и коды выхода стабильны; панель не зависит от внутренних
  Go-типов.
- **Границы.** Ops не знает про HTTP, пароли и UI. Панель не знает, как
  устроен tar.

## 2. Где лежит

```text
cmd/
  tes3mp-easy-ops/       # main
  tes3mp-easy-panel/     # main, отдельный документ
  tes3mp-easy-rootctl/   # root-helper, фиксированные start/stop/restart
internal/
  ops/                   # команды и сценарии
  archive/               # tar.gz, requiredDataFiles.json, current.txt
  systemd/               # обёртка над systemctl/journalctl/rootctl
  retention/             # buckets, бюджеты, pruning
  config/                # чтение/запись cfg, lua, banlist
  lock/                  # flock
  atomicfile/            # temp + rename
  pathguard/             # проверка имён и путей
  doctor/                # проверки окружения
```

Пакеты не должны тянуть HTTP-сервер, шаблоны или клиентские зависимости.

## 3. CLI-контракт

### Глобальные флаги

| Флаг | По умолчанию | Смысл |
|---|---|---|
| `--root` | `/tes3mp-easy` | Корень данных |
| `--json` | выкл | Только JSON в stdout, логи в stderr/journald |
| `--dry-run` | выкл | Проверить и показать план, ничего не менять |
| `--lock-timeout` | `30s` | Сколько ждать lock |
| `--command-timeout` | `2m` | Таймаут одной systemctl/journalctl-команды |

`--root` нужен для тестов и нестандартных установок. В production он совпадает
с `/tes3mp-easy`.

### Команды

| Команда | Что делает |
|---|---|
| `doctor [--json]` | Проверяет root, права, systemd, journald, runtime, память, диск, конфиги, `current.txt`, retention-бюджеты |
| `status [--unit tes3mp.service\|tes3mp-easy-backup.timer] [--json]` | Состояние unit |
| `start` / `stop` / `restart` | Управляет `tes3mp.service` через rootctl |
| `logs --tail N` | Отдаёт последние N строк journald для `tes3mp.service` |
| `backups list --type mods\|state [--limit 200] [--before <name>] [--sha256]` | Страница архива с курсором |
| `backups create --type state [--manual] [--if-running]` | Создаёт state-бэкап и применяет retention |
| `mods import --file <path>` | Проверяет и импортирует архив модов |
| `mods deploy (--current\|--latest\|--archive <name>) [--no-restart]` | Деплоит моды |
| `state deploy (--latest\|--archive <name>) [--no-restart]` | Восстанавливает state |
| `staff set --player <name> --rank 0..3` | Меняет `settings.staffRank` |
| `config get --file server\|config\|banlist` | Отдаёт структурированные настройки |
| `config set --file ... --key ... --value ...` | Меняет одну настройку |

Команды `mods deploy` и `state deploy` останавливают сервер перед подменой
файлов. Если он был запущен, после успеха они запускают его снова; если был
остановлен — оставляют остановленным. Флаг `--no-restart` оставляет сервер
остановленным и в первом случае; он не отменяет остановку перед подменой.

## 4. JSON и коды выхода

Нормативно — `03-contracts.md` §2–§3. Кратко:

- `--json` → ровно один объект `{"ok":...}` в stdout, логи в stderr;
- без `--json` — человекочитаемый вывод;
- коды `0`–`10`, включая `10 checks_failed` для `doctor`;
- панель строит логику по `error.code`, а не по `message`.

## 5. Lock и атомарность

Все мутации берут эксклюзивный lock:

```text
/tes3mp-easy/.locks/ops.lock
```

- Реализация: `flock`/`syscall.Flock` на файле; lock живёт, пока жив процесс.
- `--lock-timeout 30s`: по истечении — exit 4, `locked`.
- Читающие команды (`status`, `logs`, `backups list`, `doctor`) lock не берут:
  архивы пишутся через temp + rename и не видны частично.
- `--dry-run` берёт shared lock и ничего не пишет.
- Внутренние операции не вызывают публичную команду повторно: `mods deploy`
  делает state-бэкап через `createStateBackupLocked`, а timer-запуск — через
  ту же внутреннюю функцию. Иначе `flock` в том же процессе даст deadlock.

Любая запись файла — это temp-файл в том же каталоге, `fsync`, `rename`.
Никаких «записали прямо в `config.lua`» и «удалили state, потом распаковали».

## 6. Команды

### `doctor`

Проверяет:

- `--root` существует и читается; каталоги, в которые пишет `tes3mp`, доступны
  ему на запись; владельцы и режимы совпадают с §4 `02-architecture.md`;
- есть `configs/`, `players/`, `world/`, `mods/`, `backups/`, `panel-data/`;
- `/etc/os-release` — Debian 12+ или Ubuntu 24.04+; незнакомая ОС — `warn`,
  отсутствие файла — `error`;
- `uname -m` — `x86_64`; другая архитектура — `error`;
- `systemctl` работает, версия systemd читается; не-systemd — `error`;
- `tes3mp.service` и `tes3mp-panel.service` установлены и не в `failed`;
  `tes3mp-easy-backup.timer` активен и `enabled`;
- journald читается, `journalctl -u tes3mp.service -n 1` не требует пароля;
- `tes3mp-server` и библиотеки релиза на месте; `libluajit-5.1.so.2` находится
  через `ldconfig -p`; `ldd tes3mp-server` не показывает `not found`;
- RAM не меньше 512 MiB, swap не меньше 512 MiB, `vm.swappiness=10`;
- cgroup-лимиты юнитов соответствуют §3 `02-architecture.md`;
- свободное место: `warn` меньше 1 GiB, `error` меньше 512 MiB;
- `panel-data/uploads` и `backups/` на одной файловой системе (`stat.Dev`);
- `backups/state` и `backups/mods` в пределах бюджета 1 GiB / 512 MiB;
- `backups/mods/current.txt` канонический, указывает на существующий архив,
  sha256 совпадает;
- `requiredDataFiles.json` читается и соответствует `mods/plugins/` (кроме трёх
  базовых ESM, которых в `mods/plugins/` нет);
- state-архивы (последний и несколько случайных) имеют вложенную структуру;
- конфиги парсятся, `customScripts.lua` соответствует `mods/scripts/`;
- нет осиротевших `.part`, `.old-*`, `.staging/` и старых lock-файлов;
- journald-капы `SystemMaxUse=100M`, `RuntimeMaxUse=32M`.

Результат — список проверок в форме `doctor` из `03-contracts.md` §4; схема
здесь не дублируется.

`doctor` возвращает 0, если нет `error` (предупреждения допустимы), и 10
(`checks_failed`), если есть хотя бы одна ошибка. Код 2 зарезервирован за
неверными аргументами.

### `status`

Форма `data` — из `03-contracts.md` §4, `status`; схема здесь не дублируется.

Состояния: `running`, `stopped`, `starting`, `unhealthy`, `unknown`.

Источник — `systemctl show <unit> --property=ActiveState,SubState,ActiveEnterTimestamp`
и `systemctl is-active`. `health` = `unhealthy`, если unit в `failed` или
`is-active` вернул ошибку; иначе `healthy`/`unknown`. Если systemd недоступен —
exit 6, `service_unavailable`.

Для таймера `state` — `running`, если таймер активен, иначе `stopped`;
`started_at` — `LastTriggerUSec`, если доступен. Поле `backend` отсутствует.

### `start` / `stop` / `restart`

```text
sudo -n /usr/local/libexec/tes3mp-easy-rootctl start
sudo -n /usr/local/libexec/tes3mp-easy-rootctl stop
sudo -n /usr/local/libexec/tes3mp-easy-rootctl restart
```

`start` идемпотентен. `stop` ждёт завершения. `restart` не пересоздаёт unit.
Ошибки systemd/rootctl/прав → exit 6, `service_unavailable`. Ops не вызывает
`systemctl` напрямую для этих трёх глаголов; только через rootctl.

### `logs`

`journalctl -u tes3mp.service -n N --no-pager -o cat`.

Без `--json` — просто текст. С `--json` — объект из `03-contracts.md` с
`data.lines`; панель показывает массив в `<pre>`. Если journald недоступен —
exit 6.

### `backups list`

Читает `backups/mods/*.tar.gz` или `backups/state/*.tar.gz`, сортирует по
mtime (новые сверху). По умолчанию не считает sha256 — это дорого на больших
архивах. Флаг `--sha256` добавляет хеш только для текущей страницы.

- `--limit` 1..500, по умолчанию 200;
- `--before <name>` — следующие элементы строго старше курсора;
- `next_before` — последний элемент страницы, если страница заполнена.

```json
{
  "ok": true,
  "data": {
    "type": "mods",
    "current": "mods-2026-10-22_12-00-00.tar.gz",
    "items": [
      {"name": "mods-2026-10-22_12-00-00.tar.gz", "size": 123456, "mtime": "...", "current": true}
    ],
    "next_before": null
  }
}
```

### `backups create`

1. Берёт lock.
2. Если `--if-running`: `systemctl is-active tes3mp.service`; не `active` →
   `skipped=true`, `reason=not_running`, exit 0. Если `systemctl` недоступен —
   exit 6, `service_unavailable`, а не skipped.
3. Проверяет диск: свободно меньше 512 MiB → `no_space`; меньше 1 GiB →
   warning, но продолжает.
4. Открывает `backups/state/<name>.tar.gz.part` и пишет потоково: gzip.Writer →
   tar.Writer → файлы. Без промежуточной копии state и без буфера в памяти.
   Порядок: `players/`, `world/cell/`, `world/world/`, `world/map/`,
   `world/recordstore/`, `world/custom/`, `requiredDataFiles.json`,
   `current.txt`.
5. `fsync` файла и каталога, `rename` в `.tar.gz`.
6. Применяет retention и бюджеты; возвращает `archive`, `size`, `sha256`,
   `pruned`, `warnings`. При `skipped=true` полей `archive`, `size`, `sha256`
   нет, возвращаются `reason`, `pruned`, `warnings`.

Имя: `state-*` для таймера, `state-manual-*` при `--manual`. Внутренний вызов
для деплоя создаёт `state-predeploy-*`.

Бэкап снимается с живого сервера и не является транзакционным snapshot: файл,
который TES3MP пишет в этот момент, может попасть в архив в промежуточном
состоянии. При типичных ~100 KB и 5-минутном интервале это приемлемый
recovery point; для гарантированно согласованного архива остановите сервер
или сделайте `state-predeploy` через deploy-операцию.

### `mods import`

Вход — путь к `.tar.gz`, который панель сохранила в `panel-data/uploads/`.
Ops не доверяет ни имени, ни содержимому.

Проверки:

- upload и `backups/mods` на одной файловой системе (`stat.Dev`), иначе
  `invalid_argument`;
- gzip и tar читаются потоково, размер в пределах 512 MiB, записей ≤ 50 000,
  распаковано ≤ 1 GiB;
- нет абсолютных путей, `..`, symlink, hardlink, устройств, NUL;
- верхний уровень — только `plugins/` и `scripts/`;
- `plugins/requiredDataFiles.json` существует и парсится как массив объектов;
- `requiredDataFiles.json` в корне архива — ошибка `archive_invalid`;
- каждый плагин из JSON есть в `plugins/`, кроме трёх базовых ESM: они в архив
  не входят и проверяются клиентом по Morrowind; расширение — из белого списка;
- оригинальные `Morrowind.esm`/`Tribunal.esm`/`Bloodmoon.esm` отсутствуют в
  `plugins/`;
- CRC32 (IEEE) каждого плагина из архива совпадает с `0x...` в JSON; CRC
  базовых ESM в архиве не проверяется;
- `scripts/` содержит только `.lua` (без вложенных каталогов);
- нет дубликатов имён.

При успехе архив переименовывается в
`backups/mods/mods-YYYY-MM-DD_HH-MM-SS.tar.gz`; копирования между файловыми
системами нет. `current.txt` **не** меняется. При ошибке исходный upload не
удаляется: панель сама решает, когда чистить `panel-data/uploads`.

```json
{
  "ok": true,
  "data": {
    "archive": "mods-2026-10-22_12-00-00.tar.gz",
    "sha256": "...",
    "plugins": 12,
    "scripts": 2,
    "pruned": [],
    "warnings": []
  }
}
```

### `mods deploy`

Аргумент: `--current` (взять из `current.txt`), `--latest` (самый новый
`mods-*`/`mods-init-*`) или `--archive <name>`.

Порядок:

1. Взять lock.
2. Найти и проверить архив; для `--current` `current.txt` обязан быть
   каноническим, иначе `archive_invalid`.
3. Проверить диск: state-бэкап + распаковка + запас.
4. Сделать `state-predeploy-*` через `createStateBackupLocked`, чтобы был
   откат.
5. Запомнить, был ли сервер запущен; остановить `tes3mp.service` через
   rootctl.
6. Распаковать `plugins/` и `scripts/` в
   `/tes3mp-easy/.staging/mods-<id>/` потоково, с теми же лимитами.
7. Проверить распакованное ещё раз (пути, файлы, CRC плагинов из архива;
   базовые ESM в архиве отсутствуют).
8. Подменить `mods/plugins` и `mods/scripts` через rename; старые каталоги
   оставить как `mods/plugins.old-<id>` до успеха.
9. Сгенерировать `configs/customScripts.lua` из `mods/scripts/*.lua`
   (отсортировано, `require("custom.<name>")`).
10. Записать `backups/mods/current.txt` в формате `<sha256> <filename>`.
11. Запустить `tes3mp.service`, если он был запущен и не указан
    `--no-restart`.
12. Удалить `.old-*` и staging.

Если шаг 8–11 упал: вернуть старые `mods/`, старый `current.txt`, старый
`customScripts.lua`, запустить сервер, если он работал до операции и не указан
`--no-restart`, и вернуть ошибку. Это не «автоматический откат вообще всего», а
завершение одной транзакции деплоя.

`--no-restart` оставляет сервер остановленным. Если сервер был остановлен до
операции, ops оставляет его остановленным; если работал — запускает после
успешного деплоя. Штатный сценарий панели — деплой на работающем сервере;
`--no-restart` нужен для аварийного режима.

### `state deploy`

`--latest` выбирает самый новый `state-*` или `state-manual-*` и **не**
рассматривает `state-predeploy-*`: это внутренние точки отката, их
восстанавливают только явным `--archive`.

Порядок:

1. Взять lock.
2. Проверить архив: tar читается, нет опасных путей, есть ровно
   `players/`, `world/{cell,world,map,recordstore,custom}/`,
   `requiredDataFiles.json`, `current.txt`; иначе `archive_invalid`.
3. Проверить `current.txt` внутри архива: канонический формат, sha256,
   существующий архив модов; сравнить с текущим `backups/mods/current.txt`.
4. Проверить диск.
5. Сделать `state-predeploy-*`.
6. Остановить `tes3mp.service`.
7. Распаковать в `/tes3mp-easy/.staging/state-<id>/` с той же вложенной
   структурой; плоские каталоги не раскладываются и не принимаются.
8. Переименовать текущие `players` → `players.old-<id>`, `world` →
   `world.old-<id>`.
9. Переименовать staging-каталоги на их место.
10. Запустить `tes3mp.service`, если он был запущен и не указан
    `--no-restart`.
11. При успехе удалить `.old-*`; при ошибке вернуть старые каталоги и
    запустить сервер, если он работал до операции и не указан `--no-restart`.

`requiredDataFiles.json` и `current.txt` из архива должны соответствовать
модам, с которыми делался бэкап. Ops не подменяет моды:

- если `current.txt` указывает на другой архив, чем текущий
  `backups/mods/current.txt`, — `archive_invalid` и подмены нет;
- если `current.txt` отсутствует или в старом формате — `archive_invalid`;
- `--force` в MVP нет: лучше остановиться, чем смешать файлы с разными CRC.

### `staff set`

```text
tes3mp-easy-ops staff set --player "John Doe" --rank 2
```

- Имя проверяется: непустое, без `/`, `\`, `..`, NUL, не `.`/`..`.
- Файл: `players/<name>.json`. Если файла нет — exit 3, `not_found`; ops не
  создаёт нового игрока.
- `settings.staffRank` = 0..3 (0 none, 1 moderator, 2 admin, 3 owner).
- Перед записью делается `players/<name>.json.bak-<timestamp>`.
- Запись атомарная; при ошибке JSON не портится.
- Возвращает `{"player": "...", "rank": 2, "restart_required": true}`.
- Сервер не перезапускает: панель показывает баннер «нужен рестарт».

Полный state-бэкап перед сменой ранга не нужен: это один маленький JSON, и
рядом остаётся `.bak`.

### `config get` / `config set`

Файлы:

| `--file` | Путь | Формат |
|---|---|---|
| `server` | `configs/tes3mp-server-default.cfg` | INI |
| `config` | `configs/config.lua` | `config.key = value` |
| `banlist` | `configs/banlist.json` | JSON |

`config get` отдаёт структурированный объект известных ключей, а не сырой
текст. Для `config` поддерживаются простые значения: string, number, boolean.
Таблицы и списки в MVP read-only.

`config set`:

- ключ должен быть в белом списке;
- тип и диапазон проверяются (порт 1024–65535, `maxClientsPerIP` 1–10,
  `difficulty` -100..100 и т.п.);
- строка `config.key = value` заменяется построчно, комментарии сохраняются;
- перед записью — `.bak-<timestamp>`;
- запись атомарная;
- возвращает `restart_required`; сервер не перезапускает — это делает панель или
  админ;
- `customScripts.lua` через `config set` менять нельзя — он генерируется.

Ошибка валидации → exit 9, `config_invalid`. Панель показывает её рядом с
полем, а не общую «500».

## 7. Retention

Алгоритм вызывается после `backups create`, `mods import` и создания
`state-predeploy-*`, а также доступен `doctor` как проверка размера.
Нормативная таблица — `03-contracts.md` §6.7.

1. Прочитать архивы и их mtime.
2. Защитить самый новый state-архив, текущий mods-архив, `current.txt` и все
   месячные представители.
3. Для автоматических `state-<timestamp>`: архивы 0–7 дней не трогать;
   для 7–365 дней оставить последний за день; для 1–3 лет — последний за
   ISO-неделю; для старше 3 лет — последний за месяц. Остальные — кандидаты.
4. Для `state-manual-*` оставить 5 новейших, для `state-predeploy-*` — 2.
5. Для `mods-<timestamp>` (кроме `mods-init-*`) оставить 5 новейших, для
   `mods-init-*` — 2.
6. Удалить кандидатов, затем проверить бюджеты `backups/state` 1 GiB и
   `backups/mods` 512 MiB и свободный резерв 512 MiB.
7. Если бюджеты превышены, включить pressure pruning:
   - автоматические `state-<timestamp>` старше 24 часов, не последние за день;
   - дневные представители, самые старые первыми;
   - недельные представители, самые старые первыми.
8. Месячные не удалять никогда. Если после pressure pruning бюджет всё ещё
   превышен, вернуть `warnings`, но не удалять месячные.

Удаление — `os.Remove` + `fsync` каталога. Ошибка удаления не отменяет уже
созданный архив: имя попадает в `warnings`, а не в `pruned`, и `doctor`
покажет проблему.

## 8. Архивы и генерация

Форматы `mods` и `state`, `requiredDataFiles.json`, `current.txt` и лимиты —
только в `03-contracts.md` §6. Здесь остаётся то, что делает ops поверх этих
форматов.

### Потоковая обработка

- `archive/tar`, `compress/gzip` и `io.Copy` с буфером 32–64 KiB.
- Никаких `bytes.Buffer` для архива, `io.ReadAll` для upload или
  промежуточной копии state.
- `io.LimitReader` на gzip/tar и на каждый файл; счётчики записей и
  распакованного размера.
- Временный файл в целевом каталоге; `fsync` файла и каталога; `rename`.
- `mods import` делает rename upload → `backups/mods/`; cross-filesystem
  копирования нет.
- `state` пишется прямо из `players/` и `world/`, без staging-копии.
- `tar` детерминированный: сортировка путей, mtime из файлов, uid/gid 0, без
  лишних заголовков. Байт-в-байт совпадение с GNU tar не цель.

### `customScripts.lua`

Генерируется из `mods/scripts/*.lua`:

```lua
require("custom.test")
require("custom.another")
```

Порядок — лексикографический. Файл read-only для панели.

## 9. Systemd-адаптер

```go
type Runner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

- Production: `exec.CommandContext(ctx, "systemctl", args...)` для чтения,
  `journalctl` для логов и `sudo -n /usr/local/libexec/tes3mp-easy-rootctl`
  для start/stop/restart.
- Никакого `sh -c` и склейки аргументов в строку.
- `status`: `systemctl show` + `is-active`; `logs`: `journalctl -u`.
- Таймаут из `--command-timeout`.
- В тестах — fake runner, который пишет вызовы и возвращает заготовленный
  вывод/ошибки. Реальный systemd для unit-тестов не нужен.

## 10. Тестирование

| Уровень | Что проверяет | systemd |
|---|---|---|
| Unit | tar, CRC, `requiredDataFiles.json`, `current.txt`, retention, config, pathguard, lock | нет |
| Contract | запуск бинарника с `--root` и `--json`, коды выхода, envelope | fake |
| Integration | lifecycle на temp-root, mods/state deploy | fake + temp-root |
| Fault | битый архив, path traversal, lock, нет места, systemd error, rollback | fake |
| E2E | чистая VM: установка, вход, бэкап, восстановление, клиент | настоящий |
| Docs | ссылки, отсутствие дублированных схем | нет |

Ключевые фикстуры:

- пустой state с обязательными каталогами;
- state с игроками и всеми пятью world-каталогами;
- state с плоскими каталогами (должен падать);
- state без `current.txt` и без `requiredDataFiles.json`;
- `current.txt` только с sha256;
- моды с CRC и без;
- архив с `../evil`, абсолютным путём, symlink;
- `current.txt`, указывающий на отсутствующий файл;
- конфиги с комментариями и нестандартным порядком ключей.

Temp-root: каждый тест создаёт `t.TempDir()`, кладёт туда `configs/`,
`players/`, `world/`, `mods/`, `backups/` и fake systemd. Это позволяет
проверять почти всё без VPS.

## 11. Non-goals ops

- HTTP-сервер, auth, HTML;
- планировщик с UI — расписание задаёт systemd timer;
- управление клиентом;
- несколько серверов/проектов в одном запуске;
- произвольные shell-команды;
- удаление месячных бэкапов;
- переход со старых форматов и клиентских конфигов.
