package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestCheckCgroup(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		client := newTestClient(t, "systemd 255", nil, nil)
		check := checkCgroup(context.Background(), client, time.Second, true)
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("wrong limit", func(t *testing.T) {
		runner := systemdtest.New()
		for _, limit := range cgroupLimits {
			high, max := limit.high, limit.max
			if limit.unit == systemd.UnitTES3MPService {
				high++
			}
			runner.Respond("systemctl", []string{
				"show", limit.unit, "--property=MemoryHigh,MemoryMax",
			}, fmt.Sprintf("MemoryHigh=%d\nMemoryMax=%d\n", high, max), nil)
		}
		check := checkCgroup(context.Background(), systemd.New(runner),
			time.Second, true)
		if check.Status != StatusError ||
			!strings.Contains(check.Message, "tes3mp.service MemoryHigh") {
			t.Fatalf("check = %#v, want limit error", check)
		}
	})

	t.Run("systemd unavailable", func(t *testing.T) {
		check := checkCgroup(context.Background(), nil, time.Second, false)
		if check.Status != StatusError || !strings.Contains(check.Message, "unavailable") {
			t.Fatalf("check = %#v, want unavailable error", check)
		}
	})
}

func TestCheckFilesystems(t *testing.T) {
	root := writeDeepRoot(t)
	if check := checkFilesystems(root, func(string) (uint64, error) {
		return 42, nil
	}); check.Status != StatusOK {
		t.Fatalf("same device check = %#v, want ok", check)
	}

	check := checkFilesystems(root, func(path string) (uint64, error) {
		if strings.Contains(path, "uploads") {
			return 1, nil
		}
		return 2, nil
	})
	if check.Status != StatusError || !strings.Contains(check.Message, "different") {
		t.Fatalf("different device check = %#v, want error", check)
	}

	check = checkFilesystems(root, func(string) (uint64, error) {
		return 0, fmt.Errorf("stat failed")
	})
	if check.Status != StatusError {
		t.Fatalf("probe error check = %#v, want error", check)
	}
}

func TestCheckBudgets(t *testing.T) {
	root := writeDeepRoot(t)
	check := checkBudgets(root)
	if check.Status != StatusOK {
		t.Fatalf("check = %#v, want ok", check)
	}

	oversize := filepath.Join(root, "backups", "state", "oversize.bin")
	file, err := os.Create(oversize)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(1<<30 + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	check = checkBudgets(root)
	if check.Status != StatusWarn || !strings.Contains(check.Message, "1.0 GiB") {
		t.Fatalf("check = %#v, want state budget warning", check)
	}
}

func TestCheckCurrent(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		check := checkCurrent(writeDeepRoot(t))
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("missing", func(t *testing.T) {
		root := writeDeepRoot(t)
		if err := os.Remove(filepath.Join(root, "backups", "mods", "current.txt")); err != nil {
			t.Fatal(err)
		}
		check := checkCurrent(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "current.txt") {
			t.Fatalf("check = %#v, want missing current error", check)
		}
	})

	t.Run("sha mismatch", func(t *testing.T) {
		root := writeDeepRoot(t)
		path := filepath.Join(root, "backups", "mods", "current.txt")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(data))
		bad := strings.Repeat("0", 64) + " " + fields[1] + "\n"
		fixtures.WriteFile(t, path, bad)
		check := checkCurrent(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "sha256") {
			t.Fatalf("check = %#v, want sha error", check)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		root := writeDeepRoot(t)
		path := filepath.Join(root, "backups", "mods", "current.txt")
		fixtures.WriteFile(t, path, strings.Repeat("0", 64)+"\n")
		check := checkCurrent(root)
		if check.Status != StatusError {
			t.Fatalf("check = %#v, want malformed current error", check)
		}
	})
}

func TestCheckDataFiles(t *testing.T) {
	t.Run("base only", func(t *testing.T) {
		check := checkDataFiles(writeDeepRoot(t))
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("unlisted plugin", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t,
			filepath.Join(root, "mods", "plugins", "extra.esp"), "synthetic\n")
		check := checkDataFiles(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "not listed") {
			t.Fatalf("check = %#v, want unlisted plugin error", check)
		}
	})

	t.Run("missing base esm", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t,
			filepath.Join(root, "mods", "plugins", requiredDataFilesName),
			`[{"Morrowind.esm":[]},{"Tribunal.esm":[]}]`)
		check := checkDataFiles(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "Bloodmoon.esm") {
			t.Fatalf("check = %#v, want missing base ESM error", check)
		}
	})

	t.Run("crc mismatch", func(t *testing.T) {
		root := writeDeepRoot(t)
		body := "synthetic plugin\n"
		fixtures.WriteFile(t,
			filepath.Join(root, "mods", "plugins", "mod.esp"), body)
		required := fmt.Sprintf(
			`[{"Morrowind.esm":[]},{"Tribunal.esm":[]},{"Bloodmoon.esm":[]},{"mod.esp":["0x00000000"]}]`)
		fixtures.WriteFile(t,
			filepath.Join(root, "mods", "plugins", requiredDataFilesName), required)
		check := checkDataFiles(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "CRC") {
			t.Fatalf("check = %#v, want CRC error", check)
		}
	})
}

func TestCheckArchives(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		check := checkArchives(writeDeepRoot(t))
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("empty", func(t *testing.T) {
		root := writeDeepRoot(t)
		dir := filepath.Join(root, "backups", "state")
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				t.Fatal(err)
			}
		}
		check := checkArchives(root)
		if check.Status != StatusWarn || !strings.Contains(check.Message, "no state") {
			t.Fatalf("check = %#v, want empty warning", check)
		}
	})

	t.Run("corrupt newest", func(t *testing.T) {
		root := writeDeepRoot(t)
		name := fixtures.StateName(fixtures.DefaultTime.Add(time.Minute))
		fixtures.WriteFile(t,
			filepath.Join(root, "backups", "state", name), "not a tar.gz")
		check := checkArchives(root)
		if check.Status != StatusError || !strings.Contains(check.Message, name) {
			t.Fatalf("check = %#v, want corrupt archive error", check)
		}
	})
}

func TestCheckConfigs(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		check := checkConfigs(writeDeepRoot(t))
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("bad ini", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t,
			filepath.Join(root, "configs", "tes3mp-server-default.cfg"),
			"[General]\nthis is not a setting\n")
		check := checkConfigs(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "key=value") {
			t.Fatalf("check = %#v, want INI error", check)
		}
	})

	t.Run("bad lua", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t,
			filepath.Join(root, "configs", "config.lua"),
			"config.loginTime = \"unterminated\n")
		check := checkConfigs(root)
		if check.Status != StatusError || !strings.Contains(check.Message, "unterminated") {
			t.Fatalf("check = %#v, want Lua error", check)
		}
	})

	t.Run("custom scripts mismatch", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t,
			filepath.Join(root, "mods", "scripts", "new_script.lua"), "print('x')\n")
		check := checkConfigs(root)
		if check.Status != StatusError ||
			!strings.Contains(check.Message, "does not match") {
			t.Fatalf("check = %#v, want custom scripts error", check)
		}
	})

	t.Run("custom scripts matches", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t,
			filepath.Join(root, "mods", "scripts", "new_script.lua"), "print('x')\n")
		fixtures.WriteFile(t,
			filepath.Join(root, "configs", "customScripts.lua"),
			"require(\"custom.new_script\")\n")
		check := checkConfigs(root)
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})
}

func TestCheckCleanup(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		check := checkCleanup(writeDeepRoot(t))
		if check.Status != StatusOK {
			t.Fatalf("check = %#v, want ok", check)
		}
	})

	t.Run("leftovers", func(t *testing.T) {
		root := writeDeepRoot(t)
		fixtures.WriteFile(t, filepath.Join(root, ".staging", "tmp"), "x")
		fixtures.WriteFile(t, filepath.Join(root, ".locks", "old.lock"), "x")
		fixtures.WriteFile(t,
			filepath.Join(root, "backups", "state", "state.part"), "x")
		if err := os.Mkdir(filepath.Join(root, "players.old-1"), 0o755); err != nil {
			t.Fatal(err)
		}
		check := checkCleanup(root)
		if check.Status != StatusError {
			t.Fatalf("check = %#v, want error", check)
		}
		for _, want := range []string{
			".staging", ".locks/old.lock", "state.part", "players.old-1",
		} {
			if !strings.Contains(check.Message, want) {
				t.Errorf("message = %q, want substring %q", check.Message, want)
			}
		}
	})
}

func TestCheckJournaldConfig(t *testing.T) {
	_, _, path := writeDeepHostFiles(t)
	if check := checkJournaldConfig(path); check.Status != StatusOK {
		t.Fatalf("check = %#v, want ok", check)
	}

	bad := filepath.Join(t.TempDir(), "journald.conf")
	fixtures.WriteFile(t, bad, "[Journal]\nSystemMaxUse=50M\nRuntimeMaxUse=32M\n")
	check := checkJournaldConfig(bad)
	if check.Status != StatusError || !strings.Contains(check.Message, "SystemMaxUse") {
		t.Fatalf("check = %#v, want cap error", check)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	check = checkJournaldConfig(missing)
	if check.Status != StatusError {
		t.Fatalf("check = %#v, want missing error", check)
	}
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		raw  string
		want int64
		bad  bool
	}{
		{raw: "402653184", want: 384 << 20},
		{raw: "384M", want: 384 << 20},
		{raw: "1G", want: 1 << 30},
		{raw: "", bad: true},
		{raw: "infinity", bad: true},
		{raw: "123X", bad: true},
	}
	for _, tt := range tests {
		got, err := parseMemoryLimit(tt.raw)
		if (err != nil) != tt.bad {
			t.Errorf("parseMemoryLimit(%q) error = %v, want bad %v", tt.raw, err, tt.bad)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("parseMemoryLimit(%q) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestLuaConfigValidation(t *testing.T) {
	valid := []byte("config = {}\nconfig.loginTime = 60\nconfig.gameMode = \"Default\"\n" +
		"config.gameSettings = {\n  death = true,\n}\n--[[ comment ]] for i = 1, 2 do end\n")
	if err := validateLuaConfig(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for _, body := range []string{
		"config.loginTime = \"bad\n",
		"config.loginTime =\n",
		"config.gameSettings = {\n",
	} {
		if err := validateLuaConfig([]byte(body)); err == nil {
			t.Errorf("validateLuaConfig(%q) error = nil", body)
		}
	}
}

func TestParseINI(t *testing.T) {
	values, err := parseINI([]byte("\ufeff[General]\nport = 25565\npassword =\n"))
	if err != nil {
		t.Fatalf("parseINI: %v", err)
	}
	if values["General"]["port"] != "25565" || values["General"]["password"] != "" {
		t.Fatalf("values = %#v", values)
	}
	if _, err := parseINI([]byte("[General\n")); err == nil {
		t.Fatal("unterminated section error = nil")
	}
	if _, err := parseINI([]byte("key=value\n")); err == nil {
		t.Fatal("key outside section error = nil")
	}
}

func TestArchiveSamples(t *testing.T) {
	if got := archiveSamples(2); len(got) != 2 {
		t.Fatalf("archiveSamples(2) = %v", got)
	}
	if got := archiveSamples(5); fmt.Sprint(got) != "[0 2 4]" {
		t.Fatalf("archiveSamples(5) = %v", got)
	}
}
