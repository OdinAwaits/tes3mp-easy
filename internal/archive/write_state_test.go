package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

var writeStateTime = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

type stateWriteFixture struct {
	source  StateSource
	modsDir string
	current Current
}

func newStateWriteFixture(t *testing.T) stateWriteFixture {
	t.Helper()

	modsDir, current := modsDirWith(t, "mods")
	root := t.TempDir()
	players := filepath.Join(root, "players")
	world := filepath.Join(root, "world")
	configs := filepath.Join(root, "configs")
	required := filepath.Join(root, "requiredDataFiles.json")
	currentFile := filepath.Join(root, "current.txt")

	for _, dir := range []string{players, world, configs} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range stateWorldDirs {
		if err := os.MkdirAll(filepath.Join(world, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range stateConfigFiles {
		path := filepath.Join(configs, name)
		if err := os.WriteFile(path, []byte(name+"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, writeStateTime, writeStateTime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(required,
		[]byte(requiredJSON(baseRequiredEntries()...)), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentFile,
		[]byte(currentLine(current.SHA256, current.Name)), 0o640); err != nil {
		t.Fatal(err)
	}

	return stateWriteFixture{
		source: StateSource{
			PlayersDir:        players,
			WorldDir:          world,
			ConfigsDir:        configs,
			RequiredDataFiles: required,
			CurrentFile:       currentFile,
		},
		modsDir: modsDir,
		current: current,
	}
}

func writeStateFixtureFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, writeStateTime, writeStateTime); err != nil {
		t.Fatal(err)
	}
}

func writeStateFixtureArchive(t *testing.T, fixture stateWriteFixture) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := WriteState(&buf, fixture.source); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	return buf.Bytes()
}

func stateArchiveHeaders(t *testing.T, data []byte) []*tar.Header {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var headers []*tar.Header
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return headers
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		headers = append(headers, header)
	}
}

func TestWriteStateDeterministicLayout(t *testing.T) {
	fixture := newStateWriteFixture(t)
	writeStateFixtureFile(t, fixture.source.PlayersDir, "b.json", "{}\n")
	writeStateFixtureFile(t, fixture.source.PlayersDir, "a.json", "{}\n")
	for _, name := range stateWorldDirs {
		writeStateFixtureFile(t, filepath.Join(fixture.source.WorldDir, name),
			name+".json", "{}\n")
	}

	data := writeStateFixtureArchive(t, fixture)
	info, err := ValidateStateReader(bytes.NewReader(data), fixture.modsDir)
	if err != nil {
		t.Fatalf("ValidateStateReader: %v", err)
	}
	if info.Current != fixture.current {
		t.Fatalf("current = %+v, want %+v", info.Current, fixture.current)
	}
	if info.Players != 2 || info.WorldFiles != len(stateWorldDirs) ||
		info.ConfigFiles != len(stateConfigFiles) {
		t.Fatalf("info = %+v, want 2 players, %d world files and %d configs",
			info, len(stateWorldDirs), len(stateConfigFiles))
	}

	wantNames := []string{
		currentFileName,
		"configs/",
		"configs/config.lua",
		"configs/tes3mp-server-default.cfg",
		"configs/banlist.json",
		requiredDataFilesName,
		"players/",
		"players/a.json",
		"players/b.json",
		"world/",
		"world/cell/",
		"world/cell/cell.json",
		"world/world/",
		"world/world/world.json",
		"world/map/",
		"world/map/map.json",
		"world/recordstore/",
		"world/recordstore/recordstore.json",
		"world/custom/",
		"world/custom/custom.json",
	}
	headers := stateArchiveHeaders(t, data)
	gotNames := make([]string, 0, len(headers))
	for _, header := range headers {
		gotNames = append(gotNames, header.Name)
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			t.Fatalf("%s has non-portable ownership: %#v", header.Name, header)
		}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("entries = %v, want %v", gotNames, wantNames)
	}

	second := writeStateFixtureArchive(t, fixture)
	if !bytes.Equal(data, second) {
		t.Fatal("two writes with identical mtimes produced different archives")
	}
}

func TestWriteStateAcceptsEmptyPlayers(t *testing.T) {
	fixture := newStateWriteFixture(t)
	data := writeStateFixtureArchive(t, fixture)
	if _, err := ValidateStateReader(bytes.NewReader(data), fixture.modsDir); err != nil {
		t.Fatalf("ValidateStateReader: %v", err)
	}

	headers := stateArchiveHeaders(t, data)
	for _, name := range []string{"players/", "world/custom/"} {
		found := false
		for _, header := range headers {
			if header.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("archive is missing directory %s", name)
		}
	}
}

func TestWriteStateRejectsOversizedFile(t *testing.T) {
	fixture := newStateWriteFixture(t)
	path := filepath.Join(fixture.source.PlayersDir, "huge.dat")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxUncompressedSize + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	_, err = WriteState(&buf, fixture.source)
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("WriteState = %v, want ErrLimit", err)
	}
}

func TestWriteStateRejectsInvalidMetadata(t *testing.T) {
	fixture := newStateWriteFixture(t)
	if err := os.WriteFile(fixture.source.CurrentFile,
		[]byte("not-a-current-file\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	_, err := WriteState(&buf, fixture.source)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("WriteState = %v, want ErrInvalid", err)
	}
}

func TestWriteStateRejectsUnsafeEntries(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, fixture stateWriteFixture)
	}{
		{
			name: "symlink",
			setup: func(t *testing.T, fixture stateWriteFixture) {
				if err := os.Symlink("/etc/passwd",
					filepath.Join(fixture.source.PlayersDir, "evil")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "nested directory",
			setup: func(t *testing.T, fixture stateWriteFixture) {
				if err := os.Mkdir(
					filepath.Join(fixture.source.PlayersDir, "nested"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unexpected world directory",
			setup: func(t *testing.T, fixture stateWriteFixture) {
				if err := os.Mkdir(
					filepath.Join(fixture.source.WorldDir, "extra"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing metadata",
			setup: func(t *testing.T, fixture stateWriteFixture) {
				if err := os.Remove(fixture.source.CurrentFile); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newStateWriteFixture(t)
			tt.setup(t, fixture)
			var buf bytes.Buffer
			if _, err := WriteState(&buf, fixture.source); err == nil {
				t.Fatal("WriteState succeeded, want an error")
			}
		})
	}
}

type failingWriter struct {
	remaining int
	err       error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, w.err
	}
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, w.err
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestWriteStatePropagatesNoSpace(t *testing.T) {
	fixture := newStateWriteFixture(t)
	custom := filepath.Join(fixture.source.WorldDir, "custom")
	writeStateFixtureFile(t, custom, "large.bin",
		string(bytes.Repeat([]byte("x"), 1<<20)))

	writer := &failingWriter{remaining: 256, err: syscall.ENOSPC}
	_, err := WriteState(writer, fixture.source)
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("WriteState error = %v, want ENOSPC", err)
	}
}
