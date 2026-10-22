> **Legacy / не руководство к действию.** Старый bash-клиент заменён
> `tes3mp-easy-client`. Актуально — [`docs/v2/06-client.md`](../v2/06-client.md)
> и [`docs/v2/08-runbook.md`](../v2/08-runbook.md).

# Player Guide

## Prerequisites

- **Morrowind** installed (Steam, GOG, or any other version)
- **Proton** 9.0+

---

## 1. Client Setup


**Step 1** — Install scripts:

```bash
curl -fsSL https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/client/install.sh | bash
```

**Step 2** — Start the player menu:

```bash
bash ~/.local/share/tes3mp-easy/layer3/player.sh
```

**Step 3** — Run Setup Wizard (recommended):
- From the menu, select **Quick Setup Wizard**
- The wizard will configure Morrowind path, TES3MP directory, Proton path, install the client, fonts, localization, and set up the UI

That's it — the client is ready. See [Run the Game with Mods](#2-run-the-game-with-mods) to connect to a server.

---

## 2. Run the Game with Mods

1. From the player menu, select **Install Mods and Play**
   - The script shows the current server address
   - Press **Enter** to continue, or press **e** to change server address/port/password
   - Mods are downloaded and the client launches automatically
2. To change server settings later, just press **e** when prompted before launch

You can also launch **OpenMW-CS** (Construction Set) from the player menu for modding.

### How mods are stored

Server mods are now kept **separately** from your Morrowind installation. Each server gets its own folder:

```
TES3MP_DIR/
├── prefix/pfx/drive_c/
│   ├── Morrowind/Data Files/    ← symlink to original Morrowind (read-only)
│   ├── servers/
│   │   ├── myserver.com/
│   │   │   ├── data/Data Files/ ← mod plugins for this server
│   │   │   └── openmw.cfg       ← generated config (data= + content=)
│   │   └── anotherserver.net/
│   │       ├── data/Data Files/
│   │       └── openmw.cfg
│   └── users/steamuser/.../OpenMW/openmw.cfg  ← user config (symlink target)
├── prefix/                      ← Proton prefix
└── tes3mp.exe                   ← client (shared)
```
Server data is stored **inside the Wine prefix** so OpenMW can access it directly via paths like `C:\servers\myserver.com\data\Data Files` — no additional symlinks needed.

When you connect to a new server, the **Install Mods and Play** script:
1. Reads the server address (from `destinationAddress` in `tes3mp-client-default.cfg`)
2. Creates a folder for that server inside `TES3MP_DIR/servers/<address>/`
3. Downloads mods into `data/Data Files/` inside that folder
4. Generates an `openmw.cfg` pointing to both your Morrowind installation and the server's mod folder
5. Replaces the config in the Proton prefix

Your original Morrowind directory remains **untouched** — no plugins are copied there. This makes switching between servers clean and fast: just change the server address and run **Install Mods and Play** again.

---

## 3. Launch via External Launcher (Steam / Lutris)

Instead of opening `player.sh` every time, you can configure a launcher to run the **"Install mods and play"** command directly. This will automatically update mods before each launch.

### Option A — CLI Alias

Add to `~/.bashrc` (or `~/.bash_aliases`):

```bash
alias tes3mp='bash ~/.local/share/tes3mp-easy/layer3/player.sh install-mods-and-play'
```

Then simply run `tes3mp` from a terminal.

### Option B — Steam (Non-Steam Game)

1. Open Steam → **Games** → **Add a Non-Steam Game to My Library**
2. Click **Browse** and select `/bin/bash`
3. In **Launch Options**, paste:
   ```
   ~/.local/share/tes3mp-easy/layer3/player.sh install-mods-and-play
   ```
4. (Optional) Rename the shortcut to "TES3MP" and assign an icon

The script will:
1. Show current server address (press Enter to continue or **e** to change)
2. Download the latest server mods into an isolated folder
3. Launch TES3MP with the correct Proton and prefix

### Option C — Lutris

1. **Add a new game** in Lutris
2. **Runner**: select **Wine** (or Proton via `Wine version` manager)
3. **Executable**: `/bin/bash`
4. **Arguments**:
   ```
   ~/.local/share/tes3mp-easy/layer3/player.sh install-mods-and-play
   ```
5. **Wine prefix**: point to `$TES3MP_DIR/prefix` (where TES3MP was installed)

---

<details>
<summary>Further reading</summary>

- [Backup Guide](./backup-guide.md) — downloading backups from the server
- [Admin Guide](./admin-guide.md)

</details>

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Proton not found" | Install Proton 9.0+, or set `PROTON_PATH` in config |
| "Morrowind.esm not found" | Check `MORROWIND_PATH` path |
| Connection fails | Check server is running, matching TES3MP versions |
