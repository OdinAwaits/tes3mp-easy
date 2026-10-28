# BL-002. Сервер печатает `[ERR]: [Script]: Could not find Lua CJSON!`

**Статус:** отложено (28.10.2026). Замер сделан: выигрыш — единицы миллисекунд
на клетку, включение меняет формат `quicksave` и стоит +54 с установки.
Решение пересматривается по триггерам из раздела «Когда вернуться».

**Симптом.** При загрузке скриптов сервер пишет в лог:

```text
[ERR]: [Script]: Could not find Lua CJSON! The decoding and encoding of JSON files will always use dkjson and be slower as a result.
```

Сервер после этого работает штатно.

**Причина.** `server/lib/lua/jsonInterface.lua` сначала проверяет модуль через
`doesModuleExist("cjson")` (`server/lib/lua/utils.lua`, обход
`package.searchers`), и только при успехе берёт `cjson`. В нашем рантайме
`cjson` не находится, поэтому ветка `else` печатает `[ERR]` и весь JSON дальше
идёт через `dkjson`. Замеренный `package.cpath` встроенного LuaJIT:
`./?.so`, `/usr/local/lib/lua/5.1/?.so`,
`/usr/lib/x86_64-linux-gnu/lua/5.1/?.so`, `/usr/local/lib/lua/5.1/loadall.so`,
`./server/lib/?.so`. Отсюда: модуль ищется в `<runtime>/server/lib/cjson.so`
(каталог `lib/` рядом с `libluajit-5.1.so.2` не сканируется), `LUA_CPATH` в
юните не нужен — достаточно `WorkingDirectory` на корень рантайма, который уже
стоит. Сервер линкуется с системным `libluajit-5.1.so.2` (`ldd`), поэтому
модуль, собранный против `libluajit-5.1-dev` дистрибутива, грузится. Рантайм
ставит `bootstrap.sh` и он root-owned, поэтому «просто положить файл руками» не
вариант: обновление TES3MP его всё равно перезапишет.

**Ловушка.** `lua-cjson` из репозитория дистрибутива ставить нельзя. В Debian 12
это 2.1.0+dfsg-2.2 в `/usr/lib/x86_64-linux-gnu/lua/5.1/cjson.so` — путь есть в
`cpath`, модуль находится, но в нём нет `encode_empty_table_as_object` и
`decode_null_as_lightuserdata` (в форке TES3MP 2.1.0.6 они есть). Проверено:
`jsonInterface.lua:9` падает, сервер завершается с `std::runtime_error` (exit
134) вместо работы на `dkjson`.

**Кто использует JSON.** Все чтения/записи мира идут через один интерфейс:
`server/scripts/cell/json.lua:49` (загрузка клетки), `player/json.lua:50`,
`recordstore/json.lua:44`, `world/json.lua:26-27`, `serverCore.lua:75,124`
(`banlist.json`). Загрузка клетки: `eventHandler.lua:1261` →
`logicHandler.LoadCell` → `cell/json.lua:49`. `cjson` подхватывают только
`load` и `quicksave`; `save` (человекочитаемые сейвы с `indent` и `keyorder`)
всегда идёт через `dkjson`.

**Замер (28.10.2026, Debian 12, 1 vCPU, TES3MP 0.8.1, штатный
`tes3mp-server.x86_64`).** Нагрузка — синтетическая клетка по форме реальной
(в релизе `server/data` пустая), `os.clock()`, 200 прогонов для 100 КБ и 40 для
476 КБ:

| payload | dkjson decode | cjson decode | dkjson encode | cjson encode |
| --- | --- | --- | --- | --- |
| 100 КБ, 300 объектов | 15.0 мс | 2.2 мс | 9.3 мс | 2.5 мс |
| 476 КБ, 1400 объектов | 73.8 мс | 11.3 мс | 44.4 мс | 13.0 мс |

То есть 3.4–6.9×. В наших величинах (244 клетки, 2.3 МБ, самая большая 68 КБ):
самая большая клетка ~10 мс → ~1.5 мс, полный проход мира ~350 мс → ~52 мс.
Обе величины тонут на фоне I/O, Lua-логики и сети; выигрыш сейчас — чище лог.

Стоимость включения (там же): `apt-get install gcc libc6-dev libluajit-5.1-dev`
— 35 с, 24 пакета, +165 МиБ пика; сборка — 1.3 с, 40 КБ; `apt-get purge`
разницы по снапшоту `dpkg-query` — 18.5 с, диск возвращается (8077 → 7912 →
8076 МиБ). Итого +54 с на установку и полдня на код, тесты и доки.

| Вариант | Плюсы | Минусы |
| --- | --- | --- |
| Собирать `lua-cjson` в `bootstrap.sh` и класть в `<runtime>/server/lib/cjson.so` | убирает `[ERR]`, 3.4–6.9× на `load`/`quicksave` | +54 с и +165 МиБ пика к установке, компилятор на время сборки, формат `quicksave` меняется (компактно, без `keyorder`), нужен чек в `ops doctor` |
| `lua-cjson` из дистрибутива | одна команда | сервер не стартует (см. «Ловушка») |
| Гасить сообщение патчем `jsonInterface.lua` при установке | одна строка, без зависимостей | патчим чужой рантайм при каждом обновлении; скорость не меняется |
| Ничего не делать | ноль работы и риска | `[ERR]` в логе путает оператора и маскирует настоящие ошибки скриптов |

**Рецепт, если браться (проверен 28.10.2026: модуль загрузился, `[ERR]`
исчез, после удаления сборочных пакетов модуль продолжает грузиться).**

```bash
apt-get install -y --no-install-recommends gcc libc6-dev libluajit-5.1-dev
curl -fsSL -o lua-cjson.tar.gz \
  https://codeload.github.com/TES3MP/lua-cjson/tar.gz/c3abdb769ffd88c74d8c421d49af9f5d2c8cf845
# sha256 архива: bb99ec90aac3e15d9f43031248e511a0af02c051031e76ddb0bdaa5a7c33a69a
tar xzf lua-cjson.tar.gz && cd lua-cjson-*
gcc -O2 -fPIC -shared -DNDEBUG -I/usr/include/luajit-2.1 \
  -o <staging>/server/lib/cjson.so lua_cjson.c fpconv.c strbuf.c
```

- Источник — форк TES3MP, коммит `c3abdb7`, версия 2.1.0.6; `make` не нужен,
  модуль собирается из трёх `.c`. Проверено: `cjson._VERSION = "2.1.0.6"`.
- Уборка зависимостей — снапшот `dpkg-query -W -f='${binary:Package}\n'` до
  установки, после сборки `apt-get purge` ровно добавленной разницы. Без
  `--auto-remove`: он сносит чужие auto-пакеты. Ошибку сборки и уборки
  показывать как `warn`, установку не прерывать.
- Поведение при включении: `jsonInterface.lua` выставляет
  `encode_sparse_array(true)`, `encode_invalid_numbers("null")`,
  `encode_empty_table_as_object(false)`, `decode_null_as_lightuserdata(false)`;
  `quicksave` пишет компактно и без `keyorder`, `null` при загрузке становится
  `nil`. Переход обратно на `dkjson` вернёт `indent`+`keyorder`.
- Модуль живёт в root-owned рантайме: после переустановки/обновления TES3MP его
  нужно собрать заново, поэтому в `ops doctor` нужен чек «модуль есть / нет».

**Когда вернуться.** Замер на реальном мире и игроках покажет десятки
миллисекунд на переход между клетками или на сохранение; мы начнём собирать
рантайм сами; появится сценарий 50+ игроков; понадобится совместимость с
`null`, как в Windows-сборке. Во всех случаях рецепт выше уже проверен.

**Решение, если браться.** Только сборка `lua-cjson` под встроенный LuaJIT в
`bootstrap.sh` плюс проверка в `ops doctor` («модуль есть / модуля нет»).
Патч `jsonInterface.lua` и «положить .so руками» — не решение.
