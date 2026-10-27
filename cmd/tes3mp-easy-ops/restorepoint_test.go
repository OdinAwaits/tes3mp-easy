package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func restorePointRoot(t *testing.T) string {
	t.Helper()
	root, _ := backupCreateRoot(t)
	for _, dir := range []string{"mods/scripts"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	for _, name := range []string{
		"config.lua", "tes3mp-server-default.cfg", "banlist.json", "customScripts.lua",
	} {
		fixtures.WriteFile(t, filepath.Join(root, name), name+"\n")
	}
	old := backupDiskFreeFunc
	backupDiskFreeFunc = func(string) (int64, error) { return 10 << 30, nil }
	t.Cleanup(func() { backupDiskFreeFunc = old })
	return root
}

func runJSON(t *testing.T, root string, args ...string) (int, json.RawMessage, string, string) {
	t.Helper()
	full := append([]string{"--root", root, "--json"}, args...)
	var stdout, stderr bytes.Buffer
	code := run(full, &stdout, &stderr)

	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("decode envelope: %v; stdout=%q", err, stdout.String())
		}
	}
	data := envelope.Data
	if !envelope.OK {
		data = nil
	}
	return code, data, stdout.String(), stderr.String()
}

func decodeData[T any](t *testing.T, data json.RawMessage) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode data: %v; data=%s", err, data)
	}
	return value
}

func TestSnapshotCreateAndList(t *testing.T) {
	root := restorePointRoot(t)

	code, raw, stdout, stderr := runJSON(t, root,
		"snapshot", "create", "--label", "before quests")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	created := decodeData[snapshotCreateData](t, raw)
	if created.Reason != "manual" || created.Label == nil ||
		*created.Label != "before quests" || created.Size <= 0 ||
		len(created.SHA256) != 64 {
		t.Fatalf("snapshot create = %+v", created)
	}
	path := filepath.Join(restorepoint.Dir(root, restorepoint.KindSnapshot),
		created.Archive)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot archive: %v", err)
	}
	manifest, err := restorepoint.ReadManifest(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !manifest.IsSnapshot() || manifest.Reason != "manual" ||
		len(manifest.ModsSHA256) != 64 || len(manifest.StateSHA256) != 64 ||
		len(manifest.ConfigsSHA256) != 64 {
		t.Fatalf("manifest = %+v", manifest)
	}
	extracted := t.TempDir()
	if _, err := restorepoint.Extract(path, extracted); err != nil {
		t.Fatalf("extract snapshot: %v", err)
	}
	for _, name := range []string{
		"state/players/player.json",
		"state/world/cell/cell.json",
		"state/requiredDataFiles.json",
		"mods/plugins/requiredDataFiles.json",
		"configs/config.lua",
	} {
		if _, err := os.Stat(filepath.Join(extracted, name)); err != nil {
			t.Fatalf("extracted %s: %v", name, err)
		}
	}

	code, raw, stdout, stderr = runJSON(t, root, "snapshot", "list")
	if code != int(ops.ExitOK) {
		t.Fatalf("list exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	list := decodeData[snapshotListData](t, raw)
	if len(list.Items) != 1 || list.Items[0].Name != created.Archive ||
		list.Items[0].UsedBy != 0 || !list.Items[0].Protected {
		t.Fatalf("snapshot list = %+v", list)
	}
}

func runJSONClient(t *testing.T, root string, client *systemd.Client,
	args ...string,
) (int, json.RawMessage, string, string) {
	t.Helper()
	full := append([]string{"--root", root, "--json"}, args...)
	var stdout, stderr bytes.Buffer
	code := runWithClient(full, &stdout, &stderr, client)

	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("decode envelope: %v; stdout=%q", err, stdout.String())
		}
	}
	data := envelope.Data
	if !envelope.OK {
		data = nil
	}
	return code, data, stdout.String(), stderr.String()
}

func TestCheckpointCreateListAndPin(t *testing.T) {
	root := restorePointRoot(t)

	code, raw, stdout, stderr := runJSON(t, root, "snapshot", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	snapshot := decodeData[snapshotCreateData](t, raw)

	code, raw, stdout, stderr = runJSON(t, root,
		"checkpoint", "create", "--reason", "manual")
	if code != int(ops.ExitOK) {
		t.Fatalf("checkpoint exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	checkpoint := decodeData[checkpointCreateData](t, raw)
	if checkpoint.Base == nil || checkpoint.Base.Name != snapshot.Archive ||
		checkpoint.Players == nil || *checkpoint.Players != 1 ||
		checkpoint.Size == nil || *checkpoint.Size <= 0 {
		t.Fatalf("checkpoint create = %+v", checkpoint)
	}
	checkpointPath := filepath.Join(restorepoint.Dir(root, restorepoint.KindCheckpoint),
		checkpoint.Archive)
	if _, err := restorepoint.Extract(checkpointPath, t.TempDir()); err != nil {
		t.Fatalf("extract checkpoint: %v", err)
	}

	code, raw, stdout, stderr = runJSON(t, root, "checkpoint", "list")
	if code != int(ops.ExitOK) {
		t.Fatalf("list exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	list := decodeData[checkpointListData](t, raw)
	if len(list.Items) != 1 || list.Items[0].Name != checkpoint.Archive ||
		!list.Items[0].Base.Available || list.Items[0].Pinned {
		t.Fatalf("checkpoint list = %+v", list)
	}

	code, _, stdout, stderr = runJSON(t, root,
		"checkpoint", "pin", checkpoint.Archive)
	if code != int(ops.ExitOK) {
		t.Fatalf("pin exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, pinned, err := restorepoint.ReadPin(root, checkpoint.Archive); err != nil || !pinned {
		t.Fatalf("ReadPin = %v, %v", pinned, err)
	}

	code, raw, stdout, stderr = runJSON(t, root, "snapshot", "list")
	if code != int(ops.ExitOK) {
		t.Fatalf("snapshot list exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	snapshots := decodeData[snapshotListData](t, raw)
	if len(snapshots.Items) != 1 || snapshots.Items[0].UsedBy != 1 {
		t.Fatalf("snapshot used_by = %+v", snapshots)
	}

	code, _, stdout, stderr = runJSON(t, root,
		"checkpoint", "unpin", checkpoint.Archive)
	if code != int(ops.ExitOK) {
		t.Fatalf("unpin exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, pinned, err := restorepoint.ReadPin(root, checkpoint.Archive); err != nil || pinned {
		t.Fatalf("ReadPin after unpin = %v, %v", pinned, err)
	}
}

func TestCheckpointRestoreWorld(t *testing.T) {
	root := restorePointRoot(t)
	code, raw, stdout, stderr := runJSON(t, root, "snapshot", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	_ = decodeData[snapshotCreateData](t, raw)
	code, raw, stdout, stderr = runJSON(t, root, "checkpoint", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("checkpoint exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	checkpoint := decodeData[checkpointCreateData](t, raw)

	fixtures.WriteFile(t, filepath.Join(root, "players", "player.json"),
		"{\"changed\":true}\n")
	runner := activeRunner(t)
	code, raw, stdout, stderr = runJSONClient(t, root, systemd.New(runner),
		"checkpoint", "restore", "--archive", checkpoint.Archive)
	if code != int(ops.ExitOK) {
		t.Fatalf("restore exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	restored := decodeData[checkpointRestoreData](t, raw)
	if !restored.Restarted || restored.BaseDeployed || restored.BaseMissing ||
		restored.PredeployCheckpoint == "" || restored.Base == nil {
		t.Fatalf("restore = %+v", restored)
	}
	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{}\n" {
		t.Fatalf("restored player = %q, %v", player, err)
	}
}

func TestSnapshotRestoreStateAndConfigs(t *testing.T) {
	root := restorePointRoot(t)
	code, raw, stdout, stderr := runJSON(t, root, "snapshot", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	snapshot := decodeData[snapshotCreateData](t, raw)

	fixtures.WriteFile(t, filepath.Join(root, "players", "player.json"),
		"{\"changed\":true}\n")
	fixtures.WriteFile(t, filepath.Join(root, "config.lua"),
		"changed config\n")
	runner := activeRunner(t)
	code, raw, stdout, stderr = runJSONClient(t, root, systemd.New(runner),
		"snapshot", "restore", "--archive", snapshot.Archive)
	if code != int(ops.ExitOK) {
		t.Fatalf("restore exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	restored := decodeData[snapshotRestoreData](t, raw)
	if !restored.Restarted || restored.BaseSwitched ||
		restored.PredeployCheckpoint == "" {
		t.Fatalf("restore = %+v", restored)
	}
	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{}\n" {
		t.Fatalf("restored player = %q, %v", player, err)
	}
	config, err := os.ReadFile(filepath.Join(root, "config.lua"))
	if err != nil || string(config) != "config.lua\n" {
		t.Fatalf("restored config = %q, %v", config, err)
	}
	backups, err := filepath.Glob(filepath.Join(root, "config.lua.bak-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("config backups = %v, %v", backups, err)
	}
}


func TestRestorePointRemoveProtection(t *testing.T) {
	root := restorePointRoot(t)

	code, raw, stdout, stderr := runJSON(t, root, "snapshot", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	base := decodeData[snapshotCreateData](t, raw)

	code, _, stdout, stderr = runJSON(t, root, "snapshot", "rm", base.Archive)
	if code != int(ops.ExitArchiveInUse) {
		t.Fatalf("rm last snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(
		restorepoint.Dir(root, restorepoint.KindSnapshot), base.Archive)); err != nil {
		t.Fatalf("last snapshot was removed: %v", err)
	}

	code, raw, stdout, stderr = runJSON(t, root, "checkpoint", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("checkpoint exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	checkpoint := decodeData[checkpointCreateData](t, raw)

	code, _, stdout, stderr = runJSON(t, root, "snapshot", "rm", base.Archive)
	if code != int(ops.ExitArchiveInUse) {
		t.Fatalf("rm base exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stdout, stderr = runJSON(t, root,
		"checkpoint", "pin", checkpoint.Archive)
	if code != int(ops.ExitOK) {
		t.Fatalf("pin exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stdout, stderr = runJSON(t, root,
		"checkpoint", "rm", checkpoint.Archive)
	if code != int(ops.ExitArchiveInUse) {
		t.Fatalf("rm pinned exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stdout, stderr = runJSON(t, root,
		"checkpoint", "rm", checkpoint.Archive, "--force")
	if code != int(ops.ExitOK) {
		t.Fatalf("rm pinned --force exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(restorepoint.PinPath(root, checkpoint.Archive)); !os.IsNotExist(err) {
		t.Fatalf("pin sidecar still exists: %v", err)
	}

	code, raw, stdout, stderr = runJSON(t, root, "snapshot", "create")
	if code != int(ops.ExitOK) {
		t.Fatalf("second snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	newBase := decodeData[snapshotCreateData](t, raw)
	if newBase.Archive == base.Archive {
		t.Fatalf("second snapshot reused name %s", newBase.Archive)
	}

	code, raw, stdout, stderr = runJSON(t, root,
		"snapshot", "rm", base.Archive, "--dry-run")
	if code != int(ops.ExitOK) {
		t.Fatalf("dry-run exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	plan := decodeData[restorepoint.RemoveResult](t, raw)
	if len(plan.Removed) != 1 || plan.Removed[0] != base.Archive {
		t.Fatalf("dry-run plan = %+v", plan)
	}
	if _, err := os.Stat(filepath.Join(
		restorepoint.Dir(root, restorepoint.KindSnapshot), base.Archive)); err != nil {
		t.Fatalf("dry-run removed archive: %v", err)
	}

	code, raw, stdout, stderr = runJSON(t, root, "snapshot", "rm", base.Archive)
	if code != int(ops.ExitOK) {
		t.Fatalf("rm old snapshot exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	removed := decodeData[restorepoint.RemoveResult](t, raw)
	if len(removed.Removed) != 1 || removed.Freed <= 0 {
		t.Fatalf("rm result = %+v", removed)
	}
	if _, err := os.Stat(filepath.Join(
		restorepoint.Dir(root, restorepoint.KindSnapshot), base.Archive)); !os.IsNotExist(err) {
		t.Fatalf("old snapshot still exists: %v", err)
	}
}


func TestRetentionKeepsDocumentedLimits(t *testing.T) {
	root := restorePointRoot(t)

	for i := 0; i < 6; i++ {
		code, _, stdout, stderr := runJSON(t, root, "snapshot", "create")
		if code != int(ops.ExitOK) {
			t.Fatalf("snapshot %d exit=%d stdout=%q stderr=%q",
				i, code, stdout, stderr)
		}
	}
	code, raw, stdout, stderr := runJSON(t, root, "snapshot", "list")
	if code != int(ops.ExitOK) {
		t.Fatalf("snapshot list exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	snapshots := decodeData[snapshotListData](t, raw)
	if len(snapshots.Items) != 5 {
		t.Fatalf("snapshots after retention = %d, want 5", len(snapshots.Items))
	}

	for i := 0; i < 3; i++ {
		code, _, stdout, stderr = runJSON(t, root,
			"checkpoint", "create", "--reason", "predeploy")
		if code != int(ops.ExitOK) {
			t.Fatalf("predeploy %d exit=%d stdout=%q stderr=%q",
				i, code, stdout, stderr)
		}
	}
	code, raw, stdout, stderr = runJSON(t, root, "checkpoint", "list")
	if code != int(ops.ExitOK) {
		t.Fatalf("checkpoint list exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	checkpoints := decodeData[checkpointListData](t, raw)
	predeploy := 0
	for _, item := range checkpoints.Items {
		if item.Reason == "predeploy" {
			predeploy++
		}
	}
	if predeploy != 2 {
		t.Fatalf("predeploy after retention = %d, want 2", predeploy)
	}
}
