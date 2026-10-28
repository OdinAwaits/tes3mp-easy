# BL-006. Нет постоянной ссылки на последний релиз (`latest`)

**Статус:** сделано (28.10.2026), вариант A. Проверка на живом релизе —
после следующего тега.

**Симптом.** README и внешние ссылки (форум, Discord, вики) не могут дать
постоянный адрес на последний клиент: имена ассетов содержат версию
(`tes3mp-easy-client-0.2.6-linux-amd64.tar.gz`), а `/releases/latest` в GitHub
указывает на последний релиз любого канала, поэтому после серверного релиза он
ведёт на серверный архив. `install.sh` поэтому уже сознательно обходит
`/releases/latest` (`install.sh:14-19`).

**Вариант A (рекомендуется): движущиеся теги канала + ассеты со стабильными
именами.** В `publish` (`softprops/action-gh-release@v2`,
`.github/workflows/release.yml:185`) дополнительно:
- `git tag -f client-latest <sha> && git push -f origin client-latest` (и
  `server-latest` для серверного канала) — тег переезжает на каждый релиз;
- публиковать рядом копии ассетов с именами без версии
  `tes3mp-easy-client-linux-amd64.tar.gz`,
  `tes3mp-easy-client-windows-amd64.zip`, `tes3mp-easy-linux-amd64.tar.gz` и их
  `.sha256` (sha считать уже после переименования: `sha256sum` пишет имя файла
  внутрь);
- у alias-релиза `make_latest: false`, чтобы бейдж `Latest` остался у
  версионного релиза и порядок каналов не менялся.

Тогда стабильные адреса:
`https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-windows-amd64.zip`
и `.../download/server-latest/tes3mp-easy-linux-amd64.tar.gz`.

**Вариант B (без движущихся тегов):** те же alias-ассеты кладутся в каждый
версионный релиз. Ссылка «на последний» всё равно требует ручного обновления
README — годится только как промежуточный шаг.

**Ограничения.**
- `install.sh` ищет `tes3mp-easy-[0-9]…-linux-amd64.tar.gz` (`install.sh:19`) и
  `TES3MP_EASY_VERSION` не трогать: alias-имена без цифры под шаблон не
  попадают, поэтому выбор серверной версии не изменится — это нужно проверить,
  а не предполагать.
- Проверка контрольных сумм в README опирается на `.sha256` рядом с файлом;
  для alias их надо генерировать заново, а не копировать версионные.
- Права и триггеры: `git push -f` тегов из Actions требует `contents: write`
  (уже есть у job `publish`) и не создаёт цикл, пока новые теги не попадают в
  `on: push: tags` (`server-v*`/`client-v*`); `*-latest` в фильтр добавлять
  нельзя.

**Что сделано.** В job `publish` добавлены три шага:

1. `Prepare channel alias` — `case` по каналу собирает версионные ассеты
   (`tes3mp-easy-<v>-linux-amd64.tar.gz` для server,
   `tes3mp-easy-client-<v>-{linux-amd64.tar.gz,windows-amd64.zip}` для client),
   делает копии с вырезанной версией (`${asset/-${version}-/-}`) в `alias/` и
   пересчитывает `.sha256` уже для новых имён; там же пишется `alias-notes.md`
   с составом и ссылкой на версионный релиз.
2. `Move channel tag` — `gh api PATCH /git/refs/tags/<channel>-latest` с
   `force`, при отсутствии тега — `POST /git/refs`. Тег двигается через API, а
   не `git push`, поэтому не нужны ни локальный клон, ни изменение прав
   (`contents: write` у job уже был); фильтр `on: push: tags` содержит только
   `server-v*`/`client-v*`, так что циклов нет.
3. Второй `softprops/action-gh-release@v2` с `tag_name: <channel>-latest`,
   `body_path: alias-notes.md`, `files: alias/*`, `make_latest: false`,
   `overwrite_files: true` — бейдж `Latest` остаётся у версионного релиза.

Механика проверена локально: шаг вырезан из YAML и прогнан на поддельном
`dist/` для обоих каналов (имена и `sha256sum -c` сходятся). `install.sh` не
изменён: его `SERVER_ASSET` требует цифру сразу после `tes3mp-easy-`, поэтому
алиасные имена не участвуют в выборе версии; `test/install-smoke.sh` теперь
держит в списке релизов `server-latest` и `client-latest` и по-прежнему
выбирает `tes3mp-easy-9.9.9-linux-amd64.tar.gz`. Инварианты (движущийся тег,
`make_latest: false`, стабильные URL в README, `[0-9]` в `SERVER_ASSET`)
закреплены `TestStableReleaseLinks` (`test/docs/release_test.go`).

**Документация.** `README.md` (§игрок — ссылки через `client-latest`, §админ —
`server-latest` и оговорка, что версия видна только внутри архива),
`docs/v2/06-client.md` §2, `docs/v2/08-runbook.md` §2.1 и §17 (каналы с
движущимися тегами и проверка алиасов после релиза), `docs/v2/07-testing.md`
§10.

**Ограничения, которые остались.**
- `install.sh` по-прежнему не использует `/releases/latest` и `server-latest`,
  а сканирует список релизов: при смене формата алиаса установщик не сломается.
- Ручной `workflow_dispatch` алиасы не обновляет — `publish` требует
  `refs/tags/`, поэтому перевыпуск делается re-run запуска по тегу.
- Проверка `curl -fsSLO .../client-latest/...` из раздела «Как проверять»
  возможна только после следующего релиза: в текущем дереве алиасных ассетов
  ещё нет.

**Как проверять.** После следующего релиза:
`curl -fsSLO https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-linux-amd64.tar.gz`
— архив распаковывается, `--version` совпадает с версионным релизом, `.sha256`
сходится; установка без `TES3MP_EASY_VERSION` ставит ту же серверную версию, что
и до изменения.
