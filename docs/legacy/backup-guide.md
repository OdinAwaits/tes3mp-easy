> **Legacy / не руководство к действию.** Старая схема бэкапов через `export`
> и автоудаление старше 30 дней больше не используются. Актуально —
> [`docs/v2/08-runbook.md`](../v2/08-runbook.md) §6 и
> [`docs/v2/03-contracts.md`](../v2/03-contracts.md) §6.

# Backup Guide

The server stores backups of two data types: **mods** and **state**. State combines players and world data into a single archive.

## How Backups Work

```
Export service on the VPS (Docker)
    ──► automatically creates state backups every 5 minutes
    ──► stores in /tes3mp-easy/backups/state/
    ──► old backups (>30 days) are automatically cleaned up
    ──► backups only run when TES3MP is running
```

## Safety: Export Health Watcher

Inside the TES3MP container, a background process monitors the export service every 30 seconds.
If export becomes unreachable (crashed, removed, not responding), TES3MP automatically shuts down.
This prevents playing without backups.

## Automatic State Backups

The export service (`export-server`) runs inside a Docker container and:

- **Creates a combined state backup** (players + world) every 5 minutes
- **Only runs when TES3MP is up** — checks via Docker socket
- **Deletes backups older than 30 days** to save disk space
- **Serves backups via HTTP** on port 8085

## Admin Operations

### Exporting Mods

1. Place your mods in `EXPORT_DIR/<server_id>/mods/`:
   ```
   $EXPORT_DIR/<server_id>/mods/plugins/       ← .esp/.esm/.omwaddon
   $EXPORT_DIR/<server_id>/mods/scripts/       ← Lua scripts
   ```

2. From the admin menu, select **Export mods**

### Deploying Data

- **Deploy mods** — apply a mods archive to the server
- **Deploy state** — apply a state archive (players + world) to the server

### Downloading Backups

- **Download mod backup** — download a specific mods archive
- **Download state backup** — select an existing backup or create a fresh one
  - Pick a file from the list, or select **0** to create a fresh backup and download it

### Creating a Fresh State Backup On-Demand

- From the admin menu: **Download state backup** → select option **0**
- From CLI: `bash download-backup-state` (no filename = create fresh)

## Player Operations

- **Install Mods** — downloads and unpacks the latest mods archive automatically
- **Download state backup** — downloads a specific archive

## API Endpoints (HTTP :8085)

| Endpoint | Description |
|----------|-------------|
| `GET /list-backups/mods` | JSON list of mod backups |
| `GET /list-backups/state` | JSON list of state backups |
| `GET /download/mods` | Latest mod backup |
| `GET /download/state` | Latest state backup |
| `GET /download/state?latest` | Force new state backup and download it |
| `GET /download/mods/<file>` | Specific mod backup file |
| `GET /download/state/<file>` | Specific state backup file |

## Restore Procedure

1. **Stop** the server from the admin menu
2. **Deploy** the desired backups (mods, state)
3. **Start** the server
