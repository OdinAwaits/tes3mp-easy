> **Reference.** Это справка по ограничениям TES3MP 0.8.1, а не описание
> архитектуры проекта. Актуальные контракты архивов — в
> [`docs/v2/03-contracts.md`](../v2/03-contracts.md) §6.

# Modding TES3MP 0.8.1 — Limitations and Capabilities

This document describes what is supported and **not** supported in TES3MP 0.8.1 (based on OpenMW 0.47).

## Supported

### Plugins (`.esp`/`.esm`/`.omwaddon`/`.omwscripts`/`.omwgame`)

Plugins modify the game on the client side. They can contain:

- **Game data** (items, spells, NPCs, worlds, etc.)
- **MWScript** — Morrowind's built-in scripting language (works fully on client)

MWScript is executed by OpenMW on the client and supports functions like `MessageBox`, `Journal`, `AddItem`, `PlaceItem`, `StartScript`, etc. This is the primary way to add interactive content for players.

- All clients must have the same plugins
- Plugins are distributed via `mods-*.tar.gz` (downloaded from the panel at
  `/api/v1/mods/current/archive`)
- The server checks their presence and CRC via `requiredDataFiles.json`

### Server-side Lua scripts

Server scripts (`.lua` in `mods/scripts/`) execute **on the server**. They use
the `customEventHooks` and `customCommandHooks` API; `ops` regenerates
`customScripts.lua` from this directory on deploy.

- Documentation: `Tutorial.md` shipped with TES3MP

## NOT supported

### Client-side Lua scripts

**No client Lua API exists in TES3MP 0.8.1.** OpenMW 0.47 does not have a built-in Lua engine on the client — support appeared only in OpenMW 0.48+.

- `tes3mp.MessageBox()`, `tes3mp.LoadClientScript()` and similar client functions **do not exist**
- `.lua` files placed in `Data Files/` are ignored and never executed
- Client-side scripting requires **MWScript inside plugins** (see above), not standalone `.lua` files

### The `.omwscripts` format

The `.omwscripts` extension is accepted by the client archive validator, but its
runtime support depends on the OpenMW version. In TES3MP 0.8.1 / OpenMW 0.47:

- `.omwscripts` may be ignored or cause an error on the client
- Only `.esp`/`.esm`/`.omwaddon` are recommended

## Summary

| What can be modded | How |
|-------------------|-----|
| Game data (items, spells, worlds) | Plugins `.esp`/`.esm`/`.omwaddon` |
| Client-side scripting (dialogs, quests, interactive content) | **MWScript** inside plugins |
| Server logic (commands, events) | Lua scripts in `mods/scripts/` |
| Standalone client Lua scripts | **NOT SUPPORTED** in TES3MP 0.8.1 |

Client-side Lua scripting requires TES3MP based on OpenMW 0.48+.
