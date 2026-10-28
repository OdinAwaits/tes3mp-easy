# BL-010. Installation → Advanced: пустая строка «Профиль клиента»

**Статус:** сделано (28.10.2026). Аккордеон `profile` и ключи
`install.client_profile`, `install.automatic`, `install.profile_auto` удалены;
в Advanced остались work dir и Proton (Linux). Защита —
`TestInstallationAdvancedKeepsOnlyPathAccordions`
(`internal/ui/screen_test.go`), `docs/v2/06-client.md` обновлён.

**Симптом.** В «Установка → Дополнительно» третий аккордеон «Профиль клиента»
раскрывается в бейдж «Автоматически» и абзац о том, что профиль собирается
сам. Ни поля, ни кнопки, ни ссылки там нет: пункт ничего не настраивает и
только удлиняет список рядом с «Рабочая папка» и «Proton (Linux)».
Пользователь ищет в «Дополнительно» действия, а получает справку.

**Причина.** `installationAdvanced` добавляет аккордеон вручную:
`profile := container.NewBorder(…, aboutBadge(tr.T(
i18n.KeyInstallAutomatic), …), …)`
и `widget.NewAccordionItem(tr.T(i18n.KeyInstallClientProfile), …)` —
`internal/ui/screen_install.go:360-367`. Ключи `install.client_profile`,
`install.automatic`, `install.profile_auto` (`internal/i18n/catalog.go:83-85`,
`407-409`, `727-729`) используются только здесь. Документация это место тоже
перечисляет: «…work dir, Proton, папки, строка профиля AUTOMATIC»
(`docs/v2/06-client.md:546-547`).

**Что делать.**

- Удалить аккордеон `profile` и три ключа из обоих каталогов: мёртвые строки
  копятся, а `TestCatalogsHaveEveryKey` следит только за паритетом.
- Если факт про автоматический профиль нужен, оставить одну muted-строку под
  аккордеонами Advanced без раскрывающегося пункта: справка не должна
  выглядеть как настройка.
- Обновить `docs/v2/06-client.md:546-547` и упоминание Advanced в §12.
- После удаления в Advanced остаются только реальные действия: work dir и
  Proton на Linux.

**Как проверять.** «Установка → Дополнительно»: нет пункта «Профиль клиента»,
на Linux два аккордеона, на остальных платформах один;
`go test ./internal/i18n ./internal/ui ./test/docs/...`.
