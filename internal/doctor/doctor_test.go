package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
)

func writeTestRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, required := range requiredDataDirs {
		if err := os.Mkdir(filepath.Join(root, required.name), 0o755); err != nil {
			t.Fatalf("create %s: %v", required.name, err)
		}
	}
	return root
}

func writeOSRelease(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	return path
}

func fixedUname(machine string, err error) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		return machine, err
	}
}

func fixedFree(bytes int64, err error) func(string) (int64, error) {
	return func(string) (int64, error) {
		return bytes, err
	}
}

type unitResponse struct {
	stdout string
	err    error
}

func healthyUnit() unitResponse {
	return unitResponse{stdout: "ActiveState=active\n" +
		"LoadState=loaded\n" +
		"UnitFileState=enabled\n"}
}

func newTestClient(t *testing.T, version string, versionErr error,
	states map[string]unitResponse,
) *systemd.Client {
	t.Helper()

	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"--version"}, version, versionErr)
	for _, unit := range checkedUnits {
		response, ok := states[unit.name]
		if !ok {
			response = healthyUnit()
		}
		runner.Respond("systemctl", []string{
			"show", unit.name,
			"--property=" + systemd.ShowProperties,
			"--timestamp=utc",
		}, response.stdout, response.err)
	}
	return systemd.New(runner)
}

func TestRunAllChecksOK(t *testing.T) {
	client := newTestClient(t, "systemd 255 (255.1)\nextra output\n", nil, nil)
	report := Run(context.Background(), Options{
		Root:          writeTestRoot(t),
		OSReleasePath: writeOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"),
		Systemd:       client,
		Uname:         fixedUname("x86_64\n", nil),
		DiskFree:      fixedFree(2<<30, nil),
	})

	if report.Status != StatusOK {
		t.Fatalf("status = %q, want ok; checks=%#v", report.Status, report.Checks)
	}
	wantNames := []string{"root", "os", "arch", "systemd", "units", "disk"}
	gotNames := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		gotNames = append(gotNames, check.Name)
		if check.Status != StatusOK {
			t.Errorf("check %s = %q (%s), want ok",
				check.Name, check.Status, check.Message)
		}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("check order = %v, want %v", gotNames, wantNames)
	}
}

func TestRunAggregatesWarningsAndErrors(t *testing.T) {
	client := newTestClient(t, "systemd 255", nil, nil)

	warnReport := Run(context.Background(), Options{
		Root:          writeTestRoot(t),
		OSReleasePath: writeOSRelease(t, "ID=cachyos\nVERSION_ID=rolling\n"),
		Systemd:       client,
		Uname:         fixedUname("x86_64", nil),
		DiskFree:      fixedFree(2<<30, nil),
	})
	if warnReport.Status != StatusWarn {
		t.Fatalf("warning status = %q, want warn", warnReport.Status)
	}
	osCheck := checkByName(warnReport, "os")
	if osCheck.Status != StatusWarn || !strings.Contains(osCheck.Message, "unsupported") {
		t.Fatalf("os check = %#v, want unsupported warning", osCheck)
	}

	errorReport := Run(context.Background(), Options{
		Root:          writeTestRoot(t),
		OSReleasePath: writeOSRelease(t, "ID=cachyos\nVERSION_ID=rolling\n"),
		Systemd:       client,
		Uname:         fixedUname("arm64", nil),
		DiskFree:      fixedFree(2<<30, nil),
	})
	if errorReport.Status != StatusError {
		t.Fatalf("error status = %q, want error", errorReport.Status)
	}
	archCheck := checkByName(errorReport, "arch")
	if archCheck.Status != StatusError {
		t.Fatalf("arch check = %#v, want error", archCheck)
	}
}

func checkByName(report Report, name string) Check {
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	return Check{}
}

func TestSummarize(t *testing.T) {
	tests := []struct {
		name   string
		checks []Check
		want   Status
	}{
		{name: "empty", want: StatusOK},
		{name: "ok", checks: []Check{{Status: StatusOK}}, want: StatusOK},
		{name: "warn", checks: []Check{{Status: StatusOK}, {Status: StatusWarn}}, want: StatusWarn},
		{name: "error wins", checks: []Check{{Status: StatusWarn}, {Status: StatusError}}, want: StatusError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summarize(tt.checks); got != tt.want {
				t.Fatalf("summarize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := Options{}.withDefaults()
	if opts.OSReleasePath != DefaultOSReleasePath {
		t.Fatalf("OSReleasePath = %q, want %q", opts.OSReleasePath, DefaultOSReleasePath)
	}
	if opts.CommandTimeout != DefaultCommandTimeout {
		t.Fatalf("CommandTimeout = %v, want %v", opts.CommandTimeout, DefaultCommandTimeout)
	}
	if opts.Uname == nil || opts.DiskFree == nil {
		t.Fatal("default probes are nil")
	}
}

func TestCheckRoot(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		check := checkRoot(writeTestRoot(t))
		if check.Status != StatusOK || check.Message != "" {
			t.Fatalf("check = %#v, want clean ok", check)
		}
	})

	t.Run("missing root", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing")
		check := checkRoot(path)
		if check.Status != StatusError || !strings.Contains(check.Message, "cannot access") {
			t.Fatalf("check = %#v, want access error", check)
		}
	})

	t.Run("root is a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "root")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		check := checkRoot(path)
		if check.Status != StatusError || !strings.Contains(check.Message, "not a directory") {
			t.Fatalf("check = %#v, want not-a-directory error", check)
		}
	})

	t.Run("missing data directory", func(t *testing.T) {
		root := writeTestRoot(t)
		if err := os.Remove(filepath.Join(root, "backups")); err != nil {
			t.Fatal(err)
		}
		check := checkRoot(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "backups/ is missing") {
			t.Fatalf("check = %#v, want missing backups error", check)
		}
	})

	t.Run("data path is a file", func(t *testing.T) {
		root := writeTestRoot(t)
		path := filepath.Join(root, "world")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		check := checkRoot(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "not a directory") {
			t.Fatalf("check = %#v, want not-a-directory error", check)
		}
	})

	t.Run("data path is a symlink", func(t *testing.T) {
		root := writeTestRoot(t)
		target := filepath.Join(root, "world-target")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "configs")
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		check := checkRoot(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "configs/ is a symlink") {
			t.Fatalf("check = %#v, want symlink error", check)
		}
	})

	t.Run("not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write to a 0555 directory")
		}
		root := writeTestRoot(t)
		path := filepath.Join(root, "players")
		if err := os.Chmod(path, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o755) })

		check := checkRoot(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "players/ is not writable") {
			t.Fatalf("check = %#v, want write error", check)
		}
	})

	t.Run("empty root", func(t *testing.T) {
		check := checkRoot("")
		if check.Status != StatusError || !strings.Contains(check.Message, "empty") {
			t.Fatalf("check = %#v, want empty-root error", check)
		}
	})
}

func TestCheckOS(t *testing.T) {
	tests := []struct {
		name    string
		content string
		status  Status
		message string
	}{
		{name: "debian 12", content: "ID=debian\nVERSION_ID=\"12\"\n",
			status: StatusOK, message: "Debian 12"},
		{name: "debian 13", content: "ID=debian\nVERSION_ID=\"13.1\"\n",
			status: StatusOK, message: "Debian 13.1"},
		{name: "ubuntu 24.04", content: "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
			status: StatusOK, message: "Ubuntu 24.04"},
		{name: "old debian", content: "ID=debian\nVERSION_ID=\"11\"\n",
			status: StatusWarn, message: "older than"},
		{name: "old ubuntu", content: "ID=ubuntu\nVERSION_ID=\"22.04\"\n",
			status: StatusWarn, message: "older than"},
		{name: "unknown os", content: "ID=arch\nVERSION_ID=rolling\n",
			status: StatusWarn, message: "unsupported OS"},
		{name: "missing id", content: "VERSION_ID=12\n",
			status: StatusError, message: "ID is missing"},
		{name: "missing version", content: "ID=debian\n",
			status: StatusError, message: "VERSION_ID is missing"},
		{name: "bad version", content: "ID=debian\nVERSION_ID=stable\n",
			status: StatusWarn, message: "cannot compare"},
		{name: "comments and quotes",
			content: "# comment\nID='ubuntu'\nVERSION_ID=\"24.04\"\n",
			status:  StatusOK, message: "Ubuntu 24.04"},
		{name: "malformed line", content: "ID=debian\nnot-a-pair\n",
			status: StatusError, message: "not KEY=VALUE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := checkOS(writeOSRelease(t, tt.content))
			if check.Status != tt.status {
				t.Fatalf("status = %q (%s), want %q",
					check.Status, check.Message, tt.status)
			}
			if !strings.Contains(check.Message, tt.message) {
				t.Fatalf("message = %q, want substring %q",
					check.Message, tt.message)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		check := checkOS(filepath.Join(t.TempDir(), "missing"))
		if check.Status != StatusError || !strings.Contains(check.Message, "cannot read") {
			t.Fatalf("check = %#v, want read error", check)
		}
	})
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		required string
		actual   string
		want     bool
		wantErr  bool
	}{
		{required: "12", actual: "12", want: true},
		{required: "12", actual: "12.5", want: true},
		{required: "24.04", actual: "24.04", want: true},
		{required: "24.04", actual: "24.10", want: true},
		{required: "24.04", actual: "24.4", want: true},
		{required: "24.04", actual: "24.03", want: false},
		{required: "12", actual: "11.9", want: false},
		{required: "12", actual: "stable", wantErr: true},
		{required: "12", actual: "12.", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.required+"_"+tt.actual, func(t *testing.T) {
			got, err := versionAtLeast(tt.required, tt.actual)
			if (err != nil) != tt.wantErr {
				t.Fatalf("versionAtLeast(%q, %q) error = %v, wantErr %v",
					tt.required, tt.actual, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("versionAtLeast(%q, %q) = %v, want %v",
					tt.required, tt.actual, got, tt.want)
			}
		})
	}
}

func TestCheckArch(t *testing.T) {
	tests := []struct {
		name    string
		machine string
		err     error
		status  Status
	}{
		{name: "x86_64", machine: "x86_64\n", status: StatusOK},
		{name: "arm64", machine: "aarch64", status: StatusError},
		{name: "empty", machine: "", status: StatusError},
		{name: "probe error", err: errors.New("no uname"), status: StatusError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := checkArch(context.Background(), fixedUname(tt.machine, tt.err))
			if check.Status != tt.status {
				t.Fatalf("check = %#v, want status %q", check, tt.status)
			}
			if tt.status != StatusOK && check.Message == "" {
				t.Fatal("failed check has no message")
			}
		})
	}
}
func TestCheckSystemd(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		client := newTestClient(t, "systemd 255 (255.1)\n", nil, nil)
		check := checkSystemd(context.Background(), client, DefaultCommandTimeout)
		if check.Status != StatusOK || check.Message != "systemd 255 (255.1)" {
			t.Fatalf("check = %#v, want version", check)
		}
	})

	t.Run("version error", func(t *testing.T) {
		client := newTestClient(t, "", errors.New("systemctl: not found"), nil)
		check := checkSystemd(context.Background(), client, DefaultCommandTimeout)
		if check.Status != StatusError || !strings.Contains(check.Message, "not found") {
			t.Fatalf("check = %#v, want version error", check)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		check := checkSystemd(context.Background(), nil, DefaultCommandTimeout)
		if check.Status != StatusError || !strings.Contains(check.Message, "not configured") {
			t.Fatalf("check = %#v, want configuration error", check)
		}
	})
}

func TestCheckUnits(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		client := newTestClient(t, "systemd 255", nil, nil)
		check := checkUnits(context.Background(), client, DefaultCommandTimeout, true)
		if check.Status != StatusOK || check.Message != "" {
			t.Fatalf("check = %#v, want clean ok", check)
		}
	})

	t.Run("systemd unavailable", func(t *testing.T) {
		check := checkUnits(context.Background(), nil, DefaultCommandTimeout, false)
		if check.Status != StatusError || !strings.Contains(check.Message, "systemd") {
			t.Fatalf("check = %#v, want unavailable error", check)
		}
	})

	t.Run("unit not installed", func(t *testing.T) {
		client := newTestClient(t, "systemd 255", nil, map[string]unitResponse{
			systemd.UnitPanelService: {stdout: "ActiveState=inactive\nLoadState=not-found\n"},
		})
		check := checkUnits(context.Background(), client, DefaultCommandTimeout, true)
		if check.Status != StatusError ||
			!strings.Contains(check.Message, "tes3mp-panel.service is not installed") {
			t.Fatalf("check = %#v, want not-installed error", check)
		}
	})

	t.Run("service failed", func(t *testing.T) {
		client := newTestClient(t, "systemd 255", nil, map[string]unitResponse{
			systemd.UnitTES3MPService: {stdout: "ActiveState=failed\nLoadState=loaded\n"},
		})
		check := checkUnits(context.Background(), client, DefaultCommandTimeout, true)
		if check.Status != StatusError ||
			!strings.Contains(check.Message, "tes3mp.service is failed") {
			t.Fatalf("check = %#v, want failed error", check)
		}
	})

	t.Run("timer inactive and disabled", func(t *testing.T) {
		client := newTestClient(t, "systemd 255", nil, map[string]unitResponse{
			systemd.UnitBackupTimer: {stdout: "ActiveState=inactive\n" +
				"LoadState=loaded\nUnitFileState=disabled\n"},
		})
		check := checkUnits(context.Background(), client, DefaultCommandTimeout, true)
		for _, want := range []string{
			"tes3mp-easy-backup.timer is not active (inactive)",
			"tes3mp-easy-backup.timer is not enabled (disabled)",
		} {
			if !strings.Contains(check.Message, want) {
				t.Errorf("message = %q, want substring %q", check.Message, want)
			}
		}
		if check.Status != StatusError {
			t.Fatalf("status = %q, want error", check.Status)
		}
	})

	t.Run("show error", func(t *testing.T) {
		client := newTestClient(t, "systemd 255", nil, map[string]unitResponse{
			systemd.UnitTES3MPService: {err: errors.New("bus unavailable")},
		})
		check := checkUnits(context.Background(), client, DefaultCommandTimeout, true)
		if check.Status != StatusError ||
			!strings.Contains(check.Message, "tes3mp.service:") ||
			!strings.Contains(check.Message, "bus unavailable") {
			t.Fatalf("check = %#v, want show error", check)
		}
	})
}

func TestCheckDisk(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		err    error
		status Status
	}{
		{name: "plenty", bytes: 2 << 30, status: StatusOK},
		{name: "exactly warn threshold", bytes: 1 << 30, status: StatusOK},
		{name: "below warn", bytes: 1<<30 - 1, status: StatusWarn},
		{name: "exactly min", bytes: 512 << 20, status: StatusWarn},
		{name: "below min", bytes: 512<<20 - 1, status: StatusError},
		{name: "tiny", bytes: 100 << 20, status: StatusError},
		{name: "probe error", err: errors.New("statfs failed"), status: StatusError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := checkDisk("/tes3mp-easy", fixedFree(tt.bytes, tt.err))
			if check.Status != tt.status {
				t.Fatalf("check = %#v, want status %q", check, tt.status)
			}
			if check.Status == StatusOK && check.Message == "" {
				t.Fatal("ok disk check should report free space")
			}
		})
	}

	t.Run("empty root", func(t *testing.T) {
		check := checkDisk("", fixedFree(1<<30, nil))
		if check.Status != StatusError {
			t.Fatalf("check = %#v, want error", check)
		}
	})
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{bytes: 2 << 30, want: "2.0 GiB"},
		{bytes: 512 << 20, want: "512.0 MiB"},
		{bytes: 1 << 10, want: "1.0 KiB"},
		{bytes: 10, want: "10 B"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.bytes); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
