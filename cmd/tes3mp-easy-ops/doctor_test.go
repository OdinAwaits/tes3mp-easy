package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type doctorPayload struct {
	Status string `json:"status"`
	Checks []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"checks"`
}

func doctorRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range []string{
		"configs", "players", "world", "mods", "backups", "panel-data",
	} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	return root
}

func doctorOSRelease(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	return path
}

func doctorMode(mode uint32) os.FileMode {
	out := os.FileMode(mode & 0o777)
	if mode&0o2000 != 0 {
		out |= os.ModeSetgid
	}
	return out
}

func doctorHealthyRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	dirs := []struct {
		path string
		mode uint32
	}{
		{path: "", mode: 0o755},
		{path: "configs", mode: 0o700},
		{path: "players", mode: 0o750},
		{path: "world", mode: 0o750},
		{path: "mods", mode: 0o750},
		{path: "backups", mode: 0o750},
		{path: "panel-data", mode: 0o750},
		{path: "panel-data/tls", mode: 0o700},
		{path: "panel-data/acme", mode: 0o700},
		{path: "panel-data/uploads", mode: 0o2770},
		{path: ".locks", mode: 0o700},
		{path: ".staging", mode: 0o700},
		{path: ".config", mode: 0o700},
		{path: ".config/openmw", mode: 0o700},
		{path: ".local", mode: 0o700},
		{path: ".local/share", mode: 0o700},
		{path: ".local/share/openmw", mode: 0o700},
		{path: "world/cell", mode: 0o750},
		{path: "world/world", mode: 0o750},
		{path: "world/map", mode: 0o750},
		{path: "world/recordstore", mode: 0o750},
		{path: "world/custom", mode: 0o750},
		{path: "mods/plugins", mode: 0o750},
		{path: "mods/scripts", mode: 0o750},
		{path: "backups/mods", mode: 0o750},
		{path: "backups/state", mode: 0o750},
	}
	for _, dir := range dirs {
		path := root
		if dir.path != "" {
			path = filepath.Join(root, dir.path)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
	}
	for _, dir := range dirs {
		path := root
		if dir.path != "" {
			path = filepath.Join(root, dir.path)
		}
		if err := os.Chmod(path, doctorMode(dir.mode)); err != nil {
			t.Fatalf("chmod %s: %v", path, err)
		}
	}

	fixtures.WriteConfigs(t, filepath.Join(root, "configs"))
	fixtures.WriteFile(t,
		filepath.Join(root, "configs", "customScripts.lua"), "")

	mods := fixtures.DefaultModsInit()
	manifest := mods.Install(t, filepath.Join(root, "backups", "mods"))
	if err := os.Chmod(filepath.Join(root, "backups", "mods", "current.txt"),
		0o640); err != nil {
		t.Fatalf("chmod current.txt: %v", err)
	}
	fixtures.WriteFile(t,
		filepath.Join(root, "mods", "plugins", "requiredDataFiles.json"),
		string(mods.RequiredJSON()))
	fixtures.DefaultState(manifest).Write(t,
		filepath.Join(root, "backups", "state", fixtures.StateName(fixtures.DefaultTime)))
	return root
}

func doctorRuntimeRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	paths := []struct {
		path       string
		executable bool
	}{
		{path: "tes3mp-server", executable: true},
		{path: "tes3mp-server.x86_64", executable: true},
		{path: "tes3mp-prelaunch", executable: true},
		{path: "openmw.cfg"},
		{path: "tes3mp-server-default.cfg"},
		{path: "server/scripts/config.lua"},
		{path: "server/scripts/customScripts.lua"},
		{path: "server/scripts/serverCore.lua"},
		{path: "server/data/requiredDataFiles.json"},
	}
	for _, item := range paths {
		path := filepath.Join(root, item.path)
		mode := os.FileMode(0o644)
		if item.executable {
			mode = 0o755
		}
		fixtures.WriteFile(t, path, "synthetic runtime\n")
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("chmod %s: %v", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatalf("create lib: %v", err)
	}
	return root
}

func setDoctorEnv(t *testing.T) {
	t.Helper()

	procRoot := t.TempDir()
	fixtures.WriteFile(t, filepath.Join(procRoot, "meminfo"),
		"MemTotal: 1048576 kB\nSwapTotal: 1048576 kB\n")
	fixtures.WriteFile(t, filepath.Join(procRoot, "sys/vm/swappiness"), "10\n")

	sysctlPath := filepath.Join(t.TempDir(), "99-tes3mp-easy.conf")
	fixtures.WriteFile(t, sysctlPath, "vm.swappiness=10\n")

	journaldPath := filepath.Join(t.TempDir(), "tes3mp-easy.conf")
	fixtures.WriteFile(t, journaldPath,
		"[Journal]\nSystemMaxUse=100M\nRuntimeMaxUse=32M\n")

	binDir := t.TempDir()
	for name, body := range map[string]string{
		"ldconfig": "#!/bin/sh\nprintf '%s\\n' 'libluajit-5.1.so.2 (libc6,x86-64)'\n",
		"ldd":      "#!/bin/sh\nprintf '%s\\n' 'libc.so.6 => /lib/libc.so.6'\n",
	} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	t.Setenv(osReleaseEnv,
		doctorOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"))
	t.Setenv(procRootEnv, procRoot)
	t.Setenv(sysctlConfEnv, sysctlPath)
	t.Setenv(journaldConfEnv, journaldPath)
	t.Setenv(runtimeRootEnv, doctorRuntimeRoot(t))
	t.Setenv(skipOwnerCheckEnv, "1")
}

func doctorClient(t *testing.T, versionErr error) (*systemd.Client, *systemdtest.Runner) {
	t.Helper()

	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"--version"},
		"systemd 255 (255.4-1ubuntu8)\n", versionErr)
	for _, unit := range []string{
		systemd.UnitTES3MPService,
		systemd.UnitPanelService,
		systemd.UnitBackupTimer,
	} {
		runner.Respond("systemctl", []string{
			"show", unit,
			"--property=" + systemd.ShowProperties,
			"--timestamp=utc",
		}, "ActiveState=active\nLoadState=loaded\nUnitFileState=enabled\n", nil)
	}
	cgroups := map[string]string{
		systemd.UnitTES3MPService: "MemoryHigh=402653184\nMemoryMax=469762048\n",
		systemd.UnitPanelService:  "MemoryHigh=50331648\nMemoryMax=67108864\n",
		systemd.UnitBackupService: "MemoryHigh=50331648\nMemoryMax=67108864\n",
	}
	for unit, properties := range cgroups {
		runner.Respond("systemctl", []string{
			"show", unit, "--property=MemoryHigh,MemoryMax",
		}, properties, nil)
	}
	runner.Respond("journalctl", []string{
		"-u", systemd.UnitTES3MPService,
		"-n", "1", "--no-pager", "-o", "cat",
	}, "", nil)
	return systemd.New(runner), runner
}

func decodeDoctor(t *testing.T, stdout string) doctorPayload {
	t.Helper()

	envelope := decodeOneEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("envelope.ok = false, want true; stdout=%q", stdout)
	}
	raw, err := json.Marshal(envelope.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var data doctorPayload
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode doctor data: %v", err)
	}
	return data
}

func doctorCheck(t *testing.T, data doctorPayload, name string) (string, string) {
	t.Helper()

	for _, check := range data.Checks {
		if check.Name == name {
			return check.Status, check.Message
		}
	}
	t.Fatalf("doctor has no %q check; checks=%#v", name, data.Checks)
	return "", ""
}

func TestDoctorJSONOK(t *testing.T) {
	root := doctorHealthyRoot(t)
	setDoctorEnv(t)
	client, runner := doctorClient(t, nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--json", "doctor"},
		&stdout, &stderr, client)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	data := decodeDoctor(t, stdout.String())
	if len(data.Checks) != 19 {
		t.Fatalf("checks = %d, want 19: %#v", len(data.Checks), data.Checks)
	}
	if data.Status != "ok" && data.Status != "warn" {
		t.Fatalf("status = %q (%#v), want ok or warn", data.Status, data.Checks)
	}
	for _, name := range []string{
		"root", "os", "arch", "systemd", "units",
		"ownership", "journald", "runtime", "memory", "cgroup",
		"filesystems", "budgets", "current", "datafiles", "archives",
		"configs", "cleanup", "journald-config",
	} {
		status, message := doctorCheck(t, data, name)
		if status != "ok" {
			t.Errorf("%s = %q (%s), want ok", name, status, message)
		}
	}

	calls := runner.Calls()
	if len(calls) != 8 {
		t.Fatalf("systemd calls = %#v, want version + 3 units + journal + 3 cgroups", calls)
	}
	if calls[0].Name != "systemctl" || len(calls[0].Args) != 1 ||
		calls[0].Args[0] != "--version" {
		t.Fatalf("first call = %#v, want systemctl --version", calls[0])
	}
}

func TestDoctorJSONWarningStillExitsZero(t *testing.T) {
	root := doctorHealthyRoot(t)
	setDoctorEnv(t)
	t.Setenv(osReleaseEnv,
		doctorOSRelease(t, "ID=cachyos\nVERSION_ID=rolling\n"))
	client, _ := doctorClient(t, nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--json", "doctor"},
		&stdout, &stderr, client)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}

	data := decodeDoctor(t, stdout.String())
	if data.Status != "warn" {
		t.Fatalf("status = %q, want warn", data.Status)
	}
	status, message := doctorCheck(t, data, "os")
	if status != "warn" || !strings.Contains(message, "unsupported") {
		t.Fatalf("os check = %q (%s), want unsupported warning", status, message)
	}
}

func TestDoctorJSONErrorUsesChecksFailed(t *testing.T) {
	t.Setenv(osReleaseEnv,
		doctorOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"))
	client, _ := doctorClient(t, nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", t.TempDir(), "--json", "doctor"},
		&stdout, &stderr, client)
	if code != int(ops.ExitChecksFailed) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitChecksFailed, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	data := decodeDoctor(t, stdout.String())
	if data.Status != "error" {
		t.Fatalf("status = %q, want error; checks=%#v", data.Status, data.Checks)
	}
	status, message := doctorCheck(t, data, "root")
	if status != "error" || !strings.Contains(message, "missing") {
		t.Fatalf("root check = %q (%s), want missing-directory error", status, message)
	}
}

func TestDoctorSystemdUnavailableIsACheckError(t *testing.T) {
	root := doctorHealthyRoot(t)
	setDoctorEnv(t)
	client, runner := doctorClient(t, errors.New("systemctl: command not found"))

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--json", "doctor"},
		&stdout, &stderr, client)
	if code != int(ops.ExitChecksFailed) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitChecksFailed, stdout.String(), stderr.String())
	}

	data := decodeDoctor(t, stdout.String())
	if data.Status != "error" {
		t.Fatalf("status = %q, want error", data.Status)
	}
	status, message := doctorCheck(t, data, "systemd")
	if status != "error" || !strings.Contains(message, "not found") {
		t.Fatalf("systemd check = %q (%s), want command error", status, message)
	}
	status, _ = doctorCheck(t, data, "units")
	if status != "error" {
		t.Fatalf("units status = %q, want error", status)
	}
	if calls := runner.Calls(); len(calls) != 1 {
		t.Fatalf("calls = %#v, want only systemctl --version", calls)
	}
}

func TestDoctorRejectsArgumentsBeforeCallingSystemd(t *testing.T) {
	client, runner := doctorClient(t, nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--json", "doctor", "--unknown"},
		&stdout, &stderr, client)
	if code != int(ops.ExitInvalidArgument) {
		t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	assertJSONError(t, stdout.String(), ops.ExitInvalidArgument, "doctor")
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("doctor called systemd for invalid arguments: %#v", calls)
	}
}

func TestDoctorHumanOutput(t *testing.T) {
	root := doctorHealthyRoot(t)
	setDoctorEnv(t)
	client, _ := doctorClient(t, nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "doctor"},
		&stdout, &stderr, client)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}

	for _, want := range []string{
		"root: ok\n",
		"os: ok (Debian 12)\n",
		"arch: ok\n",
		"systemd: ok (systemd 255 (255.4-1ubuntu8))\n",
		"units: ok\n",
		"doctor: ",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}
