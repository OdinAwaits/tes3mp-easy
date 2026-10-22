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
	root := doctorRoot(t)
	t.Setenv(osReleaseEnv,
		doctorOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"))
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
	if len(data.Checks) != 6 {
		t.Fatalf("checks = %d, want 6: %#v", len(data.Checks), data.Checks)
	}
	if data.Status != "ok" && data.Status != "warn" {
		t.Fatalf("status = %q, want ok or warn", data.Status)
	}
	for _, name := range []string{"root", "os", "arch", "systemd", "units"} {
		status, message := doctorCheck(t, data, name)
		if status != "ok" {
			t.Errorf("%s = %q (%s), want ok", name, status, message)
		}
	}

	calls := runner.Calls()
	if len(calls) != 4 {
		t.Fatalf("systemd calls = %#v, want version + 3 units", calls)
	}
	if calls[0].Name != "systemctl" || len(calls[0].Args) != 1 ||
		calls[0].Args[0] != "--version" {
		t.Fatalf("first call = %#v, want systemctl --version", calls[0])
	}
}

func TestDoctorJSONWarningStillExitsZero(t *testing.T) {
	root := doctorRoot(t)
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
	root := doctorRoot(t)
	t.Setenv(osReleaseEnv,
		doctorOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"))
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
	root := doctorRoot(t)
	t.Setenv(osReleaseEnv,
		doctorOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"))
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
