# BL-012. Шрифты: десять наборов, Select, превью и один шаг

**Статус:** сделано (28.10.2026). Включает BL-016 («Каталог шрифтов: 10 наборов
и Select вместо радио»). Десять наборов с закреплёнными URL и SHA256 —
`internal/fonts/sets.go`; файлы объявлены shared-переменными, поэтому общий
`NotoSerif.ttf` не расходится между наборами. Названия — `fonts.Set.Name`, в
i18n остался только суффикс «(по умолчанию)». В мастере три шага: «Шрифты и
интерфейс» = `Select` + превью + поля `settings.cfg`. Превью —
`internal/ui/font_preview.go` (установленный TTF, иначе шрифт темы), тот же
виджет в **Installation → Fonts → Change set**. `cache/fonts/` после установки
чистится от файлов других наборов (`pruneFontCache`). Все десять наборов
проверены живой установкой 28.10.2026. Защита: `TestCatalogIsComplete`
(10 наборов, порядок, дедуп внутри набора), `TestInstallPrunesFontCacheOfOtherSets`,
`TestFontSetChooserMapsLabelsToIDs`, `TestFontPreviewFollowsTheInstalledFile`,
`TestWizardStepsAreTranslatedAndOrdered`, `go test ./...`. Документация:
`docs/v2/06-client.md` §3/§9/§12, `docs/v2/03-contracts.md`, `docs/v2/07-testing.md`.

**Симптом.**

- Мастер идёт четырьмя шагами: Morrowind, TES3MP, шрифты, UI and video. Шаг
  шрифтов — три радиокнопки, шаг интерфейса — четыре числовых поля; оба про
  внешний вид и оба необязательны, но разнесены и требуют лишнего «Далее».
- В v1 `install-fonts` предлагал 11 наборов sans+serif+mono (пункты 1–11 плюс
  ссылка на Nexus), а в v2 осталось три: Noto, Inter, IBM Plex. Хочется вернуть
  десять вариантов, но список из десяти радио-кнопок в текущей вёрстке не
  помещается.
- Набор выбирается вслепую: видно только название (Noto/Inter/Plex), а как
  выглядит игровой текст — непонятно, хотя именно размытие и «квадратики» и
  завели шрифты в мастер (`docs/v2/06-client.md:390-428`).

**Причина.**

- Шаги и их число заданы enum-ом `wizardMorrowind … wizardUIVideo` и
  `wizardStepCount` (`internal/ui/wizard.go:18-26`); заголовки —
  `wizardTitleKey` (`:121-136`), тела — `stepObjects` (`:147-161`). Переходы —
  `advance` (`:340-355`) и `previous` (`:358-370`); «Готово» включается по
  `w.step == wizardUIVideo` (`:293-297`); сохранение `settings.cfg` —
  `saveUIVideo` (`:273-287`). Шаг шрифтов (`fontsObjects`, `:188-202`) — подпись
  и `RadioGroup` из `newFontSetChooser` (`internal/ui/dialog_fonts.go:33-58`), без
  образца; шаг UI/video (`uiVideoObjects`, `:240-252`) использует
  `newUIVideoForm` — ту же форму, что диалог Installation
  (`internal/ui/dialog_uivideo.go:41-46`). Тест фиксирует список шагов
  (`internal/ui/ui_test.go:488-494`), а документация — четыре шага
  (`docs/v2/06-client.md:52-58`, `:424`, `:554-557`).
- Каталог жёстко задан тремя наборами: `internal/fonts/sets.go:14-23`
  (идентификаторы и `DefaultSet`), `:67-71` (`sets`), `:75-99` (`Sets`,
  `SetIDs`). Тест закрепляет «ровно 3» (`internal/fonts/sets_test.go:14-20`), а
  также требует уникальные имена файлов между наборами (`:38-41`), хотя
  `NotoSerif.ttf` в v1 использовался тремя наборами.
- В v1 было 11 троек (коммит `05c9b6a`, файл `client/bin/player/install-fonts`):
  IBM Plex; Noto Sans + Noto Serif + Roboto Mono; Roboto + Lora + Cousine;
  Open Sans + Noto Serif + Source Code Pro; Montserrat + Playfair Display + Space
  Mono; Fira Sans + Lora + Fira Mono; Arimo + Noto Serif + Red Hat Mono;
  Source Sans 3 + Playfair Display + Roboto Mono; Noto Sans + Lora + Cousine;
  Roboto + Playfair Display + Source Code Pro; Open Sans + Noto Serif + Space
  Mono. Скрипт качал их без SHA256 и без проверки покрытия кириллицы.
- UI — `widget.RadioGroup` в `fontSetChooser` (`internal/ui/dialog_fonts.go:30-70`),
  используется и в мастере (`internal/ui/wizard.go:188-204`), и в диалоге
  Installation (`internal/ui/dialog_fonts.go:88-120`). Диалог фиксирован на
  560×380 (`:124`); одиннадцать строк по ~36 px не влезут вместе с подсказкой и
  строкой ошибки, а радио не даёт компактно показать текущий выбор.
- Тест связывает каталог с локализацией: каждый идентификатор из
  `fonts.SetIDs()` обязан иметь ключ в `fontSetLabelKeys`
  (`internal/ui/ui_test.go:515-540`). Имена наборов — три ключа
  `KeyFontsSetNoto/Inter/Plex` (`internal/i18n/catalog.go:59-61`, `:383-385`,
  `:703-705`); на десять наборов это двадцать строк каталога ради названий
  семейств, которые не переводятся.
- Документация прямо фиксирует три набора: «выбирается из трёх готовых
  вариантов, а не из 12» (`docs/v2/06-client.md:392-395`), список из трёх
  (`:396-400`) и «Change set (три набора)» (`:541`). Расширение каталога —
  изменение контракта, а не только кода.

**Что делать.**

1. Каталог: десять троек из списка v1, все с покрытием латиницы и кириллицы
   (`1025 1105`, `192 382` из `internal/fonts/config.go:50-58`). Практично:
   оставить проверенные Noto/Inter/Plex и добавить Roboto+Lora+Cousine, Open
   Sans+Noto Serif+Source Code Pro, Montserrat+Playfair+Space Mono, Fira
   Sans+Lora+Fira Mono, Arimo+Noto Serif+Red Hat Mono, Source Sans 3+Playfair+
   Roboto Mono, Noto Sans+Lora+Cousine — всего десять. Для каждого файла —
   закреплённые HTTPS URL в `google/fonts` (OFL) и SHA256; каждый набор — те же
   три слота (sans/serif/mono).
2. Проверять покрытие до фиксации: `golang.org/x/image/font/sfnt`
   (`go.mod:45`, сейчас indirect) читает cmap TTF, тест требует U+0400–U+045F и
   U+2116 для каждого файла; иначе «квадратики» вернутся вместе с новым набором.
   Альтернатива без новой зависимости — разовый скрипт и запись результата в
   комментарий рядом с набором.
3. Разрешить общие файлы между наборами: в `sets_test.go:38-41` сравнивать не
   только имя, но и SHA256 (одинаковый файл — не конфликт). `removeStaleFonts`
   (`internal/fonts/fonts.go:317-338`) уже удаляет только имена, которых нет в
   текущем наборе, и дедуплицирует через `ManagedAssetNames`, так что общий
   `NotoSerif.ttf` переживёт переключение безопасно. `cache/fonts/` чистится
   после успешной установки (`pruneFontCache`): файлы установленного набора
   остаются, файлы других наборов удаляются, иначе при десяти наборах каталог
   распухнет на десятки мегабайт.
4. Заменить радио на `widget.NewSelect(labels, onChanged)`
   (`fyne.io/fyne/v2/widget/select.go:49`): те же параллельные `ids`/`labels`,
   `selected()` берёт поле `Selected`, `selectID` — `SetSelected`. `Select`
   занимает одну строку формы, меню рисуется поверх окна (`NewPopUpMenu`,
   `select.go:293-296`), высота диалога больше не зависит от числа наборов;
   десять пунктов по ~36 px — около 360 px, это помещается в окно 560×380.
   `OnChanged` прежний, сохранение набора по-прежнему сразу
   (`wizard.selectFontSet`, `:205-216`). Поправить
   `TestFontSetChooserMapsLabelsToIDs` (`internal/ui/ui_test.go:511-525`): поле
   `group` переименовать в `select`, обновить комментарии структуры.
5. Названия наборов держать в Go, а не в i18n: добавить `Set.Name`
   (`internal/fonts/sets.go:9-12`, например «Roboto · Lora · Cousine») и убрать
   `KeyFontsSetNoto/Inter/Plex` из `fontSetLabelKeys`; пометку «(по умолчанию)»
   собирать из нового `KeyFontsSetDefault` (`internal/i18n/catalog.go:57-61`).
   Тесту `ui_test.go:531-537` достаточно проверять непустое имя у каждого набора.
6. Слить шаги: `wizardFonts` становится «Шрифты и интерфейс», `wizardUIVideo`
   удаляется, `wizardStepCount` = 3. В теле — `Select` набора, превью и четыре
   поля `uiVideoForm` под подзаголовком. `advance` на последнем шаге зовёт
   `saveUIVideo()` и `finishWizard()`; под новый шаг переписать `canAdvance`,
   `updateButtons`, `enterStep`, `previous` и `wizardTitleKey`. Набор
   сохраняется сразу при выборе. Каталог: `wizard.uivideo.title`/`.hint`
   заменить на `wizard.fonts.title` = «Шрифты и интерфейс» и один общий hint;
   убрать дублирующие подписи; правки в обоих каталогах.
7. Превью: `canvas.NewText(sample, …)` с полем `FontSource`
   (`fyne.io/fyne/v2@v2.8.2/canvas/text.go:25-30`) — единственный способ
   отрисовать произвольный TTF в Fyne. После установки файлы набора лежат в
   `fonts.State.FontsDir` (`internal/fonts/fonts.go:88-96`, каталог
   `.../My Games/OpenMW/Fonts`); читать `os.ReadFile` +
   `fyne.NewStaticResource(name, data)`. До установки файлов нет (в бинарь они не
   встроены, `//go:embed` только у панели), поэтому превью рисуется шрифтом темы
   с пометкой «образец появится после установки». Альтернатива — вшить по одному
   small-subset TTF на набор (OFL позволяет), несколько сотен КБ в бинаре;
   решать по размеру. Образец — новая строка каталога с латиницей, кириллицей и
   цифрами: «Съешь ещё этих мягких французских булок, да выпей чаю — 0123». Тот
   же виджет переиспользовать в «Установка → Шрифты → Change set»
   (`internal/ui/dialog_fonts.go:88-120`).
8. Документация: §3 (три шага), §9 (десять наборов, их состав, принцип «три
   слота» и покрытие кириллицы; набор выбирается на шаге 3, есть превью), §12
   («Change set (десять наборов)»); `docs/v2/06-client.md:52-58`, `:392-400`,
   `:541`, `:554-557`.

**Как проверять.** Мастер: шагов три, на третьем видны `Select` из десяти
наборов, образец и четыре поля; «Назад» ведёт на TES3MP, «Готово» пишет
`settings.cfg` и запускает загрузку шрифтов. Превью меняется при переключении
набора; после установки показывает реальный TTF, до неё — шрифт темы и пометку.
Installation → Fonts → Change set: выпадающий список из десяти пунктов, текущий
набор выбран; после «Применить и установить» скачиваются три файла нового
набора, старые удаляются, `openmw_font.xml` и `fallback=Fonts_Font_0..2`
указывают на новые имена, текст в игре без «квадратиков». Тесты:
`go test ./internal/fonts ./internal/ui ./test/docs/...`, `go build ./...`;
`sets_test.go` — на десять элементов и первый `DefaultSet`; `TestCatalogIsComplete`
сохраняет проверки SHA256/HTTPS; `TestCatalogsHaveEveryKey` — новый ключ
`fonts.set.default` в обоих каталогах.

