# BL-017. Шапка: язык списком, тема тогглом, подпись темы не сливается

**Статус:** сделано (28.10.2026). Язык — `widget.Select` по `i18n.Supported` с
подписями `i18n.DisplayName`, тема — `widget.Check` «Тёмная тема»; обе группы
собирает `headerControl` (`internal/ui/ui.go`): подпись строкой над контролом,
`container.NewGridWrap` с прежней `headerControlWidth`, поэтому высота групп
одинаковая, а подпись не сливается со значением. `nextLanguage`, `themeModes`,
`TestNextLanguage` и `TestThemeModeLabels` удалены; ключи `theme.*` заменены на
`settings.language` и `settings.dark_theme`. `system` остался значением по
умолчанию: `style.IsDark` показывает фактический вариант (для `system` — тему
рабочего стола), первое переключение пишет явный `dark`/`light`. Документация:
`docs/v2/06-client.md` §12. Тесты: `TestHeaderHasLanguageAndThemeSwitches`,
`TestLanguageByLabel`, `TestThemeToggleLabels`, `TestIsDarkForExplicitModes`,
`TestHeaderFitsSupportedWindow` (без изменений).

**Симптом.**

- Язык интерфейса — кнопка с названием языка, на который переключит. Текущий
  язык в шапке не виден, а рядом со списком темы пара «кнопка + список»
  читается как «действие + настройка»: какой из двух контролов что меняет,
  понятно только после клика.
- Тема — список, но его подпись «Тема» стоит вплотную к значению списка, без
  зазора и тем же мелким приглушённым шрифтом: «ТемаСистемная» читается как
  одно слово. Подпись тише значения (`LowImportance`), поэтому выглядит частью
  выбранного пункта, а не названием настройки.

**Причина.**

- `headerActions` (`internal/ui/ui.go:208-210`) — `container.NewHBox` из двух
  контролов; порядок «язык, тема».
- `languageSwitch` (`internal/ui/ui.go:286-300`) — `widget.NewButton` с
  `i18n.DisplayName(target)`, цель считает `nextLanguage`
  (`internal/ui/ui.go:303-317`). Ширина фиксирована `headerControlWidth = 190`
  (`internal/ui/ui.go:215`) через `container.NewGridWrap`.
- `themeSwitch` (`internal/ui/ui.go:264-281`) — `widget.NewSelect` в правом слоте
  `container.NewBorder`, подпись `style.MutedLabel(tr.T(i18n.KeySettingsTheme))`
  в левом. `Border` не добавляет зазор между слотом и центром, а `MutedLabel`
  затемняет подпись относительно текста списка.
- Устройство закреплено тестами: `TestHeaderHasLanguageAndThemeSwitches`
  (`internal/ui/screen_test.go:371-410`) требует ровно один список (тема) и
  ровно одну кнопку (язык); `TestHeaderFitsSupportedWindow`
  (`internal/ui/screen_test.go:411-426`) — ширину шапки; `TestNextLanguage`
  (`internal/ui/ui_test.go:56-72`) и `TestThemeModeLabels`
  (`internal/ui/ui_test.go:26-54`) — поведение контролов.
- Контракт описан как «список темы и кнопка языка» в `docs/v2/06-client.md`
  §12 (`:570-575`).

**Что делать.**

1. Язык — `widget.NewSelect` по `i18n.Supported()` с подписями
   `i18n.DisplayName`, `OnChanged` → `setLanguage`; ширина та же фиксированная
   (`NewGridWrap`, `headerControlWidth`). Выбор виден всегда, добавление
   третьего языка ничего не ломает. `nextLanguage` и `TestNextLanguage`
   удаляются.
2. Тема — двухпозиционный тоггл: `widget.NewCheck` «Тёмная тема» / «Dark theme»
   (нужен новый ключ каталога в обоих языках) либо кнопка с иконкой
   `theme.VisibilityIcon`/`VisibilityOffIcon`, если подпись не влезает в шапку.
   `OnChanged` → `setTheme("dark")`/`setTheme("light")`.
3. Решить судьбу `system`: рекомендация — оставить его значением по умолчанию
   для конфигов, которые ещё не переключали (тоггл показывает фактический
   вариант через `theme.Variant()`), а первое переключение пишет явный
   `dark`/`light`. Это сохраняет поведение «как в системе» без третьего
   состояния в контроле. Если `system` нужен как выбираемый режим — вернуть
   `Select` и принять, что BL-008 и эта запись противоречат друг другу.
4. Подпись «Тема» не оставлять в `Border` вплотную: вынести её отдельной строкой
   над контролом (`NewVBox(caption, control)`), либо добавить зазор
   `style`/`theme.Padding()`. Заодно выровнять высоты: сейчас высота группы
   задаётся `selector.MinSize().Height`, а подпись слева делает строку выше.
5. Документация: `docs/v2/06-client.md` §12 (`:570-575`) — «список языка и
   тоггл темы»; в BL-008 добавить пометку о пересмотре.

**Как проверять.** Окно 900, 1000 и 1200 px на русском и английском: подпись
темы не сливается со значением, оба контрола одной высоты, ширина группы не
прыгает при смене языка. Язык меняется с любого экрана и сохраняется после
перезапуска; тоггл темы применяется на месте (прокрутка и фокус живы) и
сохраняется в `theme`; конфиг с `theme: system` показывает фактический вариант и
после первого клика получает явный `dark`/`light`. Тесты: переписать
`TestHeaderHasLanguageAndThemeSwitches` (один `*widget.Select` — язык, его
опции = `i18n.DisplayName` по `i18n.Supported()`; один `*widget.Check` или
кнопка — тема), удалить `TestNextLanguage`, `TestThemeModeLabels` оставить для
подписей режимов, `TestHeaderFitsSupportedWindow` — без изменений.
