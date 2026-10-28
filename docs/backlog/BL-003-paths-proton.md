# BL-003. Пути и Proton: проводник вместо ручного ввода, эффективный Proton

**Статус:** сделано (28.10.2026). Обе строки «Дополнительно» открывают
системный выбор папки: `chooseFolder`/`pathButtons` (`internal/ui/folders.go`)
стартуют с текущего каталога, «Ввести путь…» остался запасным действием, мастер
использует тот же выбор. `app.Service.ProtonStatus` отдаёт эффективный Proton
(`proton_path`, иначе `launcher.FindProton`) вместе с источником; строка
подписывает источник или просит выбрать папку. Ошибки поиска и проверки Proton
обёрнуты в `launcher.ErrProton` (`FindProton`/`ProtonFolder`), поэтому запуск
показывает `T3E-GAME-02` и кнопку «Установка». Тесты: `TestProtonStatus*`,
`TestProtonErrorsAreClassified`, `TestInstallationAdvancedEditsPathsWithPicker`,
`TestPathButtonsOfferPickerAndTypedEntry`, `TestFolderURI`,
`TestProtonHintNamesTheSource`; в `failure_test` добавлен случай «missing Proton».

**Симптом.**

- В «Установке» кнопки смены пути («Изменить» у рабочей папки и «Обзор» у
  Proton) открывают диалог с текстовым полем: путь нужно печатать руками. Окно
  выбора каталога есть только в мастере первого запуска для Morrowind, хотя
  `v2/06-client.md` (§2) обещает нативный файловый диалог.
- В «Установка → Дополнительно → Proton (Linux)» вместо пути стоит «—», даже
  когда Proton найден автоматически через Steam. Пользователь не видит, какой
  Proton будет использован.

**Причина.**

- `internal/ui/screen_install.go:388` (`changePath`) собирает `widget.NewEntry`
  и `dialog.NewCustomConfirm`. Единственный вызов `dialog.NewFolderOpen` во всём
  клиенте — `internal/ui/wizard.go:486` (`chooseMorrowind`).
- Строка Proton печатает только `cfg.ProtonPath`
  (`internal/ui/screen_install.go:354`), то есть значение из `config.json`; при
  пустом значении `pathLabel` показывает «—». Реальный Proton вычисляется лишь
  при запуске: `launcher.Resolve` вызывает `DiscoverProton` (Steam-библиотеки и
  `compatibilitytools.d`, `internal/launcher/discover.go:14-60`) и кладёт путь в
  `Environment.ProtonDir` (`internal/launcher/paths.go:179-191`), в UI это не
  попадает.
- Ошибка автопоиска Proton возвращается без `launcher.ErrProton`
  (`internal/launcher/discover.go:47-51`, `internal/launcher/paths.go:179-183`),
  поэтому классифицируется как `FailureUnknown` (`T3E-APP-99`) с английским
  текстом в деталях, хотя `ErrProton` (`internal/launcher/launch.go:103-108`)
  уже используется для неисполняемого `proton`.

**Что делать.**

1. Одна функция `chooseFolder(title, current, apply)` с `dialog.NewFolderOpen`
   (стартовая папка — текущее значение, если каталог существует), а «ввести путь
   вручную» — вторым действием для окружений, где диалог не открывается (Wayland
   без xdg-desktop-portal, удалённый X11). Применить в `changePath` (рабочая
   папка, Proton) и переиспользовать в мастере (`chooseMorrowind`), чтобы
   поведение везде было одинаковым.
2. Отдавать из `app.Service` «эффективный Proton»: `cfg.ProtonPath`, иначе
   `DiscoverProton`, плюс признак источника (вручную / найдено автоматически).
   Считать его лениво, при открытии «Дополнительно», чтобы не сканировать Steam
   на старте.
3. Показывать в строке путь моноширинно (`pathLabel`) и подпись источника;
   пустой путь — «—» с подсказкой «Proton не найден, выберите папку». Кнопку
   вести в `chooseFolder`.
4. Обернуть ошибки поиска и валидации Proton в `launcher.ErrProton`, чтобы
   пользователь видел `T3E-GAME-02` и действие «Выберите рабочую папку Proton»,
   а не `T3E-APP-99`.

**Как проверять.** Linux (X11 и Wayland) и Windows: «Изменить»/«Обзор»
открывают окно выбора каталога, выбранный путь сохраняется в `config.json`,
ручной ввод остаётся доступен. Linux со Steam и без `proton_path` в конфиге: в
строке путь обнаруженного Proton и «найдено автоматически»; после ручного
выбора — «задано вручную», этот же путь в `logs/client.log`. Linux без Proton:
подсказка вместо «—» и `T3E-GAME-02` при запуске; Windows — строка скрыта
(`runtime.GOOS == "linux"`). Тесты: драйвер Fyne (`testShell`,
`internal/ui/screen_test.go:18`) — после клика не появляется видимый
`*widget.Entry`; `internal/launcher` (DiscoverProton и обёртка `ErrProton`);
`internal/app/failure_test.go` (маппинг).
