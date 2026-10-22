> **Reference.** Справка по `config.lua` TES3MP 0.8.1. Наша панель
> редактирует только белый список ключей; см.
> [`docs/v2/05-panel.md`](../v2/05-panel.md) §11.

# config.lua reference

Full documentation of all settings in `/tes3mp-easy/container-data/server/scripts/config.lua` (TES3MP 0.8.1).

The **Installer** column shows whether the setting is covered by the interactive setup wizard questionnaire.

Legend: ✅ = asked during installation, ❌ = not asked (edit manually).

## Server info

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.serverName` | `"TES3MP Server"` | ✅ | Server name displayed in the server browser |
| `config.serverPassword` | `""` | ✅ | Server password (empty = no password) |

## Game settings

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.gameMode` | `"Default"` | ✅ | Game mode displayed in the server browser |
| `config.difficulty` | `0` | ✅ | Difficulty level (-100..100, any integer) |
| `config.loginTime` | `60` | ✅ | Time to login, in seconds |
| `config.maxClientsPerIP` | `3` | ✅ | Max clients allowed from the same IP |
| `config.dataPath` | auto | ❌ | Data folder path (auto-detected server-side) |
| `config.gameSettings` | *(table)* | ❌ | Enforced OpenMW game settings |
| `config.vrSettings` | *(table)* | ❌ | Enforced VR settings |
| `config.defaultTimeTable` | *(table)* | ❌ | World time for a newly created world |
| `config.chatWindowInstructions` | *(text)* | ❌ | Chat instructions shown to joining players |
| `config.startupScriptsInstructions` | *(text)* | ❌ | Warning shown before /runstartup |
| `config.worldStartupScripts` | *(list)* | ❌ | World startup scripts run via /runstartup |
| `config.playerStartupScripts` | *(list)* | ❌ | Player startup scripts run on join |
| `config.physicsFramerate` | `60` | ❌ | Physics framerate used by default |
| `config.enforcedLogLevel` | `-1` | ❌ | Enforced client log level (-1 = client choice) |

## Time

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.passTimeWhenEmpty` | `false` | ✅ | Pass world time when no players are online |
| `config.nightStartHour` | `20` | ✅ | Hour at which night starts |
| `config.nightEndHour` | `6` | ✅ | Hour at which night ends |

## Permissions

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.allowConsole` | `false` | ✅ | Allow players to use the ~ console |
| `config.allowBedRest` | `true` | ✅ | Allow players to rest in bed |
| `config.allowWildernessRest` | `true` | ✅ | Allow players to rest in the wilderness |
| `config.allowWait` | `true` | ✅ | Allow players to wait |
| `config.allowSuicideCommand` | `true` | ✅ | Allow /suicide command |
| `config.allowFixmeCommand` | `true` | ✅ | Allow /fixme command |
| `config.fixmeInterval` | `30` | ❌ | Cooldown between /fixme uses |

## Sharing

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.shareJournal` | `true` | ✅ | Share journal entries across players |
| `config.shareFactionRanks` | `true` | ✅ | Share faction rank changes |
| `config.shareFactionExpulsion` | `false` | ✅ | Share faction expulsion |
| `config.shareFactionReputation` | `true` | ✅ | Share faction reputation |
| `config.shareTopics` | `true` | ✅ | Share dialogue topics |
| `config.shareBounty` | `false` | ✅ | Share crime bounties |
| `config.shareReputation` | `true` | ✅ | Share reputation |
| `config.shareMapExploration` | `false` | ✅ | Share map exploration |
| `config.shareVideos` | `true` | ✅ | Share ingame videos across players |
| `config.disabledClientScriptIds` | *(list)* | ❌ | Clientside scripts blanked out |
| `config.synchronizedClientScriptIds` | *(list)* | ❌ | Scripts with synced variables across players |

## Respawn & Death

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.playersRespawn` | `true` | ✅ | Players respawn when dying |
| `config.deathTime` | `5` | ✅ | Time dead before respawn, in seconds |
| `config.deathPenaltyJailDays` | `5` | ✅ | Jail days penalty on death |
| `config.bountyResetOnDeath` | `false` | ✅ | Reset bounty to 0 on death |
| `config.bountyDeathPenalty` | `false` | ✅ | Bounty-based jail time on death |
| `config.respawnAtImperialShrine` | `true` | ✅ | Respawn at nearest Imperial shrine |
| `config.respawnAtTribunalTemple` | `true` | ✅ | Respawn at nearest Tribunal temple |
| `config.defaultRespawn` | *(table)* | ❌ | Default respawn location (Balmora Temple) |

## Spawn

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.useInstancedSpawn` | `true` | ❌ | Use instanced spawn (separate cell per player) |
| `config.instancedSpawn` | *(table)* | ❌ | Instanced spawn location (Seyda Neen office) |
| `config.noninstancedSpawn` | *(table)* | ❌ | Non-instanced spawn location |
| `config.forbiddenCells` | `{"ToddTest"}` | ❌ | Cells players cannot enter |

## Collisions

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.enablePlayerCollision` | `true` | ✅ | Player-player collision |
| `config.enableActorCollision` | `true` | ✅ | Actor-actor collision |
| `config.enablePlacedObjectCollision` | `false` | ✅ | Placed objects collide with actors |
| `config.enforcedCollisionRefIds` | *(list)* | ❌ | RefIds with enforced collision |
| `config.useActorCollisionForPlacedObjects` | `false` | ❌ | Use actor-style collision for placed objects |

## Stats limits

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.maxAttributeValue` | `200` | ✅ | Max attribute value (except Speed) |
| `config.maxSpeedValue` | `365` | ✅ | Max Speed attribute |
| `config.maxSkillValue` | `200` | ✅ | Max skill value (except Acrobatics) |
| `config.maxAcrobaticsValue` | `1200` | ✅ | Max Acrobatics value |
| `config.ignoreModifierWithMaxSkill` | `false` | ❌ | Allow modifiers to bypass max skill values |
| `config.bannedEquipmentItems` | `{"helseth's ring"}` | ❌ | Items players cannot equip |

## Safety & enforcement

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.enforceDataFiles` | `true` | ✅ | Enforce same data files for all clients |
| `config.ignoreScriptErrors` | `false` | ✅ | Ignore Lua script errors (dangerous) |
| `config.databaseType` | `"json"` | ❌ | Database format: json or sqlite3 |
| `config.databasePath` | auto | ❌ | Database file path |
| `config.disallowedNameStrings` | *(list)* | ❌ | Substrings disallowed in player/item names |

## Object & record restrictions

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.maximumObjectScale` | `20` | ❌ | Max object scale |
| `config.generatedRecordIdPrefix` | `"$custom"` | ❌ | Prefix for auto-generated record IDs |
| `config.disallowedActivateRefIds` | `{}` | ❌ | RefIds players cannot activate |
| `config.disallowedDeleteRefIds` | `{"m'aiq"}` | ❌ | RefIds players cannot delete |
| `config.disallowedCreateRefIds` | `{}` | ❌ | RefIds players cannot place/spawn |
| `config.disallowedLockRefIds` | `{}` | ❌ | RefIds players cannot lock/unlock |
| `config.disallowedTrapRefIds` | `{}` | ❌ | RefIds players cannot trap/untrap |
| `config.disallowedStateRefIds` | `{}` | ❌ | RefIds players cannot enable/disable |
| `config.disallowedDoorStateRefIds` | `{}` | ❌ | Door refIds players cannot open/close |

## Networking & authority

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.pingDifferenceRequiredForAuthority` | `40` | ❌ | Ping diff needed for cell authority change |
| `config.allowOnContainerForUnloadedCells` | `false` | ❌ | Allow container access in unloaded cells |

## Record store internals

These are complex tables used by the `/storerecord` system. Refer to the stock config.lua for full details.

| Setting | Installer | Description |
|---------|-----------|-------------|
| `config.recordStoreLoadOrder` | ❌ | Order in which record stores are loaded |
| `config.enchantableRecordTypes` | ❌ | Record types that can carry enchantments |
| `config.carriableRecordTypes` | ❌ | Record types stored by players |
| `config.unplaceableRecordTypes` | ❌ | Record types that cannot be placed in world |
| `config.validRecordSettings` | ❌ | Accepted input fields per record type |
| `config.requiredRecordSettings` | ❌ | Required fields for records without baseId |
| `config.mutuallyExclusiveRecordSettings` | ❌ | Mutually exclusive field groups |
| `config.numericalRecordSettings` | ❌ | Fields converted to numbers |
| `config.booleanRecordSettings` | ❌ | Fields converted to booleans |
| `config.minMaxRecordSettings` | ❌ | Fields converted to min/max tables |
| `config.rgbRecordSettings` | ❌ | Fields converted to RGB color tables |
| `config.cellPacketTypes` | ❌ | Packet types stored in cell data |

## Key order (JSON save order)

These control the order of keys in saved JSON files. Rarely changed.

| Setting | Installer | Description |
|---------|-----------|-------------|
| `config.playerKeyOrder` | ❌ | Key order for player JSON files |
| `config.cellKeyOrder` | ❌ | Key order for cell JSON files |
| `config.recordstoreKeyOrder` | ❌ | Key order for record store JSON files |
| `config.worldKeyOrder` | ❌ | Key order for world state JSON files |

## Misc

| Setting | Default | Installer | Description |
|---------|---------|-----------|-------------|
| `config.rankColors` | *(table)* | ❌ | Colors for server ranks |
| `config.customMenuIds` | *(table)* | ❌ | Custom menu ID numbers |
| `config.menuHelperFiles` | *(list)* | ❌ | Menu files for menuHelper |
