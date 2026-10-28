# BL-018. Выбор папки всё ещё не системный диалог

**Статус:** сделано (28.10.2026). `chooseFolder` (`internal/ui/folders.go`)
вызывает пакетную `pickFolder folderPicker`: `folders_picker_linux.go` —
`filechooser.OpenFile` портала (`github.com/rymdport/portal` стала прямой
зависимостью), `folders_picker_windows.go` — диалог оболочки
`SHBrowseForFolderW` с `BIF_USENEWDIALOGSTYLE` и стартовой папкой через
`SHParseDisplayName`, `folders_picker_other.go` — «нет нативного диалога».
Отмена (`ok == false`) ничего не меняет, ошибка или отсутствие портала
открывает прежний Fyne-список (`showFyneFolderPicker`), «Enter path…» остаётся
вторым действием. На Windows выбран диалог оболочки, а не COM `IFileOpenDialog`:
он системный, но не требует ручных обёрток vtable и `unsafe`-вызовов, которые
нечем проверить без машины с Windows. Тесты: `TestPathPickerUsesNativeDialog`,
`TestPathPickerCancelChangesNothing`, `TestPathPickerFallsBackToFyneDialog`,
`TestFolderURI`. Документация: `docs/v2/06-client.md` §6, §12.

**Симптом.** «Change…»/«Browse…» в Installation → Advanced и «Choose» в первом
шаге мастера открывают окно Fyne: свой список, стрелка вверх, поле имени, без
боковых мест, избранного, сетевых и облачных путей, другого вида и шрифта, чем
у остальных окон системы. На Windows и на Linux это выглядит как диалог из
чужого приложения.

**Причина.**

- `chooseFolder` (`internal/ui/folders.go:16-31`) вызывает
  `dialog.NewFolderOpen` — это виджет Fyne поверх `widget.List`, а не GTK/KDE и
  не Win32. Начальная папка задаётся `folderURI` (`:38-49`).
- Тест закрепляет именно Fyne-список: `TestPathPickerOpensFolderDialog`
  (`internal/ui/screen_test.go:248-277`) требует `*widget.List` в оверлее и
  запрещает текстовые поля; `TestFolderURI` (`:320`) проверяет только стартовую
  папку.
- `pathButtons` (`internal/ui/folders.go:58-70`) и `enterPath` (`:71-87`)
  добавляют ручной ввод как запас — этот путь остаётся нужным и после перехода
  на системный диалог.
- Документация уже описывает системный диалог: `docs/v2/06-client.md:305`
  («выбор идёт через системный диалог») и `:628` («открывают системный выбор
  папки»).

**Что делать.**

1. Linux: `filechooser.OpenFile(parent, title, &filechooser.OpenFileOptions{
   Directory: true, CurrentFolder: current})` из `github.com/rymdport/portal`
   (уже в `go.mod` транзитивно через Fyne, версия v0.4.2; станет прямой
   зависимостью). Портал `xdg-desktop-portal` показывает GTK- или KDE-диалог.
2. Windows: COM `IFileOpenDialog` с `FOS_PICKFOLDERS` через
   `golang.org/x/sys/windows` (уже прямая зависимость) в отдельном
   `folders_windows.go`; `SHBrowseForFolder` — упрощённый запас. CGO-обёртки
   вроде `sqweek/dialog` не тянуть: сборка клиента уже CGO, но лишняя
   зависимость ради одного вызова не нужна.
3. Контракт: `chooseFolder` остаётся единственной точкой входа, реализации —
   `folders_linux.go` / `folders_windows.go` с сигнатурой
   `(path string, ok bool, err error)`;
   `ok == false` — отмена (ничего не меняем), `err` или недоступность портала —
   прежний Fyne-диалог как запас (Wayland без портала, удалённый X11, сборка без
   D-Bus). Ручной ввод остаётся вторым действием.
4. Тесты: системный вызов в CI не воспроизвести, поэтому инъекция функции в
   `shell` (пакетная переменная) и проверки ветвей: успех, отмена, ошибка
   портала → открывается Fyne-диалог. `TestPathPickerOpensFolderDialog`
   переписать на «вызывается выбранная реализация», а Fyne-диалог проверять
   только в ветке запаса.
5. Документация: `docs/v2/06-client.md:305`, `:628` — уточнить, что список Fyne
   остался только запасным путём.

**Как проверять.** Windows 10/11 — «Change…» открывает проводник с выбором
папки; Linux GNOME и KDE (портал установлен) — системный диалог; Linux без
портала — Fyne-диалог и «Enter path…»; выбор папки Morrowind в мастере и work
folder в Installation → Advanced; отмена не меняет ни путь, ни `config.json`;
выбранная папка открывается в проводнике/файловом менеджере кнопкой «Open
folder».
