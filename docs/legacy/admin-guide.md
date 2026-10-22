> **Legacy / не руководство к действию.** Этот документ описывает старую схему
> SSH + nginx + `export`. Актуальная документация — в
> [`docs/v2/README.md`](../v2/README.md), аварийные действия — в
> [`docs/v2/08-runbook.md`](../v2/08-runbook.md). Документ сохранён только для
> истории миграции и будет удалён после v2-MVP.

# Admin Guide

## Prerequisites

- A VPS with **root/sudo access**
- **Port 25565/UDP** open for the game
- Your VPS added to `~/.ssh/config`:
  ```
  Host my-server
      HostName 203.0.113.10
      User root
  ```

---

## 1. Server Setup

**Step 1 — Install scripts on your local machine:**
```bash
curl -fsSL https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/client/install.sh | bash
```

**Step 2 — Start the admin menu:**
```bash
bash ~/.local/share/tes3mp-easy/layer3/admin.sh
```

**Step 3 — Run Quick Setup Wizard:**
- From the menu, select **Quick Setup Wizard**
- The wizard will:
  - Configure SSH host (with connection test)
  - Set up export directory
  - Install the server on your VPS (Docker + TES3MP)
  - Configure server settings (38 options from `config.lua`)
  - Create mod directories and optionally export mods
  - Start the server

That's it — the server is installed, configured, and running.
See the [Player Guide](./player-guide.md) for connecting to the server.

---

## 2. Installing Mods

1. Prepare your mods locally in `EXPORT_DIR/<server_id>`:
   ```
   $EXPORT_DIR/<server_id>/mods/plugins/    ← .esp/.esm/.omwaddon files
   $EXPORT_DIR/<server_id>/mods/scripts/    ← Lua scripts
   ```

   The `server_id` is derived from `SSH_HOST` (e.g., `my-server` becomes `my-server`).
   These directories are created automatically when you open the admin menu.

2. From the admin menu, select **Generate requiredDataFiles** — creates `requiredDataFiles.json` that the server uses to verify all players have the same mods installed

3. From the admin menu, select **Export Mods** (creates archive and uploads to VPS)

4. From the admin menu, select **Deploy Mods** (picks an archive to apply)

5. **Restart the server** for changes to take effect

Players can then download the mods from the player menu with **Install Mods**.

---

## 3. Server Management

| Action | Menu Entry | Description |
|--------|-----------|-------------|
| Start | Start | Starts Docker services on VPS |
| Stop | Stop | Stops Docker services |
| Restart | Restart | Restarts all services |
| Status | Status | Shows if services are running |
| Logs | Logs | Tails the TES3MP log |
| Quick Setup Wizard | Quick Setup Wizard | Guided server setup |

### Modding

| Action | Menu Entry | Description |
|--------|-----------|-------------|
| Generate data files | Generate requiredDataFiles | Creates `requiredDataFiles.json` for mod verification |
| Export mods | Export mods | Package local mods into archive and upload to VPS |
| Deploy mods | Deploy mods | Apply a selected archive to the server |
| Show mod backups | Show mod backups | List mod archives stored on VPS |
| Download mod backup | Download mod backup | Download a mod archive via HTTP |

The admin menu also includes **OpenMW-CS** (Construction Set) under the Server Control section for mod editing.

### Snapshots (State — Players & World combined)

State backups are created automatically every 5 minutes. Backups older than 30 days are deleted automatically.

| Action | Menu Entry | Description |
|--------|-----------|-------------|
| Deploy state | Deploy state | Apply a state archive (players + world) to the server |
| Show state backups | Show state backups | List state archives on VPS |
| Download state backup | Download state backup | Download a state archive via HTTP |

### Config Editing

| Action | Menu Entry | Description |
|--------|-----------|-------------|
| Server config | Server config | Edit `tes3mp-server-default.cfg` on VPS |
| Lua settings | Lua settings | Edit `customScripts.lua` on VPS |
| Ban list | Ban list | Edit `banlist.json` on VPS |
| Settings | Settings | Edit local admin config (`tes3mp-easy.ini`) |

---

<details>
<summary>Backups & further reading</summary>

### Backups

See [Backup Guide](./backup-guide.md) for details on creating, deploying, and downloading backups.

### Further Reading

- [Modding Documentation](./admin/modding.md) — TES3MP modding capabilities
- [Server Settings Reference](./admin/tes3mp_settings.md) — all `config.lua` settings
- [Player Guide](./player-guide.md)

If you want to extend tes3mp-easy, see the [Development Guide](./development/README.md) for internals, full command reference, and how to add new features.

</details>
