# BL-006. Нет постоянной ссылки на последний релиз (`latest`)

**Статус:** новое (28.10.2026), не начато.

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

**Что сделать после выбора варианта.** Обновить `README.md` (§игрок, §админ) и
`docs/v2/06-client.md` на стабильные URL, добавить в runbook строку «проверить
`latest`-ссылки после релиза».

**Как проверять.** После следующего релиза:
`curl -fsSLO https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-linux-amd64.tar.gz`
— архив распаковывается, `--version` совпадает с версионным релизом, `.sha256`
сходится; установка без `TES3MP_EASY_VERSION` ставит ту же серверную версию, что
и до изменения.
