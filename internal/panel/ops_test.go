package panel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

type opsCall struct {
	name string
	args []string
}

type fakeOpsRunner struct {
	mu   sync.Mutex
	fn   func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error)
	runs []opsCall
}

func (f *fakeOpsRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	f.mu.Lock()
	f.runs = append(f.runs, opsCall{name: name, args: append([]string(nil), args...)})
	f.mu.Unlock()
	if f.fn == nil {
		return []byte(`{"ok":true,"data":{}}`), nil, 0, nil
	}
	return f.fn(ctx, name, args...)
}

func (f *fakeOpsRunner) calls() []opsCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]opsCall(nil), f.runs...)
}

func newTestOpsClient(t *testing.T, runner OpsRunner) *OpsClient {
	t.Helper()
	client, err := NewOpsClient(OpsClientConfig{
		Binary: "/opt/tes3mp-easy/bin/tes3mp-easy-ops",
		Root:   "/tes3mp-easy",
		Direct: true,
		Runner: runner,
	})
	if err != nil {
		t.Fatalf("NewOpsClient: %v", err)
	}
	return client
}

func TestOpsExitCodeHTTPMapping(t *testing.T) {
	tests := []struct {
		exit ops.ExitCode
		code string
		http int
	}{
		{exit: ops.ExitInternal, code: ops.CodeInternal, http: 500},
		{exit: ops.ExitInvalidArgument, code: ops.CodeInvalidArgument, http: 400},
		{exit: ops.ExitNotFound, code: ops.CodeNotFound, http: 404},
		{exit: ops.ExitLocked, code: ops.CodeLocked, http: 409},
		{exit: ops.ExitNoSpace, code: ops.CodeNoSpace, http: 507},
		{exit: ops.ExitServiceUnavailable, code: ops.CodeServiceUnavailable, http: 503},
		{exit: ops.ExitArchiveInvalid, code: ops.CodeArchiveInvalid, http: 422},
		{exit: ops.ExitUnsafePath, code: ops.CodeUnsafePath, http: 400},
		{exit: ops.ExitConfigInvalid, code: ops.CodeConfigInvalid, http: 422},
		{exit: ops.ExitChecksFailed, code: ops.CodeChecksFailed, http: 503},
		{exit: ops.ExitPluginInUse, code: ops.CodePluginInUse, http: 409},
		{exit: ops.ExitArchiveInUse, code: ops.CodeArchiveInUse, http: 409},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			payload := fmt.Sprintf(
				`{"ok":false,"error":{"code":%q,"message":"ops said no","details":{"field":"value"}}}`,
				tt.code)
			runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
				return []byte(payload), []byte("diagnostic\n"), int(tt.exit), nil
			}}
			client := newTestOpsClient(t, runner)

			_, err := client.Status(context.Background(), "tes3mp.service")
			if err == nil {
				t.Fatal("Status returned nil error")
			}
			var opsErr *OpsError
			if !errors.As(err, &opsErr) {
				t.Fatalf("error type = %T, want *OpsError", err)
			}
			if opsErr.Exit != tt.exit || opsErr.Code != tt.code {
				t.Fatalf("error = exit %d code %q, want %d/%q",
					opsErr.Exit, opsErr.Code, tt.exit, tt.code)
			}
			if opsErr.Message != "ops said no" {
				t.Fatalf("message = %q, want envelope message", opsErr.Message)
			}
			if got := opsErr.Details["field"]; got != "value" {
				t.Fatalf("details[field] = %v, want value", got)
			}
			if got := OpsHTTPStatus(err); got != tt.http {
				t.Fatalf("HTTP status = %d, want %d", got, tt.http)
			}
		})
	}

	t.Run("success", func(t *testing.T) {
		runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
			return []byte(`{"ok":true,"data":{"unit":"tes3mp.service","state":"running","health":"healthy","started_at":null}}`), nil, 0, nil
		}}
		client := newTestOpsClient(t, runner)
		status, err := client.Status(context.Background(), "tes3mp.service")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if status.State != "running" || status.Health != "healthy" || status.StartedAt != nil {
			t.Fatalf("status = %+v", status)
		}
		if got := OpsHTTPStatus(nil); got != 200 {
			t.Fatalf("OpsHTTPStatus(nil) = %d, want 200", got)
		}
	})

	t.Run("exit status wins over envelope code", func(t *testing.T) {
		runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
			return []byte(`{"ok":false,"error":{"code":"not_found","message":"mismatch"}}`), nil, int(ops.ExitInternal), nil
		}}
		client := newTestOpsClient(t, runner)
		_, err := client.Status(context.Background(), "")
		if got := OpsHTTPStatus(err); got != 500 {
			t.Fatalf("HTTP status = %d, want 500 from exit 1", got)
		}
	})
}

func TestOpsHTTPStatusForCode(t *testing.T) {
	tests := map[string]int{
		ops.CodeInvalidArgument:    400,
		ops.CodeUnsafePath:         400,
		ops.CodeNotFound:           404,
		ops.CodeLocked:             409,
		ops.CodeNoSpace:            507,
		ops.CodeServiceUnavailable: 503,
		ops.CodeChecksFailed:       503,
		ops.CodeArchiveInvalid:     422,
		ops.CodeConfigInvalid:      422,
		ops.CodeInternal:           500,
		"unknown":                  500,
	}
	for code, want := range tests {
		if got := OpsHTTPStatusForCode(code); got != want {
			t.Fatalf("OpsHTTPStatusForCode(%q) = %d, want %d", code, got, want)
		}
	}
	if got := OpsHTTPStatus(errors.New("plain")); got != 500 {
		t.Fatalf("OpsHTTPStatus(plain) = %d, want 500", got)
	}
	if got := OpsHTTPStatus(&OpsError{Code: ops.CodeLocked}); got != 409 {
		t.Fatalf("OpsHTTPStatus(code-only locked) = %d, want 409", got)
	}
	if got := OpsHTTPStatus(&OpsError{}); got != 500 {
		t.Fatalf("OpsHTTPStatus(empty) = %d, want 500", got)
	}
}

func opsCommandKey(args []string) string {
	for i, arg := range args {
		if arg == "--json" && i+1 < len(args) {
			return strings.Join(args[i+1:], " ")
		}
	}
	return ""
}

func opsSuccessResponse(args []string) []byte {
	key := opsCommandKey(args)
	switch {
	case key == "status --unit tes3mp.service":
		return []byte(`{"ok":true,"data":{"unit":"tes3mp.service","state":"running","health":"healthy","started_at":"2026-10-23T10:00:00Z"}}`)
	case strings.HasPrefix(key, "logs --tail"):
		return []byte(`{"ok":true,"data":{"lines":["first","second"]}}`)
	case strings.HasPrefix(key, "backups list"):
		return []byte(`{"ok":true,"data":{"type":"mods","current":"mods-current.tar.gz","items":[{"name":"mods-current.tar.gz","size":42,"mtime":"2026-10-23T10:00:00Z","current":true}],"next_before":null}}`)
	case strings.HasPrefix(key, "backups create"):
		return []byte(`{"ok":true,"data":{"archive":"state-manual.tar.gz","size":77,"sha256":"abc","skipped":false,"pruned":[],"warnings":[]}}`)
	case strings.HasPrefix(key, "snapshot list"):
		return []byte(`{"ok":true,"data":{"items":[{"name":"snapshot-2026-10-27_16-40-00.tar.gz","size":2048,"mtime":"2026-10-27T16:40:00Z","reason":"manual","label":null,"mods_sha256":"abc","state_sha256":"def","used_by":1,"protected":true}],"next_before":null}}`)
	case strings.HasPrefix(key, "checkpoint list"):
		return []byte(`{"ok":true,"data":{"items":[{"name":"checkpoint-2026-10-27_16-45-00.tar.gz","size":1024,"mtime":"2026-10-27T16:45:00Z","label":null,"reason":"timer","pinned":false,"players":2,"base":{"name":"snapshot-2026-10-27_16-40-00.tar.gz","sha256":"abc","available":true}}],"next_before":null}}`)
	case strings.HasPrefix(key, "snapshot create"):
		return []byte(`{"ok":true,"data":{"archive":"snapshot-2026-10-27_16-40-00.tar.gz","size":2048,"sha256":"abc","reason":"manual","label":null,"mods_archive":"mods-current.tar.gz","warnings":[]}}`)
	case strings.HasPrefix(key, "checkpoint create"):
		return []byte(`{"ok":true,"data":{"archive":"checkpoint-2026-10-27_16-45-00.tar.gz","size":1024,"sha256":"def","reason":"manual","label":null,"base":{"name":"snapshot-2026-10-27_16-40-00.tar.gz","sha256":"abc","mods_sha256":"123"},"players":2,"skipped":false,"warnings":[]}}`)
	case strings.HasPrefix(key, "snapshot restore"):
		return []byte(`{"ok":true,"data":{"archive":"snapshot-2026-10-27_16-40-00.tar.gz","mods_archive":null,"base_switched":false,"predeploy_checkpoint":"checkpoint-predeploy-2026-10-27_17-00-00.tar.gz","restarted":true}}`)
	case strings.HasPrefix(key, "checkpoint restore"):
		return []byte(`{"ok":true,"data":{"archive":"checkpoint-2026-10-27_16-45-00.tar.gz","base":{"name":"snapshot-2026-10-27_16-40-00.tar.gz","sha256":"abc"},"base_deployed":false,"base_missing":false,"mods_archive":null,"missing_plugins":[],"predeploy_checkpoint":"checkpoint-predeploy-2026-10-27_17-00-00.tar.gz","restarted":true}}`)
	case strings.HasPrefix(key, "mods import"):
		return []byte(`{"ok":true,"data":{"archive":"mods-import.tar.gz","sha256":"def","plugins":3,"scripts":1,"pruned":[],"warnings":[]}}`)
	case strings.HasPrefix(key, "mods deploy"):
		return []byte(`{"ok":true,"data":{"archive":"mods-new.tar.gz","sha256":"123","previous":null,"state_backup":"state-predeploy.tar.gz","restarted":true}}`)
	case strings.HasPrefix(key, "state deploy"):
		return []byte(`{"ok":true,"data":{"archive":"state-restored.tar.gz","mods_archive":"mods-current.tar.gz","predeploy_backup":"state-predeploy.tar.gz","restarted":false}}`)
	case strings.HasPrefix(key, "config get --file server"):
		return []byte(`{"ok":true,"data":{"file":"server","values":{"hostname":"Morrowind with friends","port":25565},"read_only":["rules"]}}`)
	case strings.HasPrefix(key, "config get --file config"):
		return []byte(`{"ok":true,"data":{"file":"config","values":{"loginTime":60},"read_only":[]}}`)
	}
	return []byte(`{"ok":true,"data":{}}`)
}

func TestOpsClientCommands(t *testing.T) {
	runner := &fakeOpsRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, []byte, int, error) {
		return opsSuccessResponse(args), nil, 0, nil
	}}
	client := newTestOpsClient(t, runner)
	ctx := context.Background()

	status, err := client.Status(ctx, "tes3mp.service")
	if err != nil || status.State != "running" || status.StartedAt == nil ||
		*status.StartedAt != "2026-10-23T10:00:00Z" {
		t.Fatalf("Status = %+v, %v", status, err)
	}
	lines, err := client.Logs(ctx, 25)
	if err != nil || !slices.Equal(lines, []string{"first", "second"}) {
		t.Fatalf("Logs = %v, %v", lines, err)
	}
	backups, err := client.BackupsList(ctx, "mods", 50, "mods-before.tar.gz")
	if err != nil || backups.Current == nil || *backups.Current != "mods-current.tar.gz" ||
		len(backups.Items) != 1 || !backups.Items[0].Current || backups.NextBefore != nil {
		t.Fatalf("BackupsList = %+v, %v", backups, err)
	}
	backup, err := client.BackupCreate(ctx, "state", true)
	if err != nil || backup.Archive != "state-manual.tar.gz" || backup.Size != 77 ||
		backup.SHA256 != "abc" || backup.Skipped {
		t.Fatalf("BackupCreate = %+v, %v", backup, err)
	}
	snapshots, err := client.SnapshotsList(ctx, 50, "snapshot-before.tar.gz")
	if err != nil || len(snapshots.Items) != 1 || snapshots.Items[0].UsedBy != 1 ||
		!snapshots.Items[0].Protected {
		t.Fatalf("SnapshotsList = %+v, %v", snapshots, err)
	}
	checkpoints, err := client.CheckpointsList(ctx, 50, "")
	if err != nil || len(checkpoints.Items) != 1 ||
		!checkpoints.Items[0].Base.Available {
		t.Fatalf("CheckpointsList = %+v, %v", checkpoints, err)
	}
	createdSnapshot, err := client.SnapshotCreate(ctx)
	if err != nil || createdSnapshot.Archive != "snapshot-2026-10-27_16-40-00.tar.gz" {
		t.Fatalf("SnapshotCreate = %+v, %v", createdSnapshot, err)
	}
	createdCheckpoint, err := client.CheckpointCreate(ctx)
	if err != nil || createdCheckpoint.Base == nil || createdCheckpoint.Players != 2 {
		t.Fatalf("CheckpointCreate = %+v, %v", createdCheckpoint, err)
	}
	snapshotRestore, err := client.SnapshotRestore(ctx, "snapshot-2026-10-27_16-40-00.tar.gz")
	if err != nil || !snapshotRestore.Restarted || snapshotRestore.ModsArchive != nil {
		t.Fatalf("SnapshotRestore = %+v, %v", snapshotRestore, err)
	}
	checkpointRestore, err := client.CheckpointRestore(ctx,
		"checkpoint-2026-10-27_16-45-00.tar.gz")
	if err != nil || !checkpointRestore.Restarted || checkpointRestore.BaseMissing {
		t.Fatalf("CheckpointRestore = %+v, %v", checkpointRestore, err)
	}
	imported, err := client.ModsImport(ctx, "/tmp/upload")
	if err != nil || imported.Plugins != 3 || imported.Scripts != 1 {
		t.Fatalf("ModsImport = %+v, %v", imported, err)
	}
	modsDeploy, err := client.ModsDeploy(ctx, "current")
	if err != nil || modsDeploy.StateBackup != "state-predeploy.tar.gz" ||
		!modsDeploy.Restarted || modsDeploy.Previous != nil {
		t.Fatalf("ModsDeploy current = %+v, %v", modsDeploy, err)
	}
	if _, err := client.ModsDeploy(ctx, "mods-explicit.tar.gz"); err != nil {
		t.Fatalf("ModsDeploy archive: %v", err)
	}
	if _, err := client.ModsDeploy(ctx, "latest"); err != nil {
		t.Fatalf("ModsDeploy latest: %v", err)
	}
	stateDeploy, err := client.StateDeploy(ctx, "latest")
	if err != nil || stateDeploy.ModsArchive != "mods-current.tar.gz" ||
		stateDeploy.PredeployBackup != "state-predeploy.tar.gz" || stateDeploy.Restarted {
		t.Fatalf("StateDeploy latest = %+v, %v", stateDeploy, err)
	}
	if _, err := client.StateDeploy(ctx, "state-explicit.tar.gz"); err != nil {
		t.Fatalf("StateDeploy archive: %v", err)
	}
	if err := client.StaffSet(ctx, "John Doe", 2); err != nil {
		t.Fatalf("StaffSet: %v", err)
	}
	snapshot, err := client.ConfigSnapshot(ctx, "server")
	if err != nil || snapshot.Values["hostname"] != "Morrowind with friends" ||
		!slices.Equal(snapshot.ReadOnly, []string{"rules"}) {
		t.Fatalf("ConfigSnapshot = %+v, %v", snapshot, err)
	}
	values, err := client.ConfigGet(ctx, "config")
	if err != nil || values["loginTime"] != float64(60) {
		t.Fatalf("ConfigGet = %v, %v", values, err)
	}
	if err := client.ConfigSet(ctx, "config", "hostname", "Morrowind with friends"); err != nil {
		t.Fatalf("ConfigSet string: %v", err)
	}
	if err := client.ConfigSet(ctx, "config", "loginTime", 60); err != nil {
		t.Fatalf("ConfigSet int: %v", err)
	}
	if err := client.ConfigSet(ctx, "config", "enabled", true); err != nil {
		t.Fatalf("ConfigSet bool: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := client.Restart(ctx); err != nil {
		t.Fatalf("Restart: %v", err)
	}

	root := []string{"--root", "/tes3mp-easy", "--json"}
	withRoot := func(extra ...string) []string {
		return append(append([]string{}, root...), extra...)
	}
	calls := runner.calls()
	assertOpsCall(t, calls[0], client.binary, withRoot("status", "--unit", "tes3mp.service")...)
	assertOpsCall(t, calls[1], client.binary, withRoot("logs", "--tail", "25")...)
	assertOpsCall(t, calls[2], client.binary, withRoot(
		"backups", "list", "--type", "mods", "--limit", "50", "--before", "mods-before.tar.gz")...)
	assertOpsCall(t, calls[3], client.binary, withRoot(
		"backups", "create", "--type", "state", "--manual")...)
	assertOpsCall(t, calls[4], client.binary, withRoot(
		"snapshot", "list", "--limit", "50", "--before", "snapshot-before.tar.gz")...)
	assertOpsCall(t, calls[5], client.binary, withRoot(
		"checkpoint", "list", "--limit", "50")...)
	assertOpsCall(t, calls[6], client.binary, withRoot(
		"snapshot", "create", "--reason", "manual")...)
	assertOpsCall(t, calls[7], client.binary, withRoot(
		"checkpoint", "create", "--reason", "manual")...)
	assertOpsCall(t, calls[8], client.binary, withRoot(
		"snapshot", "restore", "--archive", "snapshot-2026-10-27_16-40-00.tar.gz")...)
	assertOpsCall(t, calls[9], client.binary, withRoot(
		"checkpoint", "restore", "--archive", "checkpoint-2026-10-27_16-45-00.tar.gz")...)
	assertOpsCall(t, calls[10], client.binary, withRoot(
		"mods", "import", "--file", "/tmp/upload")...)
	assertOpsCall(t, calls[11], client.binary, withRoot("mods", "deploy", "--current")...)
	assertOpsCall(t, calls[12], client.binary, withRoot(
		"mods", "deploy", "--archive", "mods-explicit.tar.gz")...)
	assertOpsCall(t, calls[13], client.binary, withRoot("mods", "deploy", "--latest")...)
	assertOpsCall(t, calls[14], client.binary, withRoot("state", "deploy", "--latest")...)
	assertOpsCall(t, calls[15], client.binary, withRoot(
		"state", "deploy", "--archive", "state-explicit.tar.gz")...)
	assertOpsCall(t, calls[16], client.binary, withRoot(
		"staff", "set", "--player", "John Doe", "--rank", "2")...)
	assertOpsCall(t, calls[17], client.binary, withRoot("config", "get", "--file", "server")...)
	assertOpsCall(t, calls[18], client.binary, withRoot("config", "get", "--file", "config")...)
	assertOpsCall(t, calls[19], client.binary, withRoot(
		"config", "set", "--file", "config", "--key", "hostname",
		"--value", "Morrowind with friends")...)
	assertOpsCall(t, calls[20], client.binary, withRoot(
		"config", "set", "--file", "config", "--key", "loginTime", "--value", "60")...)
	assertOpsCall(t, calls[21], client.binary, withRoot(
		"config", "set", "--file", "config", "--key", "enabled", "--value", "true")...)
	assertOpsCall(t, calls[22], client.binary, withRoot("start")...)
	assertOpsCall(t, calls[23], client.binary, withRoot("stop")...)
	assertOpsCall(t, calls[24], client.binary, withRoot("restart")...)
}

func assertOpsCall(t *testing.T, call opsCall, name string, args ...string) {
	t.Helper()
	if call.name != name {
		t.Fatalf("command name = %q, want %q", call.name, name)
	}
	if !slices.Equal(call.args, args) {
		t.Fatalf("argv = %#v, want %#v", call.args, args)
	}
}

func TestOpsClientUsesSudoByDefault(t *testing.T) {
	runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
		return []byte(`{"ok":true,"data":{}}`), nil, 0, nil
	}}
	client, err := NewOpsClient(OpsClientConfig{Runner: runner})
	if err != nil {
		t.Fatalf("NewOpsClient: %v", err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	assertOpsCall(t, runner.calls()[0], "sudo",
		"-n", "-u", "tes3mp", "/opt/tes3mp-easy/bin/tes3mp-easy-ops",
		"--root", "/tes3mp-easy", "--json", "start")
}

func TestOpsClientRejectsInvalidLocalArguments(t *testing.T) {
	client := newTestOpsClient(t, &fakeOpsRunner{})
	tests := []struct {
		name string
		call func() error
	}{
		{name: "logs", call: func() error {
			_, err := client.Logs(context.Background(), 0)
			return err
		}},
		{name: "backups list", call: func() error {
			_, err := client.BackupsList(context.Background(), "", 0, "")
			return err
		}},
		{name: "backups limit", call: func() error {
			_, err := client.BackupsList(context.Background(), "mods", 501, "")
			return err
		}},
		{name: "backups create", call: func() error {
			_, err := client.BackupCreate(context.Background(), "", false)
			return err
		}},
		{name: "mods import", call: func() error {
			_, err := client.ModsImport(context.Background(), "")
			return err
		}},
		{name: "mods deploy", call: func() error {
			_, err := client.ModsDeploy(context.Background(), "")
			return err
		}},
		{name: "state deploy", call: func() error {
			_, err := client.StateDeploy(context.Background(), "")
			return err
		}},
		{name: "staff set", call: func() error {
			return client.StaffSet(context.Background(), "", 1)
		}},
		{name: "config get", call: func() error {
			_, err := client.ConfigGet(context.Background(), "")
			return err
		}},
		{name: "config set file", call: func() error {
			return client.ConfigSet(context.Background(), "", "key", "value")
		}},
		{name: "config set key", call: func() error {
			return client.ConfigSet(context.Background(), "config", "", "value")
		}},
		{name: "config set value", call: func() error {
			return client.ConfigSet(context.Background(), "config", "key", []string{"x"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if got := OpsHTTPStatus(err); got != 400 {
				t.Fatalf("HTTP status = %d, want 400 for %v", got, err)
			}
			var opsErr *OpsError
			if !errors.As(err, &opsErr) || opsErr.Code != ops.CodeInvalidArgument {
				t.Fatalf("error = %v, want invalid_argument", err)
			}
		})
	}
	if calls := client.runner.(*fakeOpsRunner).calls(); len(calls) != 0 {
		t.Fatalf("local validation invoked ops: %#v", calls)
	}
}

func TestConfigValueString(t *testing.T) {
	tests := []struct {
		value any
		want  string
		ok    bool
	}{
		{value: "text", want: "text", ok: true},
		{value: "", want: "", ok: true},
		{value: true, want: "true", ok: true},
		{value: false, want: "false", ok: true},
		{value: 60, want: "60", ok: true},
		{value: int64(-2), want: "-2", ok: true},
		{value: uint(3), want: "3", ok: true},
		{value: 1.5, want: "1.5", ok: true},
		{value: float32(2.5), want: "2.5", ok: true},
		{value: nil, ok: false},
		{value: []string{"x"}, ok: false},
	}
	for _, tt := range tests {
		got, err := configValueString(tt.value)
		if tt.ok && (err != nil || got != tt.want) {
			t.Fatalf("configValueString(%#v) = %q, %v; want %q", tt.value, got, err, tt.want)
		}
		if !tt.ok && err == nil {
			t.Fatalf("configValueString(%#v) accepted unsupported value", tt.value)
		}
	}
}

func TestOpsClientRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		exitCode  int
		wantCode  string
		wantHTTP  int
		wantCause bool
	}{
		{name: "empty stdout", stdout: "", wantCode: ops.CodeInternal, wantHTTP: 500, wantCause: true},
		{name: "invalid json", stdout: "{", wantCode: ops.CodeInternal, wantHTTP: 500, wantCause: true},
		{name: "missing data", stdout: `{"ok":true}`, wantCode: ops.CodeInternal, wantHTTP: 500, wantCause: true},
		{name: "failure with exit zero", stdout: `{"ok":false,"error":{"code":"locked","message":"busy"}}`, wantCode: ops.CodeLocked, wantHTTP: 500},
		{name: "success with error", stdout: `{"ok":true,"error":{"code":"internal","message":"bad"}}`, wantCode: ops.CodeInternal, wantHTTP: 500, wantCause: true},
		{name: "trailing object", stdout: `{"ok":true,"data":{}}{"ok":true}`, wantCode: ops.CodeInternal, wantHTTP: 500, wantCause: true},
		{name: "non-zero without envelope", stdout: "not json", exitCode: int(ops.ExitLocked), wantCode: ops.CodeLocked, wantHTTP: 409},
		{name: "non-zero without error object", stdout: `{"ok":true,"data":{}}`, exitCode: int(ops.ExitNotFound), wantCode: ops.CodeNotFound, wantHTTP: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
				return []byte(tt.stdout), nil, tt.exitCode, nil
			}}
			client := newTestOpsClient(t, runner)
			_, err := client.Status(context.Background(), "tes3mp.service")
			if err == nil {
				t.Fatal("Status returned nil error")
			}
			var opsErr *OpsError
			if !errors.As(err, &opsErr) {
				t.Fatalf("error type = %T, want *OpsError", err)
			}
			if opsErr.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", opsErr.Code, tt.wantCode)
			}
			if got := OpsHTTPStatus(err); got != tt.wantHTTP {
				t.Fatalf("HTTP status = %d, want %d", got, tt.wantHTTP)
			}
			if tt.wantCause && opsErr.Cause == nil {
				t.Fatal("Cause is nil, want protocol/decode error")
			}
		})
	}
}

func TestOpsClientHandlesRunnerFailuresAndContext(t *testing.T) {
	t.Run("start failure", func(t *testing.T) {
		runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
			return nil, nil, -1, errors.New("sudo not found")
		}}
		client := newTestOpsClient(t, runner)
		_, err := client.Status(context.Background(), "")
		if got := OpsHTTPStatus(err); got != 500 {
			t.Fatalf("HTTP status = %d, want 500", got)
		}
		if !strings.Contains(err.Error(), "sudo not found") {
			t.Fatalf("error = %v, want runner cause", err)
		}
	})

	t.Run("stdout limit", func(t *testing.T) {
		runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
			return bytes.Repeat([]byte("x"), opsStdoutLimit+1), nil, 0, nil
		}}
		client := newTestOpsClient(t, runner)
		_, err := client.Status(context.Background(), "")
		if got := OpsHTTPStatus(err); got != 500 {
			t.Fatalf("HTTP status = %d, want 500", got)
		}
		if !errors.Is(err, errOpsOutputLimit) {
			t.Fatalf("error = %v, want output limit", err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		runner := &fakeOpsRunner{fn: func(ctx context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
			return nil, nil, -1, ctx.Err()
		}}
		client := newTestOpsClient(t, runner)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Status(ctx, "")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if got := OpsHTTPStatus(err); got != 500 {
			t.Fatalf("HTTP status = %d, want 500", got)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		runner := &fakeOpsRunner{fn: func(ctx context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
			<-ctx.Done()
			return nil, nil, -1, ctx.Err()
		}}
		client, err := NewOpsClient(OpsClientConfig{
			Binary:      "/ops",
			Root:        "/root",
			Direct:      true,
			ReadTimeout: 10 * time.Millisecond,
			Runner:      runner,
		})
		if err != nil {
			t.Fatalf("NewOpsClient: %v", err)
		}
		_, err = client.Status(context.Background(), "")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want deadline exceeded", err)
		}
		if !strings.Contains(err.Error(), "timed out after 10ms") {
			t.Fatalf("error = %v, want timeout detail", err)
		}
	})
}

func TestOpsClientLogsStderr(t *testing.T) {
	var logs bytes.Buffer
	runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
		return []byte(`{"ok":false,"error":{"code":"locked","message":"busy"}}`),
			[]byte("another mutation is running\n"), int(ops.ExitLocked), nil
	}}
	client, err := NewOpsClient(OpsClientConfig{
		Binary: "/ops",
		Root:   "/root",
		Direct: true,
		Runner: runner,
		Logger: log.New(&logs, "", 0),
	})
	if err != nil {
		t.Fatalf("NewOpsClient: %v", err)
	}
	if _, err := client.Status(context.Background(), ""); err == nil {
		t.Fatal("Status returned nil error")
	}
	if !strings.Contains(logs.String(), "ops status: stderr: another mutation is running") {
		t.Fatalf("logs = %q, want stderr line", logs.String())
	}
}

func TestCappedBuffer(t *testing.T) {
	buffer := newCappedBuffer(4)
	n, err := buffer.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("Write = %d, %v; want 6, nil", n, err)
	}
	if !buffer.Truncated() || string(buffer.Bytes()) != "abcd" {
		t.Fatalf("buffer = %q truncated=%v", buffer.Bytes(), buffer.Truncated())
	}
}

func TestOpsExecRunnerHelper(t *testing.T) {
	if os.Getenv("TES3MP_OPS_HELPER") != "1" {
		return
	}
	if delay := os.Getenv("TES3MP_OPS_HELPER_SLEEP"); delay != "" {
		duration, err := time.ParseDuration(delay)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad helper sleep %q: %v", delay, err)
			os.Exit(99)
		}
		time.Sleep(duration)
	}
	fmt.Fprint(os.Stdout, os.Getenv("TES3MP_OPS_HELPER_STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("TES3MP_OPS_HELPER_STDERR"))
	code, err := strconv.Atoi(os.Getenv("TES3MP_OPS_HELPER_EXIT"))
	if err != nil {
		os.Exit(99)
	}
	os.Exit(code)
}

func setOpsExecHelper(t *testing.T, stdout, stderr string, exitCode int) []string {
	t.Helper()
	t.Setenv("TES3MP_OPS_HELPER", "1")
	t.Setenv("TES3MP_OPS_HELPER_SLEEP", "")
	t.Setenv("TES3MP_OPS_HELPER_STDOUT", stdout)
	t.Setenv("TES3MP_OPS_HELPER_STDERR", stderr)
	t.Setenv("TES3MP_OPS_HELPER_EXIT", strconv.Itoa(exitCode))
	return []string{"-test.run=^TestOpsExecRunnerHelper$"}
}

func TestOpsExecRunnerCapturesOutputAndExit(t *testing.T) {
	args := setOpsExecHelper(t, `{"ok":true}`, "warning\n", 0)
	stdout, stderr, exitCode, err := (execOpsRunner{}).Run(
		context.Background(), os.Args[0], args...)
	if err != nil || exitCode != 0 {
		t.Fatalf("Run = exit %d, %v", exitCode, err)
	}
	if string(stdout) != `{"ok":true}` || string(stderr) != "warning\n" {
		t.Fatalf("stdout/stderr = %q/%q", stdout, stderr)
	}

	args = setOpsExecHelper(t, "partial", "boom\n", int(ops.ExitUnsafePath))
	stdout, stderr, exitCode, err = (execOpsRunner{}).Run(
		context.Background(), os.Args[0], args...)
	if err != nil {
		t.Fatalf("Run exit %d: %v", exitCode, err)
	}
	if exitCode != int(ops.ExitUnsafePath) || string(stdout) != "partial" ||
		!strings.Contains(string(stderr), "boom") {
		t.Fatalf("Run = exit %d stdout %q stderr %q", exitCode, stdout, stderr)
	}
}

func TestOpsExecRunnerReportsMissingCommand(t *testing.T) {
	_, _, _, err := (execOpsRunner{}).Run(
		context.Background(), "/definitely/not/a/tes3mp-ops")
	if err == nil {
		t.Fatal("Run accepted a missing executable")
	}
}

func TestOpsExecRunnerHonorsContext(t *testing.T) {
	args := setOpsExecHelper(t, "", "", 0)
	t.Setenv("TES3MP_OPS_HELPER_SLEEP", "10s")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, _, err := (execOpsRunner{}).Run(ctx, os.Args[0], args...)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want deadline exceeded", err)
	}
}

func TestOpsExecRunnerDoesNotInvokeShell(t *testing.T) {
	_, _, _, err := (execOpsRunner{}).Run(context.Background(), "printf hello")
	if err == nil {
		t.Fatal("Run accepted a command name containing shell syntax")
	}
}

func TestOpsClientDoctorKeepsReportOnChecksFailed(t *testing.T) {
	runner := &fakeOpsRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, []byte, int, error) {
		if key := opsCommandKey(args); key != "doctor" {
			t.Fatalf("command = %q, want doctor", key)
		}
		return []byte(`{"ok":true,"data":{"status":"error","checks":[{"name":"root","status":"error","message":"missing"},{"name":"disk","status":"ok"}]}}`),
			nil, int(ops.ExitChecksFailed), nil
	}}
	client := newTestOpsClient(t, runner)
	report, err := client.Doctor(context.Background())
	if err == nil {
		t.Fatal("Doctor returned nil error for checks_failed")
	}
	if report.Status != "error" || len(report.Checks) != 2 ||
		report.Checks[0].Name != "root" || report.Checks[0].Message != "missing" {
		t.Fatalf("report = %+v", report)
	}
	var opsErr *OpsError
	if !errors.As(err, &opsErr) || opsErr.Exit != ops.ExitChecksFailed ||
		opsErr.Code != ops.CodeChecksFailed {
		t.Fatalf("error = %#v, want checks_failed", err)
	}
	if got := OpsHTTPStatus(err); got != 503 {
		t.Fatalf("HTTP status = %d, want 503", got)
	}
	checks, ok := opsErr.Details["checks"].([]DoctorCheck)
	if !ok || len(checks) != 2 {
		t.Fatalf("details checks = %#v", opsErr.Details["checks"])
	}
}

func TestOpsClientDoctorWarningsAreSuccessful(t *testing.T) {
	runner := &fakeOpsRunner{fn: func(context.Context, string, ...string) ([]byte, []byte, int, error) {
		return []byte(`{"ok":true,"data":{"status":"warn","checks":[{"name":"disk","status":"warn","message":"low"}]}}`),
			nil, int(ops.ExitOK), nil
	}}
	client := newTestOpsClient(t, runner)
	report, err := client.Doctor(context.Background())
	if err != nil || report.Status != "warn" || len(report.Checks) != 1 {
		t.Fatalf("Doctor = %+v, %v", report, err)
	}
}
