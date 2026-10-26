# Передизайн клиента v3: возврат паритета с v1

Макет: [`client-redesign-mockup.html`](client-redesign-mockup.html) — открыть в браузере,
разделы слева и диалоги кликабельны.

Цель: всё локальное — из окна, без терминала и без знания `prefix/pfx/drive_c/...`.
Бэкапы state/модов, админка и SSH остаются в панели.

## 0. Ревью №2 (текущие решения)

1. Мастер — **4 шага**: Morrowind → TES3MP → Fonts → UI and video. Шага Ready нет.
2. На шаге Fonts **одна кнопка Next**; набор только запоминается. Загрузка не
   начинается по клику в мастере, а стартует фоном после **Finish** — с прогрессом,
   отменой и повтором в строке Installation → Fonts.
3. Глобального меню «⋯» нет: тема — переключатель в шапке, остальное разнесено по
   вкладкам рельса. Ни одна функция не доступна из двух мест.
4. Рельс — **Play · Installation · OpenMW-CS · Diagnostics · About**; «＋ Add server»
   остаётся только на Play.

## 1. Диагноз (по текущему коду)

| Проблема | Где видно |
|---|---|
| Нет навигации: только меню из 5 пунктов и диалог «More» | `internal/ui/ui.go:116`, `ui.go:226` |
| Нет экрана состояния компонентов; после `setup_complete` чек-лист недоступен | `internal/app/app.go:129` |
| Нет экранов v1: `configure-ui`, выбор шрифтов, озвучка, uninstall, редактор конфигов | `settings.cfg` в Go-клиенте не пишется нигде |
| Пути известны, но не показываются; открыть можно только папку лога | `logs.go:22` |

## 2. Структура окна

```
┌ TES3MP Easy ──────────────────── [System│Dark│Light] ┐
│ Play           │ Servers                                   │
│ Installation   │  ┌ Saagilat Keep · ACTIVE ──────────────┐ │
│ OpenMW-CS      │  │ play.tes3mp.example:25565           │ │
│ Diagnostics    │  │ ● Panel online    [▶ Play] [More]   │ │
│ About          │  └─────────────────────────────────────┘ │
│                │  ＋ Add server                             │
└────────────────┴───────────────────────────────────────────┘
  config.json · /home/user/.config/tes3mp-easy/config.json
```

- **Rail**: Play · Installation · OpenMW-CS · Diagnostics · About; счётчика действий
  на Installation нет.
- **Шапка**: бренд и переключатель темы System / Dark / Light. Ни «＋», ни «⋯»:
  добавление сервера — только на Play, всё остальное — во вкладках.
- **Diagnostics**: хвост клиентского лога, Copy report, все папки (TES3MP / OpenMW
  config / work dir / config / log).
- **About**: версия, лицензия, ссылка на проект и **Interface language** — единственное
  место, где меняется язык интерфейса.
- **Game language** (пак и озвучка) — только Installation → Game language.
- **Uninstall** — только Installation.
- Отдельного экрана **Settings нет**; глобального меню тоже нет.


## 3. Экраны

### 3.1. Installation (чек-лист, заменяет одноразовый мастер)

| Строка | Статусы | Действия |
|---|---|---|
| Morrowind | найдено / неполное / не выбрано | Change folder… (проверка — при входе на вкладку, отдельной кнопки Check нет) |
| TES3MP 0.8.1 | установлен / устарел / нет | Install / Reinstall (папка — в Diagnostics) |
| Fonts | набор X установлен / частично / нет | Install… / Change set… — один вход в диалог; пока идёт фоновая загрузка, на месте кнопки прогресс, Cancel и Retry |
| Game language | пакет не установлен (нейтрально) / установлен (язык, текст или текст + озвучка) | Install language pack… / Change… — Remove внутри диалога |
| UI and video | настроено / `settings.cfg` ещё нет | Configure… |

Сверху **Run wizard** — тот же набор шагов по одному. Строки — из существующих
`app.TES3MPState`, `app.FontsState`, `app.LocalizationState`, `app.ModsState`,
`app.CheckMorrowind`; добавляются `ClientSettingsState` и `EditorState`.

Ниже свёрнутый **Advanced** (`widget.Accordion`): Work directory (платформенный
дефолт, «Change…»), Proton (Linux) и строка Client profile со статусом **AUTOMATIC** —
`openmw.cfg` и endpoint пересобираются из активного профиля при каждом Play,
отдельной кнопки «Recreate» не нужно. Папок, лога и отчёта здесь больше нет.
Внизу — только Uninstall (danger zone).

### 3.2. OpenMW-CS (отдельный раздел)

Своя вкладка, потому что на Linux редактор запускается хитрее игры: он должен
стартовать в том же Proton-префиксе, иначе не увидит `Documents/My Games/OpenMW`.

- Статус: есть ли `openmw-cs.exe` (`launcher.OpenMWCSExecutable`) в `tes3mp.Dir`, путь;
  кнопки **Launch editor**, Check again, Open CS folder.
- Нужны установленные моды активного сервера — та же проверка, что у Play
  (`ErrModsNotInstalled`): пока их нет, кнопка неактивна и объясняет почему.
- Linux: редактор идёт через тот же Proton-префикс, что и игра; если префикса ещё нет,
  клиент готовит его так же, как перед Play.
- «What the editor sees»: Morrowind + `data/Data Files` активного сервера; Play
  пересобирает `openmw.cfg`, но файлы модов не трогает (и панель их не получает).
- Редактора нет в сборке — показываем ожидаемый путь и ссылку на релиз OpenMW,
  без тихого отказа.

### 3.3. Мастер: 4 шага

Morrowind → TES3MP → Fonts → **UI and video (Finish)**.

- Шага Work directory больше нет: work dir = платформенный дефолт
  (`~/.local/share/tes3mp-easy`), меняется в Installation → Advanced.
- На Fonts одна кнопка **Next**: выбранный набор (Noto по умолчанию) только
  сохраняется в `font_set`, никакой загрузки и никакой развилки
  «Next / Install fonts and continue».
- После **Finish** пишется `setup_complete`, диалог закрывается, выбранный набор
  скачивается фоном; строка Installation → Fonts показывает прогресс, **Cancel** и
  **Retry**. До окончания игра стартует со стоковым шрифтом, следующий Play повторяет.
- Шага **Ready нет**: его сводка дублировала Installation, `Finish` стоит на шаге 4.
- Шага Localization нет: пак ставится из Installation → Game language.

### 3.4. Diagnostics (отдельный раздел)

Единственное место для лога, отчёта и путей — до этого они дублировались в меню,
Installation → Advanced → Folders и в карточке Diagnostics.

- **Client log**: последние 200 строк (`app.ReadLogTail`), Refresh и Open log folder.
- **Report**: Copy report (тот же текст, что копирует диалог ошибки), размер, отметка
  «ошибок с запуска нет».
- **Folders**: TES3MP, OpenMW config, work dir, client config, client log — только
  открытие; смена work dir остаётся в Installation → Advanced.

### 3.5. About (отдельный раздел)

- Версия клиента и TES3MP, лицензия, ссылка на проект, список компонентов.
- **Interface language** — единственный селектор языка интерфейса
  (`app.UpdateSettings` + `i18n.Resolve`), применяется сразу; в коде отдельного
  экрана настроек для этого не нужно.
- Тема сюда не дублируется: она в шапке.


## 4. Новое в коде

| Слой | Изменения |
|---|---|
| `internal/openmwcfg` (новый) | INI-редактор `settings.cfg`: `[GUI] ttf resolution / font size / scaling factor`, `[Video] framerate limit`; сохранять порядок, комментарии и CRLF; бэкап перед записью |
| `internal/app` | `ClientSettings/UpdateClientSettings`, `EditorState`, `SetFontSet/FontsState`, `Uninstall(scope)`, `InstallLocalization(..., voices bool)`; `Settings` сокращается до Language/Theme/MorrowindPath/WorkDir/ProtonPath, `SkipFonts` убирается |
| `internal/ui` | `ui.go` + `screen_play.go`, `screen_install.go`, `screen_cs.go`, `screen_diagnostics.go`, `screen_about.go`; диалоги `dialog_uivideo.go`, `dialog_fonts.go`, `dialog_uninstall.go`, `language_wizard.go` (диалог Game language); `wizard.go` — 4 шага и фоновая загрузка шрифтов после Finish; удаляются `appMenu`/`headerActions`, `showSettings` (`dialogs.go:156`) и редактор произвольных конфигов |
| `internal/config` | поля `FontSet`, `Voiceover` (omitempty); `skip_fonts` больше не читается — старые конфиги грузятся, набор по умолчанию `noto` |
| `internal/fonts` | `Sets` — каталог наборов (`Options.Set`): 3 набора в P1, все 12 из v1 в P2; текущее поведение = `noto` |
| `internal/i18n` | ~40 ключей en+ru (install, editor, fonts, uninstall, uivideo, diagnostics, about); ключи меню (`menu.*`, `rail.menu_note`) и Ready-шага удаляются |

## 5. Порядок работ

1. **P0** — rail, Installation из существующих `app`-методов, экран OpenMW-CS (на `app.OpenMWCS`), меню «⋯» (язык, тема, лог, отчёт, About); `showSettings` удаляется. **Сделано; меню в P1.1 удалено.**
2. **P1** — `openmwcfg` + UI and video, обязательный шаг Fonts и `font_set`, озвучка как опция, Uninstall, мастер 5 шагов (в P1.1 сокращён до 4). **Сделано:** шаг Fonts вместо шага Work directory (кнопки Skip больше нет), `internal/openmwcfg` (settings.cfg с сохранением комментариев/CRLF и бэкапом), строка Installation → UI and video и шаг 4 мастера, каталог `internal/fonts.Sets` с тремя наборами (`noto`/`inter`/`plex`), поле `font_set` в `config.json`, диалог Installation → Fonts → Change set и выбор набора на шаге 3 мастера, danger zone Installation → Uninstall (`app.Uninstall(scope)` с флагами GameFiles/Cache/Settings и защитой Morrowind, home и config). **Уточнение после ревью:** шага Localization в мастере нет — диалог **Game language** открывается из строки Installation, вариант «только текст / текст + озвучка» (и `voiceover` в `config.json`) выбирается там же, кнопка **Remove** видна только при установленном пакете; отсутствие пакета не считается ошибкой.
3. **P1.1 (ревью №2; макет готов, код не начат)** — убрать меню «⋯» и дубли: тема в шапку, Diagnostics и About отдельными вкладками, «＋ Add server» только на Play, папки/лог/отчёт только в Diagnostics; мастер 4 шага, `Finish` на шаге 4, удалить `wizardReady` и сводку; на Fonts оставить один Next и убрать загрузку из шага — стартовать её фоном после Finish с прогрессом/Cancel/Retry в строке Installation → Fonts; в диалоге Fonts качать только по явной кнопке; `Remove` из строки Installation убрать (остаётся в диалоге Game language).
4. **P2** — все 12 наборов шрифтов, «проверить целостность установки», перенос work dir без переустановки.


## 6. Тесты и приёмка

Unit: `openmwcfg` round-trip (нет секции, дубликаты ключей, CRLF, комментарии), pathguard
uninstall, валидация каталога шрифтов и миграция `skip_fonts`, миграция `config.json`,
полнота i18n.
GUI в CI не гоняем: логика — в `app`/`openmwcfg`, UI-тесты — на чистых функциях (`ui_test.go`).

Ручной smoke = сценарий v1: мастер (4 шага, на Fonts ничего не качается) → Finish → фоновая
загрузка шрифтов с прогрессом → UI/видео → (опционально) Game language → сервер → play →
OpenMW-CS → Diagnostics (лог/отчёт/папки) → About (язык интерфейса) → удаление установки.

## 7. Риски и открытые вопросы

- `settings.cfg` появляется только после первого запуска игры → строка «UI and video» до этого: «Launch the game once».
- OpenMW-CS требует установленных модов активного сервера (`ErrModsNotInstalled`) → строка показывает это до клика, а не ошибку после; на Linux префикс готовится так же, как перед Play.
- Смена work dir после установки = переустановка TES3MP и модов: показываем предупреждение, перенос — P2.
- Шрифты обязательны в мастере, но не блокируют игру: на шаге 3 нет ни загрузки, ни
  кнопки Install — после Finish набор качается фоном; если все три файла не скачались —
  предупреждение, игра стартует со стоковым шрифтом, повтор на следующем Play (docs §9).
- 12 наборов шрифтов = 36 ссылок и SHA256: нужен генератор/проверка; в P1 — три набора.
- Uninstall: удалять только внутри workDir/cacheDir/configDir, без прохода по симлинкам.

## 8. Что покрывает пробелы v1

| v1 | Куда попадает |
|---|---|
| `configure-ui` | Installation → UI and video (`settings.cfg`, «Launch the game once» до первого старта) |
| `install-fonts` (12 наборов) | Installation → Fonts (Install… / Change set…) + обязательный шаг 3 мастера (только выбор, загрузка после Finish) |
| `install-localization` (вопрос про озвучку) | Installation → Game language: вариант «только текст / текст + озвучка» |
| `edit-config`, `edit-client-cfg` | не возвращаем: `settings.cfg` правится через UI and video, `openmw.cfg` пересобирается на Play, `tes3mp-client-default.cfg` — из профиля сервера |
| `run-openmw-cs` | отдельный раздел OpenMW-CS |
| `uninstall` | Installation → Uninstall |
| Показ путей и папок, лог, отчёт | Diagnostics (только открытие и копирование) |
| Язык интерфейса, тема | About → Interface language; тема — переключатель в шапке |
| `interactive-download-*`, `show/download-backup-*` | не возвращаем: панель |
