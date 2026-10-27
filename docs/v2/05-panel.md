# 05 — Архитектура панели

Документ описывает поведение панели. Нормативные JSON-схемы, коды ошибок и
HTTP-контракты — в `03-contracts.md`; при расхождении прав 03. Панель — это
единственная HTTPS-точка входа, внешний прокси не нужен.

## 1. Роль

Панель — единственная HTTPS-точка входа:

- для админа: браузерный UI и админский API;
- для игрока: публичная страница, список/скачивание бэкапов, текущий архив
  модов;
- для клиента: информация о сервере и архив модов.

Панель не управляет systemd напрямую и не пишет в игровые данные. Она вызывает
`tes3mp-easy-ops` от пользователя `tes3mp` через узкое правило sudoers и
читает файлы для отображения/скачивания.

## 2. Стек

- Go, стандартный `net/http` (роутинг по шаблонам Go 1.22+).
- `html/template` + `embed` для HTML, CSS и минимального JS.
- Никакого Node.js, SPA-фреймворка и сборки фронтенда.
- Argon2id для пароля (`golang.org/x/crypto/argon2`), память 32 MiB, 1 поток.
- TLS: встроенный `github.com/go-acme/lego/v4` получает и продлевает
  сертификат Let's Encrypt для публичного IP (RFC 8738, профиль `shortlived`).
  Внешний ACME-клиент не нужен. Режим `files` служит готовую пару.
- Файлы: `auth.json`, TLS-сертификаты, ACME-аккаунт, uploads. Внешней БД нет.

Почему так: панель — это несколько форм и таблиц. SPA и Node.js добавили бы
сборку, зависимости и ещё один слой отладки без пользы для одного админа.

### Интерфейс и тема

Визуальный слой — один встроенный `internal/panel/static/panel.css`, который
отдаётся same-origin по `/static/panel.css`. Ни CDN, ни Node.js, ни внешних
шрифтов нет: системный sans-serif для текста, serif для заголовков, моноширинный
для кода и логов.

- Тёмная тема TES3MP с янтарным акцентом включена по умолчанию.
- Светлая «пергаментная» тема включается через `prefers-color-scheme: light`;
  отдельной JS-кнопки переключения нет.
- Общий `<head>` вынесен в `templates/head.html`, навигация — в
  `templates/admin_nav.html` и `templates/public_nav.html`.
- Активный раздел админки подсвечивается через `aria-current="page"`;
  значение вычисляется сервером по `r.URL.Path`, включая POST-маршруты.
- Все стили — во внешнем CSS. Inline `style=` запрещён действующим CSP
  (`default-src 'self'`), поэтому шаблоны не содержат inline-стилей.
- Таблицы обёрнуты в `.table-wrap` и прокручиваются по горизонтали; при
  ширине до `720px` карточки, формы и навигация перестраиваются в одну колонку.
- Учитываются `:focus-visible` и `prefers-reduced-motion`; контраст текста
  рассчитан на WCAG AA.

## 3. Процесс

```text
/opt/tes3mp-easy/bin/tes3mp-easy-panel   # HTTP-сервер
/opt/tes3mp-easy/bin/tes3mp-easy-ops     # вызывается как подпроцесс
/opt/tes3mp-easy/templates/              # встроены в бинарник через embed
/opt/tes3mp-easy/static/                 # встроены
/tes3mp-easy/panel-data/                 # изменяемые данные
```

Панель запускается как `tes3mp-panel.service` от пользователя
`tes3mp-panel`, постоянно слушает только `:443`, пишет логи в stdout/stderr →
journald. На время ACME HTTP-01 challenge она занимает `:80` и сразу закрывает
его после проверки. `panel-data` принадлежит `tes3mp-panel:tes3mp` и не зависит
от жизненного цикла игры. Сессии живут в памяти и при рестарте теряются — это
осознанно.

Systemd-лимиты: `MemoryHigh=48M`, `MemoryMax=64M`, `GOMAXPROCS=1`,
`GOMEMLIMIT=64MiB`, `OOMScoreAdjust=+500`, `CAP_NET_BIND_SERVICE` для `:443` и
временного `:80`; bounding set дополнительно содержит
`CAP_SETUID`/`CAP_SETGID`/`CAP_AUDIT_WRITE`/`CAP_SYS_RESOURCE`, иначе setuid-`sudo`
не сможет переключиться на `tes3mp` и применить PAM-лимиты.
`EnvironmentFile=-/etc/tes3mp-easy/panel.env` задаёт режим TLS
и публичный IP. Ops, который панель запускает, наследует cgroup панели; поэтому
все операции с архивами обязаны быть потоковыми и не держать файл в памяти.

### Конфигурация процесса

Отдельного config-файла нет. Настройки читаются один раз при старте из
окружения; флаги командной строки перекрывают env. `SIGHUP` ничего не
перечитывает: изменение настроек требует рестарта unit.

| Env | Флаг | По умолчанию | Смысл |
|---|---|---|---|
| `TES3MP_PANEL_ROOT` | `--root` | `/tes3mp-easy` | Корень данных |
| `TES3MP_PANEL_DATA_DIR` | `--data-dir` | `<root>/panel-data` | Изменяемые данные панели |
| `TES3MP_PANEL_HTTPS_ADDR` | `--https-addr` | `:443` | Единственный постоянный listener |
| `TES3MP_PANEL_PUBLIC_ADDRESS` | `--public-address` | пусто | Публичный IP для ACME-сертификата и API; обязателен в `acme` |
| `TES3MP_PANEL_TLS_MODE` | `--tls-mode` | `acme` | `acme` или `files` |
| `TES3MP_PANEL_ACME_DIRECTORY` | `--acme-directory` | Let's Encrypt production | ACME directory URL |
| `TES3MP_PANEL_ACME_EMAIL` | `--acme-email` | пусто | Необязательный контакт аккаунта |
| `TES3MP_PANEL_ACME_PROFILE` | `--acme-profile` | `shortlived` | Профиль сертификата; для IP обязателен `shortlived` |
| `TES3MP_PANEL_TLS_CERT` | `--tls-cert` | `<data-dir>/tls/cert.pem` | Сертификат (в `files` — входной) |
| `TES3MP_PANEL_TLS_KEY` | `--tls-key` | `<data-dir>/tls/key.pem` | Приватный ключ (в `files` — входной) |

`--public-address` в режиме `acme` принимает только публичный IP без порта,
scheme и path; частные, loopback, link-local, CGNAT (`100.64.0.0/10`) и
зарезервированные диапазоны отклоняются. Bootstrap получает его автоматически
или через `--public-address` и пишет в `/etc/tes3mp-easy/panel.env`:

```ini
TES3MP_PANEL_TLS_MODE=acme
TES3MP_PANEL_PUBLIC_ADDRESS=203.0.113.10
```

Если публичный IP не задан, `/api/v1/server` не сможет отдать стабильный
`address`. В `files` режиме `public_address` не участвует в TLS и может быть
пустым.

Коды выхода: `0` — штатный старт/остановка и `--help`/`--version`, `2` — ошибка
конфигурации, `1` — ошибка ACME/TLS, bind или runtime. В `acme` режиме панель
до приёма трафика проверяет сертификат и, если нужно, выпускает новый; при
ошибке старт завершается. HTTPS-listener открывается только после успешной
проверки/выпуска.

## 4. Маршруты

### Публичные

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/` | Имя сервера, статус, текущие моды, ссылки на бэкапы, вход |
| GET | `/backups` | Публичный список mods/state с пагинацией |
| GET | `/backups/mods/<name>` | Скачать архив модов |
| GET | `/backups/state/<name>` | Скачать state |
| GET | `/api/v1/server` | Информация для клиента (без пароля) |
| GET | `/api/v1/mods/current` | Метаданные текущего архива модов |
| GET | `/api/v1/mods/current/archive` | Текущий архив модов |
| GET | `/healthz` | Liveness для systemd/мониторинга |

Точные схемы `/api/v1/server`, `/api/v1/mods/current` и
`/api/v1/mods/current/archive` — в `03-contracts.md` §7. Пароль сервера в
публичном API отсутствует; клиент спрашивает его у игрока.

Статические файлы `panel.css` и `admin.js` встроены через `embed` и отдаются
по `/static/`; отдельных маршрутов и внешних CDN для них нет.

Маршруты реализованы в `internal/panel`: `/healthz` и TLS-listener; auth
(`/admin/login`, `/admin/logout`, `/admin/password`); публичные страницы и API
(`public.go`); админские страницы (`admin.go`);
ops-клиент (`ops.go`). Contract-тесты панели живут в `test/contract/panel` и
проверяют cookie, ошибки и реальный бинарник; лимиты памяти — в
`test/resource`.

Публичные скачивания и `/api/v1/mods/current/archive` делят лимит 60 запросов
в минуту на IP. Имена архивов проверяются как `filepath.Base` + `.tar.gz`,
symlink и каталоги не отдаются. `/api/v1/server` берёт `hostname`/`port` из
`configs/tes3mp-server-default.cfg` (пустой `hostname` отдаётся как `TES3MP`)
и `mods_sha256` из `current.txt`; при
недоступном systemd он отвечает `503 service_unavailable`, а не выдумывает
`running`. Публичная HTML-страница при этом остаётся `200` и показывает
предупреждение. Подробности ошибок ops не попадают в публичный HTML: только
код и безопасное сообщение.

### Админские

| Метод | Путь | Назначение |
|---|---|---|
| GET/POST | `/admin/login` | Вход |
| POST | `/admin/logout` | Выход |
| GET | `/admin` | Дашборд |
| POST | `/admin/server/start\|stop\|restart` | Управление сервером |
| GET | `/admin/logs` | Последние логи |
| GET | `/admin/mods` | Список архивов, текущий, загрузка |
| POST | `/admin/mods/upload` | Загрузить архив |
| POST | `/admin/mods/deploy` | Задеплоить |
| POST | `/admin/mods/rollback` | Задеплоить предыдущий |
| GET | `/admin/state` | Список state-бэкапов |
| POST | `/admin/state/create` | Создать бэкап сейчас |
| POST | `/admin/state/deploy` | Восстановить state |
| GET | `/admin/config` | Формы настроек |
| POST | `/admin/config/server\|config\|banlist` | Сохранить настройки |
| GET | `/admin/staff` | Игроки и ранги |
| POST | `/admin/staff/<player>` | Сменить ранг |
| GET/POST | `/admin/password` | Сменить пароль |
| GET | `/api/v1/admin/logs?tail=200` | Логи для автообновления |
| GET | `/api/v1/admin/status` | Статус сервера и backup-таймера для автообновления |

Все админские GET требуют сессию, а POST — ещё и CSRF. Дашборд и страница логов
обновляют статус и журнал через `/api/v1/admin/{status,logs}` раз в 5 секунд
(`internal/panel/static/admin.js`, без inline-скриптов). Upload идёт потоково:
`MaxBytesReader` на 512 MiB, `MultipartReader`, UUID-имя в
`panel-data/uploads/`, CSRF-поле проверяется в том же проходе, а файл удаляется
после `mods import`. Действия пишут аудит в journald
(`action=mods.deploy archive=... result=ok`) без паролей, сессий и содержимого
файлов. `doctor` кэшируется на минуту; статус `error` блокирует мутации с
`503` и показывает проблемные проверки.

## 5. Аутентификация

`panel-data/auth.json`:

```json
{
  "version": 1,
  "algorithm": "argon2id",
  "salt": "base64...",
  "hash": "base64...",
  "params": {"time": 3, "memory": 32768, "threads": 1, "key_len": 32}
}
```

- При первом запуске генерируется случайный пароль (24 символа base64url),
  пишется в `panel-data/initial-password.txt` с правами `0600` и один раз
  печатается в journald.
- При первом входе панель требует сменить пароль.
- После успешной смены пароля панель удаляет `initial-password.txt`.
- Хранение — только хеш; открытый пароль после смены нигде не остаётся.
- Argon2id: 32 MiB, 1 поток, `time=3`. Это осознанно меньше, чем на большой
  машине: панель должна укладываться в 64 MiB вместе с Go-рантаймом.
- Сброс — через SSH:

  ```bash
  sudo systemctl stop tes3mp-panel.service
  sudo -u tes3mp-panel /opt/tes3mp-easy/bin/tes3mp-easy-panel reset-password
  sudo systemctl start tes3mp-panel.service
  sudo cat /tes3mp-easy/panel-data/initial-password.txt
  ```

  Команда перезаписывает `auth.json`, создаёт новый `initial-password.txt` и
  печатает временный пароль. TLS-сертификаты, загрузки и бэкапы не
  затрагиваются. Панель останавливается, чтобы не было двух писателей
  `auth.json`.

Сессии:

- случайный 32-байтный ID в cookie `tes3mp_session`;
- `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`;
- серверный map в памяти; рестарт панели = повторный вход;
- idle timeout 12 часов, absolute timeout 7 дней;
- CSRF-токен привязан к сессии и вставляется в каждую форму;
- форма логина — исключение: до сессии панель выдаёт короткоживущий
  pre-session CSRF-токен в отдельной cookie и проверяет его при POST; иначе
  логин был бы уязвим к login CSRF;
- заголовки: `Content-Security-Policy`, `X-Frame-Options: DENY`,
  `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`.

Защита входа:

- не более 5 неудачных попыток за 15 минут с одного IP;
- задержка/429 после превышения;
- сравнение хеша — штатный `argon2` verify, без утечки времени;
- логирование неудачных попыток без пароля.

## 6. Публичные бэкапы

Список пагинируется через ops, а не через чтение каталога панелью:

```text
GET /backups?type=state&before=<name>
  panel ──sudo -u tes3mp──► ops backups list --type state --limit 200 --before <name> --json
  panel группирует items по дням/неделям/месяцам для отображения
```

- `limit` по умолчанию 200, максимум 500; `next_before` из ответа становится
  ссылкой «показать более старые»;
- без `type` страница открывает `state`; `type=mods` показывает архивы модов,
  переключатель — обычные ссылки, без изменения данных;
- `before` проверяется как безопасное имя `.tar.gz` до вызова ops;
- группировка только визуальная: контракт списка остаётся плоским; свежие
  архивы группируются по дням, старше 7 дней — по ISO-неделям, старше года —
  по месяцам;
- метаданные берём у ops, чтобы не дублировать сортировку и определение
  `current`;
- скачивание — прямое чтение файла:

```text
GET /backups/mods/<name>
  name = filepath.Base(name)
  if name != original || !strings.HasSuffix(name, ".tar.gz") → 404
  file = backups/mods/<name>
  http.ServeContent(w, r, name, mtime, file)
```

- без авторизации;
- без directory listing;
- `Content-Type: application/gzip`,
  `Content-Disposition: attachment; filename="..."`;
- rate limit 60 скачиваний в минуту на IP;
- текущий архив модов помечен в списке, но скачивается так же;
- удаления и переименования через публичные маршруты нет.

Почему публично: это осознанное решение из целей. Если бэкапы станут секретом,
это отдельный режим с токеном, он не реализован. Публичная страница показывает
только имя, размер, дату и тип; путей и логов там нет.

## 7. Интеграция с ops

```go
type Ops interface {
    Status(ctx context.Context, unit string) (Status, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    Logs(ctx context.Context, tail int) ([]string, error)
    BackupsList(ctx context.Context, typ string, limit int, before string) (Backups, error)
    BackupCreate(ctx context.Context, typ string, manual bool) (Backup, error)
    ModsImport(ctx context.Context, path string) (ImportResult, error)
    ModsDeploy(ctx context.Context, ref string) (DeployResult, error)
    StateDeploy(ctx context.Context, name string) (DeployResult, error)
    StaffSet(ctx context.Context, player string, rank int) error
    ConfigGet(ctx context.Context, file string) (map[string]any, error)
    ConfigSet(ctx context.Context, file, key string, value any) error
}
```

Production-реализация:

```text
sudo -n -u tes3mp /opt/tes3mp-easy/bin/tes3mp-easy-ops \
  --root /tes3mp-easy --json mods deploy --archive <name>
```

- аргументы — массив, не строка;
- stdout парсится как JSON-envelope, stderr идёт в journald;
- exit code → HTTP-статус и `error.code`;
- таймауты — по таблице `03-contracts.md` §8: чтение 5 с, короткие мутации
  2 мин, импорт/деплой/state 10 мин;
- одновременно может идти только одна мутация: ops lock, панель показывает
  «операция уже выполняется» при `locked`.

`internal/panel/ops.go` реализует это: `OpsClient` с фиксированным
argv, `OpsError` с exit code и envelope, `OpsHTTPStatus` для обработчиков.
`stdout` ограничен 1 MiB, `stderr` — 64 KiB и уходит в logger; второй
JSON-объект или мусор после envelope считаются `internal`. `doctor` — отдельное
исключение: при exit 10 отчёт возвращается вместе с ошибкой `checks_failed`,
чтобы панель показала проблемные проверки.

В тестах `Ops` — fake: обработчики проверяются без systemd и без бинарника.

## 8. Ошибки

| Ops | HTTP | Что видит админ |
|---|---|---|
| `invalid_argument` / `unsafe_path` | 400 | «Проверьте имя/путь» |
| `not_found` | 404 | «Архив не найден» |
| `locked` | 409 | «Другая операция выполняется» |
| `no_space` | 507 | «Мало места, скопируйте старые бэкапы на другую машину» |
| `service_unavailable` | 503 | «systemd, journald или unit недоступен» |
| `archive_invalid` | 422 | Конкретная причина: CRC, путь, старый формат, обязательный файл |
| `config_invalid` | 422 | Ошибка рядом с полем |
| `checks_failed` (exit 10) | 503 | «Проверьте doctor»: панель показывает список ошибок |
| `internal` | 500 | «Внутренняя ошибка», лог с id |

Панель не показывает stack trace и не подменяет ошибку общим «что-то пошло не
так», если ops дал конкретную причину.

## 9. Моды

Страница `/admin/mods`:

- таблица архивов: имя, размер, дата, `current`;
- кнопка «Загрузить архив» и раскрытая подсказка «Формат архива»: дерево
  `plugins/` + `scripts/`, обязательный `plugins/requiredDataFiles.json`, CRC32,
  пример JSON и запрет на базовые ESM;
- кнопка «Deploy» у выбранного;
- кнопка «Rollback» — деплой предыдущего архива;
- текущий мод-пак и его `sha256`;
- результат последней операции.

Загрузка:

```text
POST /admin/mods/upload (multipart, max 512 MiB)
  panel стримит в panel-data/uploads/<uuid>.tar.gz
  файл создаётся 0600, после записи получает 0640 (группа tes3mp)
  panel ──sudo -u tes3mp──► ops mods import --file ... --json
  при успехе: удалить upload, если он ещё существует
              (ops перенёс его в backups/mods/ — rename или копирование)
  при ошибке: удалить upload и показать причину (ops уже вернул детали)
```

Лимит проверяется до записи: `http.MaxBytesReader` на тело запроса,
`r.MultipartReader` для потокового чтения части, без `ParseMultipartForm` и
без `io.ReadAll` всего файла. Имя исходного файла не используется как путь;
на диске — UUID. `panel-data/uploads` и `/tes3mp-easy/backups` должны быть на
одной файловой системе; `doctor` проверяет `stat.Dev`, а `mods import`
возвращает `invalid_argument`, если устройства разные.

Panel unit использует один `ReadWritePaths=/tes3mp-easy`, поэтому uploads,
`backups/`, `.staging` и `mods/` находятся на одном mount и `rename` работает
без копирования. Для нестандартных unit-файлов `mods import` сохраняет
fallback: при `EXDEV` копирует upload в `.part` в `backups/mods`, делает
`fsync`, переименовывает `.part` в целевое имя и удаляет upload. Для
`mods deploy` и `state deploy` такого fallback нет: granular-пути в panel unit
недопустимы.

Деплой:

- подтверждение с предупреждением «сервер будет перезапущен, если он запущен»;
- `ops mods deploy --archive <name> --json`;
- панель показывает прогресс-заглушку (операция синхронная, но может занять
  минуты), затем результат;
- при `archive_invalid` показывает конкретную причину;
- после успеха перечитывает `current` и `mods_sha256` для интерфейса и
  публичного API.

Rollback — это не отдельная магия, а `mods deploy --archive <предыдущий>`.
Список уже отсортирован, панель знает соседний архив.

## 10. State

Страница `/admin/state`:

- список state-архивов (новые сверху), размер, дата, класс
  (`state`, `state-manual`, `state-predeploy`);
- пагинация через `next_before`, чтобы страница не тянула тысячи записей;
- группировка по дням/неделям/месяцам для длинной истории;
- кнопка «Создать бэкап сейчас» → `ops backups create --type state --manual`;
- кнопка «Восстановить» → подтверждение → `ops state deploy --archive <name>`;
- предупреждение: текущие `players/` и `world/` будут заменены; если сервер
  был запущен, он перезапустится, иначе останется остановленным;
- после успеха — сообщение и обновлённый список бэкапов; `state deploy` не
  создаёт `state-manual`, но всегда делает `state-predeploy-*`.

Панель не даёт удалять state-бэкапы. Это защита от случайного удаления
единственной копии и от удаления месячной истории. Retention работает только
в ops и никогда не удаляет месячные архивы.

## 11. Конфиги

### `server` — `tes3mp-server-default.cfg`

Поля:

- `hostname` (string, 1..64);
- `port` (int, 1024..65535);
- `maximumPlayers` (int, 1..64, по умолчанию 8);
- `password` (string, пусто = без пароля);
- `logLevel` (0..4);
- `[MasterServer] enabled` (bool).

### `config` — `config.lua`

Поля из белого списка:

- `gameMode` (string);
- `loginTime` (int, 10..600);
- `maxClientsPerIP` (int, 1..10, по умолчанию 3);
- `difficulty` (int, -100..100);
- `passTimeWhenEmpty` (bool);
- `nightStartHour`, `nightEndHour` (int, 0..23);
- `allowConsole`, `allowBedRest`, `allowWildernessRest`, `allowWait`,
  `allowSuicideCommand`, `allowFixmeCommand` (bool);
- `shareJournal`, `shareFactionRanks`, `shareTopics`, `shareBounty`,
  `shareReputation` (bool);
- `enforceDataFiles` (bool).

Таблицы (`gameSettings`, `vrSettings`, `defaultTimeTable`, списки) read-only:
показываем, но не редактируем.

### `banlist` — `banlist.json`

Два списка: `playerNames` и `ipAddresses`. Форма — textarea по одному
значению на строку; ops валидирует и пишет JSON атомарно, сохраняя ключи
именно в таком виде.

### Как сохраняем

- Панель отправляет одно поле/ключ, а не весь файл.
- Ops проверяет ключ, тип, диапазон.
- Ops делает `.bak-<timestamp>` и атомарную замену.
- Панель перечитывает значения через `ops config get` и показывает «сохранено».
- Ошибка `config_invalid` приходит с именем поля; панель подсвечивает его.
- Если `ops` вернул `restart_required=true`, показываем баннер «нужен рестарт»
  и кнопку «Перезапустить сервер».
- `customScripts.lua` показывается read-only, с пояснением, что он
  генерируется из `mods/scripts/`.

## 12. Staff

Страница `/admin/staff`:

- список игроков из `players/*.json` (панель читает файлы только для
  отображения);
- имя, текущий `staffRank`, кнопки 0/1/2/3;
- `POST /admin/staff/<player>` → `ops staff set`;
- после успеха — баннер «нужен рестарт», кнопка «Перезапустить сервер».

Панель не редактирует JSON игрока напрямую и не даёт менять другие поля.

## 13. Статус и логи

- Дашборд опрашивает `ops status --unit tes3mp.service --json` каждые 5 секунд
  (fetch + обновление DOM). WebSocket/SSE не используются.
- Логи: `GET /admin/logs?tail=200`, автообновление раз в 5 секунд; кнопка
  «Пауза». Источник — journald через ops.
- Если `ops status` вернул `service_unavailable`, показываем красный баннер и
  не пытаемся изображать «running».
- Если таймер `tes3mp-easy-backup.timer` неактивен, показываем предупреждение
  «автобэкапы не идут» и кнопку «Создать бэкап сейчас».
- Дашборд раз в минуту вызывает `ops doctor --json` и показывает `warn`/`error`
  отдельным баннером; `error` не блокирует вход, но блокирует мутации, которые
  заведомо не смогут пройти (например, нет места).
- Собственные логи панели идут в journald с префиксом `panel:`; отдельного
  `panel.log` нет. Аудит действий: `action=mods.deploy archive=... result=ok`,
  без паролей, сессионных ID и содержимого файлов.

## 14. TLS

- Панель постоянно слушает только `:443`; TLS не ниже 1.2.
- В режиме `acme` (по умолчанию) встроенный lego получает сертификат Let's
  Encrypt для публичного IP по RFC 8738. Let's Encrypt требует профиль
  `shortlived` (шесть дней); `--acme-profile` можно переопределить, но другой
  профиль для IP не выдаст сертификат.
- CSR строится через `ObtainForCSR` с пустым Common Name и IP только в
  subjectAltName: lego `Obtain` кладёт первый identifier в CN, а Let's Encrypt
  отклоняет IP в CN с `badCSR`.
- HTTP-01 challenge обслуживает `http01.NewProviderServer("", "80")`. Порт 80
  открывается только на время `Present`/`CleanUp`; после выпуска или продления
  listener закрывается. Постоянного HTTP-редиректа нет.
- Аккаунт ACME хранится в `panel-data/acme/account.key` (`0600`, каталог `0700`).
  При старте панель находит аккаунт по ключу или регистрирует его (Terms of
  Service Let's Encrypt принимаются автоматически); email необязателен.
- Сертификаты и ключи — в `panel-data/tls/`, права `0600`, каталог `0700`.
  Запись атомарная (temp + rename).
- При старте:
  - если пара есть и сертификат покрывает публичный IP и ещё не в окне
    продления, панель его использует;
  - legacy self-signed пара заменяется, даже если покрывает IP: ACME-режим не
    должен обслуживать локально сгенерированный сертификат;
  - если сертификат истекает, панель пытается продлить его сейчас; при ошибке
    продолжает работу со старым сертификатом и повторяет попытку в фоне;
  - если пары нет, сертификат просрочен или не покрывает IP, выпуск
    обязателен, и его ошибка завершает старт.
- Фоновое продление проверяет сертификат каждые 6 часов. Окно — треть срока
  жизни (для шестидневного сертификата — двое суток), не больше 30 суток.
  После ошибки повтор через 30 минут; сертификат подменяется атомарно, без
  рестарта и без разрыва соединений.
- Режим `files` не обращается к ACME и требует готовые `cert.pem`/`key.pem`.
  Он предназначен для нестандартных установок и тестов; self-signed генерации
  в production-коде нет.
- Домен не требуется. Порты 443 и временный 80 открываются ambient-капабилити
  `CAP_NET_BIND_SERVICE`, а не запуском панели под root.
- Для отладки можно направить `--acme-directory` на staging; production-ветка
  Let's Encrypt имеет лимиты на неудачные проверки.

## 15. Данные панели

```text
panel-data/
├── auth.json                 # Argon2id-хеш, 0600
├── initial-password.txt      # только до первой смены, 0600
├── acme/
│   └── account.key           # ключ аккаунта ACME, 0600
├── tls/
│   ├── cert.pem              # текущий сертификат
│   └── key.pem               # текущий ключ
└── uploads/                  # временные архивы модов, чистится после import
```

Сессии — в памяти. Если панель перезапустилась, админ входит заново.
Это проще, чем хранить сессии на диске, и не мешает работе сервера.

`uploads/` — единственное место, куда панель пишет пользовательские файлы.
Файлы называются UUID, а не исходным именем. После успешного импорта ops
переносит upload в `backups/mods/` (rename в общем `/tes3mp-easy` mount или
атомарное копирование при `EXDEV`), и панель не пытается удалить уже
несуществующий путь.

## 16. Безопасность

- Панель не отдаёт файлы по произвольному пути: только `backups/mods/<name>`
  и `backups/state/<name>` после `filepath.Base` и проверки суффикса.
- Нет file manager, web terminal, raw editor.
- Все формы — CSRF.
- Все изменяющие действия — POST, без GET-побочек.
- Загрузка — `MaxBytesReader` 512 MiB и потоковый `MultipartReader`; проверка
  архива в ops, а не в панели.
- Ops вызывается с фиксированными аргументами; имя архива и игрока
  проверяются, но не вставляются в shell.
- Панель не входит в группу с правом записи в игровые данные; sudoers даёт
  только запуск ops от `tes3mp`.
- Прямого доступа к управлению хостом нет, только два узких sudo-правила.
- Публичные страницы не раскрывают пути, версии с уязвимостями и логи.
- Логин и скачивания — rate limit.
- Заголовки безопасности и HTTPS — обязательны.

## 17. Тестирование

| Что | Как |
|---|---|
| Роутинг и шаблоны | `httptest`, проверка статусов и ключевых элементов HTML |
| CSS и тема | `/static/panel.css` отдаётся как `text/css`; полные шаблоны подключают общий `head`; нет `style=`; активный раздел помечен `aria-current` |
| Логин | верный/неверный пароль, rate limit, cookie-флаги, CSRF |
| Argon2id | memory 32768, threads 1, time 3, verify |
| Публичные скачивания | path traversal, не-`.tar.gz`, отсутствующий файл, `Content-Disposition` |
| Пагинация | `limit`/`before`, `next_before`, группировка по дням/неделям/месяцам |
| Загрузка модов | лимит 512 MiB, UUID-имя, `MultipartReader`, вызов fake ops, очистка upload |
| Ошибки ops | каждый `error.code` → ожидаемый HTTP и текст |
| Конфиги | валидация, повторное чтение, `config_invalid` |
| Staff | список из `players/*.json`, вызов `ops staff set` |
| Lifecycle и TLS | конфиг env/flags, ACME fake-obtainer (IP SAN, shortlived profile, wrong IP, renewal window, отказ продления, замена legacy self-signed), `files` pair, TLS ≥ 1.2, `/healthz`, HSTS, bind rollback, `SIGTERM` |
| Интеграция | реальный ops на temp-root, fake systemd/journalctl |

Панель не тестирует tar и CRC — это тесты ops. Панель тестирует, что она
правильно вызывает ops и правильно показывает результат. Тест на загрузку
большого файла проверяет, что память панели не растёт пропорционально
размеру upload. Bind на 443 и временный 80 проверяются на VM с
`CAP_NET_BIND_SERVICE`; unit-тесты используют эфемерные порты и не требуют root.

## 18. Non-goals панели

- несколько админов, роли, приглашения;
- регистрация и email;
- удаление/переименование бэкапов;
- файловый менеджер и редактор любых файлов;
- веб-терминал;
- управление другими серверами;
- мод-браузер и загрузка с Nexus;
- уведомления в Telegram/Discord;
- SSO, 2FA, passkeys не реализованы;
- SPA и отдельный фронтенд-билд.
