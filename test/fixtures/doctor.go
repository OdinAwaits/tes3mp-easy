package fixtures

import (
	"os"
	"path/filepath"
	"testing"
)

// DoctorRoot builds a synthetic /tes3mp-easy root that passes every doctor
// ownership, permission, runtime and backup check, plus the environment
// overrides that point doctor at synthetic OS and host files. Owner checks are
// skipped because a temp directory cannot be chowned in unprivileged tests.
func DoctorRoot(t testing.TB) (string, map[string]string) {
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
			t.Fatalf("fixtures: create %s: %v", path, err)
		}
	}
	for _, dir := range dirs {
		path := root
		if dir.path != "" {
			path = filepath.Join(root, dir.path)
		}
		if err := os.Chmod(path, doctorMode(dir.mode)); err != nil {
			t.Fatalf("fixtures: chmod %s: %v", path, err)
		}
	}

	WriteConfigs(t, filepath.Join(root, "configs"))
	WriteFile(t, filepath.Join(root, "configs", "customScripts.lua"), "")

	mods := DefaultModsInit()
	manifest := mods.Install(t, filepath.Join(root, "backups", "mods"))
	if err := os.Chmod(filepath.Join(root, "backups", "mods", "current.txt"),
		0o640); err != nil {
		t.Fatalf("fixtures: chmod current.txt: %v", err)
	}
	WriteFile(t,
		filepath.Join(root, "mods", "plugins", "requiredDataFiles.json"),
		string(mods.RequiredJSON()))
	DefaultState(manifest).Write(t,
		filepath.Join(root, "backups", "state", StateName(DefaultTime)))

	runtimeRoot := DoctorRuntimeRoot(t)
	procRoot, sysctlPath, journaldPath := DoctorHost(t)
	env := map[string]string{
		"TES3MP_EASY_OS_RELEASE":         OSReleaseFile(t, "ID=debian\nVERSION_ID=\"12\"\n"),
		"TES3MP_EASY_PROC_ROOT":          procRoot,
		"TES3MP_EASY_SYSCTL_CONF":        sysctlPath,
		"TES3MP_EASY_JOURNALD_CONF":      journaldPath,
		"TES3MP_EASY_RUNTIME_ROOT":       runtimeRoot,
		"TES3MP_EASY_DOCTOR_SKIP_OWNERS": "1",
	}
	return root, env
}

// DoctorRuntimeRoot creates the minimum synthetic TES3MP runtime that the
// doctor runtime check expects.
func DoctorRuntimeRoot(t testing.TB) string {
	t.Helper()

	root := t.TempDir()
	files := []struct {
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
	for _, file := range files {
		path := filepath.Join(root, file.path)
		mode := os.FileMode(0o644)
		if file.executable {
			mode = 0o755
		}
		WriteFile(t, path, "synthetic runtime\n")
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("fixtures: chmod %s: %v", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatalf("fixtures: create lib: %v", err)
	}
	return root
}

// DoctorHost creates synthetic /proc, sysctl and journald configuration files.
func DoctorHost(t testing.TB) (procRoot, sysctlPath, journaldPath string) {
	t.Helper()

	procRoot = t.TempDir()
	WriteFile(t, filepath.Join(procRoot, "meminfo"),
		"MemTotal: 1048576 kB\nSwapTotal: 1048576 kB\n")
	WriteFile(t, filepath.Join(procRoot, "sys/vm/swappiness"), "10\n")

	sysctlPath = filepath.Join(t.TempDir(), "99-tes3mp-easy.conf")
	WriteFile(t, sysctlPath, "vm.swappiness=10\n")

	journaldPath = filepath.Join(t.TempDir(), "tes3mp-easy.conf")
	WriteFile(t, journaldPath,
		"[Journal]\nSystemMaxUse=100M\nRuntimeMaxUse=32M\n")
	return procRoot, sysctlPath, journaldPath
}

// OSReleaseFile writes an os-release body and returns its path.
func OSReleaseFile(t testing.TB, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("fixtures: write os-release: %v", err)
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
