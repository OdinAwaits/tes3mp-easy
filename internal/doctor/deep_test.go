package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func writeDeepRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, spec := range ownershipDirs {
		path := root
		if spec.rel != "" {
			path = filepath.Join(root, spec.rel)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
	}
	for _, spec := range ownershipDirs {
		path := root
		if spec.rel != "" {
			path = filepath.Join(root, spec.rel)
		}
		if err := os.Chmod(path, chmodMode(spec.mode)); err != nil {
			t.Fatalf("chmod %s: %v", path, err)
		}
	}

	fixtures.WriteConfigs(t, filepath.Join(root, "configs"))
	fixtures.WriteFile(t, filepath.Join(root, "configs", "customScripts.lua"), "")

	mods := fixtures.DefaultModsInit()
	manifest := mods.Install(t, filepath.Join(root, "backups", "mods"))
	if err := os.Chmod(filepath.Join(root, "backups", "mods", "current.txt"),
		0o640); err != nil {
		t.Fatalf("chmod current.txt: %v", err)
	}
	fixtures.WriteFile(t,
		filepath.Join(root, "mods", "plugins", requiredDataFilesName),
		string(mods.RequiredJSON()))
	fixtures.DefaultState(manifest).Write(t,
		filepath.Join(root, "backups", "state", fixtures.StateName(fixtures.DefaultTime)))
	return root
}

func chmodMode(mode uint32) os.FileMode {
	out := os.FileMode(mode & 0o777)
	if mode&0o2000 != 0 {
		out |= os.ModeSetgid
	}
	if mode&0o1000 != 0 {
		out |= os.ModeSticky
	}
	return out
}

func writeRuntimeRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, required := range runtimeFiles {
		path := filepath.Join(root, required.rel)
		if required.dir {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("create %s: %v", path, err)
			}
			continue
		}
		mode := os.FileMode(0o644)
		if required.executable {
			mode = 0o755
		}
		fixtures.WriteFile(t, path, "synthetic runtime\n")
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("chmod %s: %v", path, err)
		}
	}
	return root
}

func writeDeepHostFiles(t *testing.T) (procRoot, sysctlPath, journaldPath string) {
	t.Helper()

	procRoot = t.TempDir()
	fixtures.WriteFile(t, filepath.Join(procRoot, "meminfo"),
		"MemTotal: 1048576 kB\nSwapTotal: 1048576 kB\n")
	fixtures.WriteFile(t, filepath.Join(procRoot, "sys/vm/swappiness"), "10\n")

	sysctlPath = filepath.Join(t.TempDir(), "99-tes3mp-easy.conf")
	fixtures.WriteFile(t, sysctlPath, "# synthetic\nvm.swappiness=10\n")

	journaldPath = filepath.Join(t.TempDir(), "tes3mp-easy.conf")
	fixtures.WriteFile(t, journaldPath,
		"[Journal]\nSystemMaxUse=100M\nRuntimeMaxUse=32M\n")
	return procRoot, sysctlPath, journaldPath
}

func TestCheckOwnership(t *testing.T) {
	t.Run("ok modes", func(t *testing.T) {
		check := checkOwnership(writeDeepRoot(t), defaultOwnerNames, true)
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("wrong mode", func(t *testing.T) {
		root := writeDeepRoot(t)
		path := filepath.Join(root, "configs")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		check := checkOwnership(root, defaultOwnerNames, true)
		if check.Status != StatusError || !strings.Contains(check.Message, "configs/ mode") {
			t.Fatalf("check = %#v, want mode error", check)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		root := writeDeepRoot(t)
		path := filepath.Join(root, "mods")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "players"), path); err != nil {
			t.Fatal(err)
		}
		check := checkOwnership(root, defaultOwnerNames, true)
		if check.Status != StatusError || !strings.Contains(check.Message, "symlink") {
			t.Fatalf("check = %#v, want symlink error", check)
		}
	})

	t.Run("owner mismatch", func(t *testing.T) {
		info, err := os.Stat(writeDeepRoot(t))
		if err != nil {
			t.Fatal(err)
		}
		lookup := newOwnerLookup(func(uint32, uint32) (string, string, error) {
			return "alice", "staff", nil
		}, false)
		problem := lookup("root", "root", "root", info)
		if !strings.Contains(problem, "want root:root") {
			t.Fatalf("problem = %q, want owner mismatch", problem)
		}
	})
}

func TestCheckJournald(t *testing.T) {
	client := newTestClient(t, "systemd 255", nil, nil)
	check := checkJournald(context.Background(), client, time.Second, true)
	if check.Status != StatusOK {
		t.Fatalf("check = %#v, want ok", check)
	}

	check = checkJournald(context.Background(), nil, time.Second, false)
	if check.Status != StatusError || !strings.Contains(check.Message, "unavailable") {
		t.Fatalf("check = %#v, want unavailable error", check)
	}
}

func TestCheckRuntime(t *testing.T) {
	files := func(t *testing.T) (string, func(context.Context, string, ...string) ([]byte, error)) {
		t.Helper()
		return writeRuntimeRoot(t), func(_ context.Context, name string, _ ...string) ([]byte, error) {
			switch name {
			case "ldconfig":
				return []byte("libluajit-5.1.so.2\n"), nil
			case "ldd":
				return []byte("libc.so.6 => /lib/libc.so.6\n"), nil
			default:
				return nil, fmt.Errorf("unexpected %s", name)
			}
		}
	}

	t.Run("ok", func(t *testing.T) {
		root, run := files(t)
		check := checkRuntime(context.Background(), root, time.Second, run)
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("ldconfig missing luajit", func(t *testing.T) {
		root, _ := files(t)
		run := func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if name == "ldconfig" {
				return []byte("libc.so.6\n"), nil
			}
			return []byte("ok\n"), nil
		}
		check := checkRuntime(context.Background(), root, time.Second, run)
		if check.Status != StatusError || !strings.Contains(check.Message, "libluajit") {
			t.Fatalf("check = %#v, want luajit error", check)
		}
	})

	t.Run("ldd not found", func(t *testing.T) {
		root, _ := files(t)
		run := func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if name == "ldconfig" {
				return []byte("libluajit-5.1.so.2\n"), nil
			}
			return []byte("liblua.so.5 => not found\n"), nil
		}
		check := checkRuntime(context.Background(), root, time.Second, run)
		if check.Status != StatusError || !strings.Contains(check.Message, "missing") {
			t.Fatalf("check = %#v, want ldd error", check)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		root, run := files(t)
		if err := os.Remove(filepath.Join(root, "openmw.cfg")); err != nil {
			t.Fatal(err)
		}
		check := checkRuntime(context.Background(), root, time.Second, run)
		if check.Status != StatusError || !strings.Contains(check.Message, "openmw.cfg") {
			t.Fatalf("check = %#v, want missing file error", check)
		}
	})
}

func TestCheckMemory(t *testing.T) {
	healthy := func(t *testing.T) (string, string) {
		t.Helper()
		proc, sysctl, _ := writeDeepHostFiles(t)
		return proc, sysctl
	}

	t.Run("ok", func(t *testing.T) {
		proc, sysctl := healthy(t)
		check := checkMemory(proc, sysctl)
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("low ram and swap", func(t *testing.T) {
		proc, sysctl := healthy(t)
		fixtures.WriteFile(t, filepath.Join(proc, "meminfo"),
			"MemTotal: 400000 kB\nSwapTotal: 400000 kB\n")
		check := checkMemory(proc, sysctl)
		if check.Status != StatusError ||
			!strings.Contains(check.Message, "RAM") ||
			!strings.Contains(check.Message, "swap") {
			t.Fatalf("check = %#v, want RAM and swap errors", check)
		}
	})

	t.Run("512 MiB swap file", func(t *testing.T) {
		proc, sysctl := healthy(t)
		fixtures.WriteFile(t, filepath.Join(proc, "meminfo"),
			"MemTotal: 1048576 kB\nSwapTotal: 524284 kB\n")
		check := checkMemory(proc, sysctl)
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("wrong swappiness", func(t *testing.T) {
		proc, sysctl := healthy(t)
		fixtures.WriteFile(t, sysctl, "vm.swappiness=60\n")
		check := checkMemory(proc, sysctl)
		if check.Status != StatusError || !strings.Contains(check.Message, "swappiness") {
			t.Fatalf("check = %#v, want swappiness error", check)
		}
	})
}

func deepOptions(t *testing.T) Options {
	t.Helper()

	procRoot, sysctlPath, journaldPath := writeDeepHostFiles(t)
	return Options{
		Root:           writeDeepRoot(t),
		OSReleasePath:  writeOSRelease(t, "ID=debian\nVERSION_ID=\"12\"\n"),
		ProcRoot:       procRoot,
		SysctlPath:     sysctlPath,
		JournaldConfig: journaldPath,
		RuntimeRoot:    writeRuntimeRoot(t),
		CommandTimeout: time.Second,
		Systemd:        newTestClient(t, "systemd 255", nil, nil),
		Uname:          fixedUname("x86_64", nil),
		DiskFree:       fixedFree(2<<30, nil),
		DeviceOf:       func(string) (uint64, error) { return 42, nil },
		RunCommand: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			switch name {
			case "ldconfig":
				return []byte("libluajit-5.1.so.2 (libc6,x86-64) => /lib/libluajit-5.1.so.2\n"), nil
			case "ldd":
				return []byte("linux-vdso.so.1\nlibluajit-5.1.so.2 => /lib/libluajit-5.1.so.2\n"), nil
			default:
				return nil, fmt.Errorf("unexpected command %s", name)
			}
		},
		SkipOwnership: true,
	}
}
