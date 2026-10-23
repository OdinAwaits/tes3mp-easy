//go:build !race

package fault

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

const (
	interruptSourceSize = 128 << 20
	interruptPartSize   = 1 << 20
)

func TestBackupsCreateKilledMidWriteLeavesPart(t *testing.T) {
	root, sourcePath := interruptStateRoot(t)
	sourceBefore := fileSHA256(t, sourcePath)
	stateDir := filepath.Join(root, "backups", "state")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd, stdout, stderr := command(ctx, root, nil,
		"--json", "backups", "create", "--type", "state")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start backups create: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	partPath, found, waitErr := waitForPart(stateDir, interruptPartSize, done)
	if waitErr != nil || !found {
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("backup did not reach an interrupted state: found=%v err=%v; "+
			"stdout=%q stderr=%q", found, waitErr, stdout.String(), stderr.String())
	}

	if err := cmd.Process.Kill(); err != nil {
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("kill backups create: %v", err)
	}
	waitErr = <-done
	if waitErr == nil {
		t.Fatal("backups create exited successfully after SIGKILL")
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		t.Fatalf("wait after SIGKILL = %v, want *exec.ExitError", waitErr)
	}

	info, err := os.Stat(partPath)
	if err != nil {
		t.Fatalf("partial archive disappeared after SIGKILL: %v", err)
	}
	if info.Size() < interruptPartSize {
		t.Fatalf("partial archive size = %d, want at least %d",
			info.Size(), interruptPartSize)
	}
	assertNoStateArchives(t, stateDir)

	if got := fileSHA256(t, sourcePath); got != sourceBefore {
		t.Fatalf("live state changed after SIGKILL: sha256=%s, want %s",
			got, sourceBefore)
	}

	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, time.Second)
	if err != nil {
		t.Fatalf("lock was not released after SIGKILL: %v", err)
	}
	if err := held.Close(); err != nil {
		t.Fatalf("release lock: %v", err)
	}

	res := runRoot(t, root, nil, "--json", "backups", "create", "--type", "state")
	res.requireExit(t, 0)
	env := res.decodeEnvelope(t)
	if !env.OK || env.Error != nil {
		t.Fatalf("recovery backup failed: %+v; stderr=%q", env, res.Stderr)
	}
	var data struct {
		Archive string `json:"archive"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode recovery backup: %v; data=%s", err, env.Data)
	}
	if data.Archive == "" {
		t.Fatalf("recovery backup returned no archive: %s", env.Data)
	}

	matches, err := filepath.Glob(filepath.Join(stateDir, "*.part"))
	if err != nil {
		t.Fatalf("glob parts after recovery: %v", err)
	}
	if len(matches) > 1 {
		t.Fatalf("recovery backup left %d .part files, want at most 1",
			len(matches))
	}
	finalPath := filepath.Join(stateDir, data.Archive)
	if _, err := os.Stat(finalPath + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery backup left its own .part: %v", err)
	}
	if _, _, err := archive.ValidateStateFileWithSize(finalPath,
		filepath.Join(root, "backups", "mods")); err != nil {
		t.Fatalf("recovery archive is invalid: %v", err)
	}
	if got := fileSHA256(t, sourcePath); got != sourceBefore {
		t.Fatalf("live state changed after recovery: sha256=%s, want %s",
			got, sourceBefore)
	}
}

func interruptStateRoot(t *testing.T) (root, sourcePath string) {
	t.Helper()

	root = t.TempDir()
	for _, dir := range []string{
		"players",
		"world",
		"mods/plugins",
		"backups/mods",
		"backups/state",
		".locks",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	worldDirs := []string{"cell", "world", "map", "recordstore", "custom"}
	for _, name := range worldDirs {
		if err := os.MkdirAll(filepath.Join(root, "world", name), 0o750); err != nil {
			t.Fatalf("create world/%s: %v", name, err)
		}
	}

	mods := fixtures.DefaultModsInit()
	mods.Install(t, filepath.Join(root, "backups", "mods"))
	fixtures.WriteFile(t,
		filepath.Join(root, "mods", "plugins", fixtures.MetadataName),
		string(mods.RequiredJSON()))
	fixtures.WriteFile(t, filepath.Join(root, "players", "player.json"), "{}\n")
	for _, name := range worldDirs {
		fixtures.WriteFile(t,
			filepath.Join(root, "world", name, name+".json"), "{}\n")
	}

	sourcePath = filepath.Join(root, "world", "custom", "large.bin")
	writePseudoRandomFile(t, sourcePath, interruptSourceSize)
	return root, sourcePath
}

func assertNoStateArchives(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".gz" {
			t.Fatalf("final archive %s exists after SIGKILL", entry.Name())
		}
	}
}

func writePseudoRandomFile(t *testing.T, path string, size int64) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	reader := &xorshiftReader{state: 0x9E3779B97F4A7C15}
	if _, err := io.CopyN(file, reader, size); err != nil {
		_ = file.Close()
		t.Fatalf("write %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

type xorshiftReader struct {
	state uint64
}

func (r *xorshiftReader) Read(p []byte) (int, error) {
	for i := 0; i+8 <= len(p); i += 8 {
		r.state ^= r.state >> 12
		r.state ^= r.state << 25
		r.state ^= r.state >> 27
		binary.LittleEndian.PutUint64(p[i:i+8], r.state*2685821657736338717)
	}
	if remainder := len(p) % 8; remainder > 0 {
		r.state ^= r.state >> 12
		r.state ^= r.state << 25
		r.state ^= r.state >> 27
		var tail [8]byte
		binary.LittleEndian.PutUint64(tail[:], r.state*2685821657736338717)
		copy(p[len(p)-remainder:], tail[:remainder])
	}
	if len(p) > 0 {
		return len(p), nil
	}
	return 0, fmt.Errorf("xorshift: empty buffer")
}
