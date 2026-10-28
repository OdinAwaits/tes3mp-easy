# BL-009. В About просачиваются `%!(EXTRA string=…)`

**Статус:** сделано (28.10.2026). В оба ключа обоих каталогов добавлен `%s`,
защита — `TestScreensHaveNoFormatArtifacts` (`internal/ui/screen_test.go`):
обходит пять экранов в EN и RU и падает на подстроке `%!`.

**Симптом.** На экране «О клиенте» вместо версии видно
`Версия%!(EXTRA string=0.2.6)` и `TES3MP%!(EXTRA string=0.8.1)`; в английском
интерфейсе — `Version%!(EXTRA string=0.2.6)`. Строки длинные, ломают карточку
и выглядят как отладочный мусор. Подвал окна при этом печатает обе версии
правильно.

**Причина.** `aboutScreen` форматирует два ключа, у которых нет глагола:

- `style.Heading(tr.Tf(i18n.KeyAboutVersion, app.Version))` —
  `internal/ui/screen_about.go:54`;
- `aboutLine(tr.Tf(i18n.KeyAboutTES3MP, app.TES3MPVersion), …)` —
  `internal/ui/screen_about.go:73`.

В каталоге `about.version` = «Version»/«Версия», `about.components.tes3mp` =
«TES3MP» — без `%s` (`internal/i18n/catalog.go:623,633,943,953`), а `Tf` — это
просто `fmt.Sprintf(t.T(key), args...)` (`internal/i18n/i18n.go:128-130`).
Лишний аргумент `Sprintf` печатает как `%!(EXTRA string=…)`.

`TestCatalogsHaveEveryKey` (`internal/i18n/i18n_test.go:94-109`) проверяет
только наличие ключа, но не совпадение глаголов и аргументов; UI-тестов на
About нет. Сверка всех вызовов `Tf` по коду (28.10.2026) показала: эти два —
единственные с расхождением. `footer.version` с двумя `%s`
(`catalog.go:644,964`) поэтому и работает.

**Что делать.**

- Добавить глагол в оба ключа обоих каталогов: «Version %s»/«Версия %s»,
  «TES3MP %s» (EN `catalog.go:623,633`, RU `:943,953`). Ключи уже форматные,
  вёрстка не меняется.
- Защита от повтора: тест в `internal/ui`, который обходит объекты экранов
  (`playScreen`, `installScreen`, `aboutScreen`, `editorScreen`,
  `diagnosticsScreen`) через драйвер `testShell` и падает, если в
  `widget.Label`, `widget.RichText` или `widget.Button` есть подстрока `%!`.
  Обход понадобился, чтобы поймать этот баг; в репозитории его нет.
- При добавлении новых форматных ключей сверять число глаголов с числом
  аргументов: сейчас это видно только на экране.

**Как проверять.** About в EN и RU: «Версия 0.2.6», «TES3MP 0.8.1» без
`%!(EXTRA…)`; `go test ./internal/i18n ./internal/ui`; новый тест падает,
если вернуть старые строки.
