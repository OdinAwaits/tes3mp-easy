# 05 — Архитектура панели

**Статус:** draft.

**Обновлено:** 2026-10-22.

Документ описывает поведение панели. Нормативные JSON-схемы, коды ошибок и
HTTP-контракты — в `03-contracts.md`; при расхождении прав 03. Панель заменяет
схему с `nginx` + `export` + `certbot`.

## 1. Роль

Панель — единственная HTTPS-точка входа:

- для админа: браузерный UI и админский API;
- для игрока: публичная страница, список/скачивание бэкапов, текущий архив
  модов;
- для клиента: информация о сервере и архив модов.

Панель не управляет Docker напрямую и не пишет в игровые данные. Она вызывает
`tes3mp-easy-ops` и читает файлы для отображения/скачивания.

## 2. Стек

- Go, стандартный `net/http` (роутинг по шаблонам Go 1.22+).
- `html/template` + `embed` для HTML, CSS и минимального JS.
- Никакого Node.js, SPA-фреймворка и сборки фронтенда.
- Argon2id для пароля (`golang.org/x/crypto/argon2`).
- Встроенный ACME-клиент для Let's Encrypt по IP; отдельного `certbot` нет.
- Файлы: `auth.json`, `panel.log`, TLS-сертификаты, uploads. Внешней БД нет.

Почему так: панель — это несколько форм и таблиц. SPA и Node.js добавили бы
сборку, зависимости и ещё один слой отладки без пользы для одного админа.

## 3. Процесс

```text
/app/tes3mp-easy-panel     # HTTP-сервер
/app/tes3mp-easy-ops       # вызывается как подпроцесс
/app/templates/            # встроены в бинарник через embed
/app/static/               # встроены
/tes3mp-easy/panel-data/   # изменяемые данные
```

Панель запускается как PID 1 в контейнере, слушает `:80` и `:443`, пишет логи
в stdout/stderr. `panel-data` монтируется volume'ом, чтобы пароль, TLS и
загрузки переживали пересоздание контейнера. Сессии живут в памяти и при
рестарте теряются — это осознанно.

## 4. Маршруты

### Публичные

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/` | Имя сервера, статус, текущие моды, ссылки на бэкапы, вход |
| GET | `/backups` | Публичный список mods/state |
| GET | `/backups/mods/<name>` | Скачать архив модов |
| GET | `/backups/state/<name>` | Скачать state |
| GET | `/api/v1/server` | Информация для клиента (без пароля) |
| GET | `/api/v1/mods/current` | Метаданные текущего архива модов |
| GET | `/api/v1/mods/current/archive` | Текущий архив модов |
| GET | `/healthz` | Liveness для compose/мониторинга |

Точные схемы `/api/v1/server`, `/api/v1/mods/current` и
`/api/v1/mods/current/archive` — в `03-contracts.md` §7. Пароль сервера в
публичном API отсутствует; клиент спрашивает его у игрока.

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

Админские действия — обычные HTML-формы с POST/redirect/GET. Никакого
отдельного JSON API для каждой кнопки: браузеру он не нужен.

## 5. Аутентификация

Один админ, один пароль. Никаких пользователей, ролей и email.

`panel-data/auth.json`:

```json
{
  "version": 1,
  "algorithm": "argon2id",
  "salt": "base64...",
  "hash": "base64...",
  "params": {"time": 3, "memory": 65536, "threads": 2, "key_len": 32}
}
```

- При первом запуске генерируется случайный пароль (20+ символов), пишется в
  `panel-data/initial-password.txt` с правами `0600` и один раз печатается в
  лог.
- При первом входе панель требует сменить пароль.
- Хранение — только хеш; открытый пароль после смены нигде не остаётся.
- Сброс — через SSH:
  `docker compose exec panel tes3mp-easy-panel reset-password`. Команда
  перезаписывает `auth.json`, создаёт новый `initial-password.txt` и печатает
  временный пароль. Если панель не запущена, тот же бинарник можно вызвать
  через `docker compose run --rm panel ...`.

Сессии:

- случайный 32-байтный ID в cookie `tes3mp_session`;
- `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`;
- серверный map в памяти; рестарт панели = повторный вход (это нормально);
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

Список:

```text
GET /backups
  panel ──exec──► ops backups list --type mods --json
  panel ──exec──► ops backups list --type state --json
```

Метаданные берём у ops, чтобы не дублировать сортировку и определение
`current`. Скачивание — прямое чтение файла:

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
это отдельный режим с токеном, но не MVP.

## 7. Интеграция с ops

```go
type Ops interface {
    Status(ctx context.Context) (Status, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    Logs(ctx context.Context, tail int) ([]string, error)
    BackupsList(ctx context.Context, typ string) (Backups, error)
    BackupCreate(ctx context.Context, typ string) (Backup, error)
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
exec.CommandContext(ctx, opsBinary,
  "--root", root, "--json", "mods", "deploy", "--archive", name)
```

- аргументы — массив, не строка;
- stdout парсится как JSON-envelope, stderr идёт в лог;
- exit code → HTTP-статус и `error.code`;
- таймауты — по таблице `03-contracts.md` §8: чтение 5 с, короткие мутации
  2 мин, импорт/деплой/state 10 мин;
- одновременно может идти только одна мутация: ops lock, панель показывает
  «операция уже выполняется» при `locked`.

В тестах `Ops` — fake: обработчики проверяются без Docker и без бинарника.

## 8. Ошибки

| Ops | HTTP | Что видит админ |
|---|---|---|
| `invalid_argument` / `unsafe_path` | 400 | «Проверьте имя/путь» |
| `not_found` | 404 | «Архив не найден» |
| `locked` | 409 | «Другая операция выполняется» |
| `no_space` | 507 | «Мало места, удалите старые бэкапы» |
| `docker_unavailable` | 503 | «Docker недоступен» |
| `archive_invalid` | 422 | Конкретная причина: CRC, путь, обязательный файл |
| `config_invalid` | 422 | Ошибка рядом с полем |
| `checks_failed` | 503 | «Проверьте doctor»: панель показывает список ошибок |
| `internal` | 500 | «Внутренняя ошибка», лог с id |

Панель не показывает stack trace и не подменяет ошибку общим «что-то пошло не
так», если ops дал конкретную причину.

## 9. Моды

Страница `/admin/mods`:

- таблица архивов: имя, размер, дата, `current`;
- кнопка «Загрузить архив»;
- кнопка «Deploy» у выбранного;
- кнопка «Rollback» — деплой предыдущего архива;
- текущий мод-пак и его `sha256`;
- результат последней операции.

Загрузка:

```text
POST /admin/mods/upload (multipart, max 2 GiB)
  panel стримит в panel-data/uploads/<uuid>.tar.gz
  panel ──exec──► ops mods import --file ... --json
  при успехе: удалить upload, если он ещё существует
              (ops мог переименовать/перенести его в backups/mods/)
  при ошибке: удалить upload и показать причину (ops уже вернул детали)
```

Лимит размера проверяется до записи (`http.MaxBytesReader`). Имя исходного
файла не используется как путь; на диске — UUID.

Деплой:

- подтверждение с предупреждением «сервер будет перезапущен»;
- `ops mods deploy --archive <name> --json`;
- панель показывает прогресс-заглушку (операция синхронная, но может занять
  минуты), затем результат;
- при `archive_invalid` показывает конкретную причину;
- после успеха обновляет `current` и `mods_sha256` в публичном API.

Rollback — это не отдельная магия, а `mods deploy --archive <предыдущий>`.
Список уже отсортирован, панель знает соседний архив.

## 10. State

Страница `/admin/state`:

- список state-архивов (новые сверху), размер, дата;
- кнопка «Создать бэкап сейчас» → `ops backups create --type state`;
- кнопка «Восстановить» → подтверждение → `ops state deploy --archive <name>`;
- предупреждение: текущие `players/` и `world/` будут заменены, сервер
  перезапустится;
- после успеха — сообщение и обновлённый список бэкапов; `state deploy` сам
  новый бэкап не создаёт.

Панель не даёт удалять state-бэкапы в MVP. Это защита от случайного удаления
единственной копии.

## 11. Конфиги

### `server` — `tes3mp-server-default.cfg`

Поля:

- `hostname` (string, 1..64);
- `port` (int, 1024..65535);
- `maximumPlayers` (int, 1..64);
- `password` (string, пусто = без пароля);
- `logLevel` (0..4);
- `[MasterServer] enabled` (bool).

### `config` — `config.lua`

Поля из белого списка:

- `gameMode` (string);
- `loginTime` (int, 10..600);
- `maxClientsPerIP` (int, 1..10);
- `difficulty` (int, -100..100);
- `passTimeWhenEmpty` (bool);
- `nightStartHour`, `nightEndHour` (int, 0..23);
- `allowConsole`, `allowBedRest`, `allowWildernessRest`, `allowWait`,
  `allowSuicideCommand`, `allowFixmeCommand` (bool);
- `shareJournal`, `shareFactionRanks`, `shareTopics`, `shareBounty`,
  `shareReputation` (bool);
- `enforceDataFiles` (bool).

Таблицы (`gameSettings`, `vrSettings`, `defaultTimeTable`, списки) в MVP
read-only: показываем, но не редактируем.

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

- Дашборд опрашивает `ops status --json` каждые 5 секунд (fetch + обновление
  DOM). WebSocket/SSE в MVP не нужны.
- Логи: `GET /admin/logs?tail=200`, автообновление раз в 5 секунд; кнопка
  «Пауза».
- Если `ops status` вернул `docker_unavailable`, показываем красный баннер и
  не пытаемся изображать «running».
- Если `backup` не запущен, показываем предупреждение «автобэкапы не идут».
- Дашборд раз в минуту вызывает `ops doctor --json` и показывает `warn`/`error`
  отдельным баннером; `error` не блокирует вход, но блокирует мутации, которые
  заведомо не смогут пройти (например, нет места).

## 14. TLS

- Панель слушает `:80` для ACME HTTP-01 и редиректа на HTTPS.
- `:443` — TLS.
- Целевой режим: Let's Encrypt по IP (short-lived), встроенный ACME-клиент,
  без отдельного `certbot`.
- Сертификаты и ключи — в `panel-data/tls/`, права `0600`.
- Если выпустить сертификат не удалось, панель стартует с self-signed и
  показывает предупреждение. Это аварийный режим: браузер будет ругаться, а
  клиентские загрузки могут не пройти проверку TLS.
- Домен не требуется. Если позже появится домен, ACME-код не меняется.

## 15. Данные панели

```text
panel-data/
├── auth.json                 # Argon2id-хеш
├── initial-password.txt      # только до первой смены, 0600
├── tls/
│   ├── cert.pem
│   └── key.pem
├── uploads/                  # временные архивы модов, чистится после import
└── panel.log                 # ошибки и аудит действий (без паролей)
```

Сессии в MVP — в памяти. Если панель перезапустилась, админ входит заново.
Это проще, чем хранить сессии на диске, и не мешает работе сервера.

`panel.log` — не полноценный audit trail, а лог действий:
`2026-10-22T12:00:00Z action=mods.deploy archive=... result=ok`. Без паролей,
сессионных ID и содержимого файлов.

## 16. Безопасность

- Панель не отдаёт файлы по произвольному пути: только `backups/mods/<name>`
  и `backups/state/<name>` после `filepath.Base` и проверки суффикса.
- Нет file manager, web terminal, raw editor.
- Все формы — CSRF.
- Все изменяющие действия — POST, без GET-побочек.
- Загрузка — `MaxBytesReader`, проверка архива в ops, а не в панели.
- Ops вызывается с фиксированными аргументами; имя архива и игрока
  проверяются, но не вставляются в shell.
- Docker socket не доступен из интернета и не проксируется.
- Публичные страницы не раскрывают пути, версии с уязвимостями и логи.
- Логин и скачивания — rate limit.
- Заголовки безопасности и HTTPS — обязательны.

## 17. Тестирование

| Что | Как |
|---|---|
| Роутинг и шаблоны | `httptest`, проверка статусов и ключевых элементов HTML |
| Логин | верный/неверный пароль, rate limit, cookie-флаги, CSRF |
| Публичные скачивания | path traversal, не-`.tar.gz`, отсутствующий файл, `Content-Disposition` |
| Загрузка модов | лимит размера, UUID-имя, вызов fake ops, очистка upload |
| Ошибки ops | каждый `error.code` → ожидаемый HTTP и текст |
| Конфиги | валидация, повторное чтение, `config_invalid` |
| Staff | список из `players/*.json`, вызов `ops staff set` |
| Интеграция | реальный ops на temp-root, фейковый Docker |

Панель не тестирует tar и CRC — это тесты ops. Панель тестирует, что она
правильно вызывает ops и правильно показывает результат.

## 18. Миграция

- `nginx` и `export` удаляются; их публичные URL заменяются на `/backups` и
  `/api/v1/...`.
- Старые бэкапы читаются без конвертации: имена и структура совместимы.
- `certbot` не нужен: ACME внутри панели.
- Старый `list-backups.sh` больше не используется панелью.
- Обратная совместимость со старыми URL не обязательна (проект молодой).
- Панель не пытается импортировать старый пароль/сессии — их нет.

## 19. Non-goals панели

- несколько админов, роли, приглашения;
- регистрация и email;
- удаление/переименование бэкапов;
- файловый менеджер и редактор любых файлов;
- веб-терминал;
- управление другими серверами;
- мод-браузер и загрузка с Nexus;
- уведомления в Telegram/Discord;
- SSO, 2FA, passkeys (можно добавить позже, но не в MVP);
- SPA и отдельный фронтенд-билд.
