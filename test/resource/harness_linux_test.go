//go:build linux && !race

package resource

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/retention"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

const (
	memoryLimit    = int64(64 << 20)
	maxRSS         = uint64(128 << 20)
	maxRSSGrowth   = uint64(32 << 20)
	measureTimeout = 5 * time.Minute
)

type measurement struct {
	ExitCode   int
	Stdout     string
	Stderr     string
	MaxRSS     uint64
	CgroupPeak uint64
	CgroupUsed bool
	Elapsed    time.Duration
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *envelopeError  `json:"error"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func runMeasured(t *testing.T, root string, cg *cgroup, args ...string,
) measurement {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), measureTimeout)
	defer cancel()

	full := append([]string{"--root", root}, args...)
	cmd := exec.CommandContext(ctx, opsBinary, full...)
	cmd.Dir = root
	cmd.Env = environment(map[string]string{
		"GOMAXPROCS": "1",
		"GOMEMLIMIT": "64MiB",
		"TZ":         "UTC",
	})
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start ops %v: %v", args, err)
	}
	cgroupUsed := false
	if cg != nil {
		if err := cg.attach(cmd.Process.Pid); err != nil {
			t.Logf("cgroup attach unavailable: %v", err)
		} else {
			cgroupUsed = true
		}
	}
	err := cmd.Wait()
	elapsed := time.Since(started)
	if ctx.Err() != nil {
		t.Fatalf("ops %v timed out after %v", args, measureTimeout)
	}

	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run ops %v: %v", args, err)
		}
		exitCode = exitErr.ExitCode()
	}

	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Fatalf("rusage type = %T, want *syscall.Rusage",
			cmd.ProcessState.SysUsage())
	}

	m := measurement{
		ExitCode:   exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		MaxRSS:     uint64(usage.Maxrss) * 1024,
		CgroupUsed: cgroupUsed,
		Elapsed:    elapsed,
	}
	if cgroupUsed {
		m.CgroupPeak = cg.peak()
	}
	return m
}

func (m measurement) requireSuccess(t *testing.T) {
	t.Helper()
	if m.ExitCode != 0 {
		t.Fatalf("exit = %d; stdout=%q stderr=%q",
			m.ExitCode, m.Stdout, m.Stderr)
	}
	if strings.TrimSpace(m.Stdout) == "" {
		t.Fatalf("stdout is empty; stderr=%q", m.Stderr)
	}
	var env envelope
	if err := json.Unmarshal([]byte(m.Stdout), &env); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%q", err, m.Stdout)
	}
	if !env.OK || env.Error != nil {
		t.Fatalf("envelope = %+v; stdout=%q", env, m.Stdout)
	}
}

func checkMemory(t *testing.T, label string, small, large measurement) {
	t.Helper()
	t.Logf("%s: small maxrss=%s, large maxrss=%s, elapsed=%s, cgroup=%v",
		label, formatBytes(small.MaxRSS), formatBytes(large.MaxRSS),
		large.Elapsed.Round(time.Millisecond), large.CgroupUsed)

	if large.MaxRSS > maxRSS {
		t.Fatalf("%s: peak RSS=%s exceeds %s",
			label, formatBytes(large.MaxRSS), formatBytes(maxRSS))
	}
	if large.MaxRSS > small.MaxRSS+maxRSSGrowth {
		t.Fatalf("%s: peak RSS grew from %s to %s; want at most +%s",
			label, formatBytes(small.MaxRSS), formatBytes(large.MaxRSS),
			formatBytes(maxRSSGrowth))
	}
	if large.CgroupUsed {
		if large.CgroupPeak > uint64(memoryLimit) {
			t.Fatalf("%s: cgroup memory.peak=%s exceeds MemoryMax=%s",
				label, formatBytes(large.CgroupPeak), formatBytes(uint64(memoryLimit)))
		}
		t.Logf("%s: cgroup MemoryMax=%s, memory.peak=%s",
			label, formatBytes(uint64(memoryLimit)), formatBytes(large.CgroupPeak))
	} else {
		t.Logf("%s: cgroup v2 MemoryMax not applied; RSS + GOMEMLIMIT used", label)
	}
}

func environment(extra map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	for key, value := range extra {
		values[key] = value
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func formatBytes(value uint64) string {
	const mib = 1 << 20
	if value >= mib {
		return fmt.Sprintf("%.1fMiB", float64(value)/mib)
	}
	return fmt.Sprintf("%dB", value)
}

func requireSpace(t *testing.T, path string, need int64) {
	t.Helper()
	free, err := retention.DiskFree(path)
	if err != nil {
		t.Fatalf("disk free %s: %v", path, err)
	}
	if free < need {
		t.Skipf("not enough free space in %s: %s, want at least %s",
			path, formatBytes(uint64(free)), formatBytes(uint64(need)))
	}
}

// cgroup is a best-effort writable cgroup v2 leaf. It is only created when the
// host delegates cgroup writes to the test user; otherwise tests fall back to
// GOMEMLIMIT plus Maxrss and log that MemoryMax was not applied.
type cgroup struct {
	dir string
}

func newTestCgroup(t *testing.T) *cgroup {
	t.Helper()

	base := "/sys/fs/cgroup"
	if _, err := os.Stat(filepath.Join(base, "cgroup.controllers")); err != nil {
		t.Logf("cgroup v2 unavailable: %v", err)
		return nil
	}
	dir, err := os.MkdirTemp(base, "tes3mp-easy-resource-")
	if err != nil {
		t.Logf("cgroup v2 is not writable: %v", err)
		return nil
	}
	cg := &cgroup{dir: dir}
	if err := os.WriteFile(filepath.Join(dir, "memory.max"),
		[]byte(strconv.FormatInt(memoryLimit, 10)), 0o644); err != nil {
		t.Logf("cgroup memory controller unavailable: %v", err)
		_ = os.Remove(dir)
		return nil
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "cgroup.kill"), []byte("1"), 0o644)
		if err := os.Remove(dir); err != nil {
			t.Logf("remove cgroup %s: %v", dir, err)
		}
	})
	if err := os.WriteFile(filepath.Join(dir, "memory.swap.max"),
		[]byte("0"), 0o644); err != nil {
		t.Logf("cgroup swap limit unavailable: %v", err)
	}
	return cg
}

func (c *cgroup) attach(pid int) error {
	return os.WriteFile(filepath.Join(c.dir, "cgroup.procs"),
		[]byte(strconv.Itoa(pid)), 0o644)
}

func writeModsArchive(t *testing.T, path string, payloadSize int64) int64 {
	t.Helper()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	gz, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
	if err != nil {
		_ = file.Close()
		t.Fatalf("create gzip writer: %v", err)
	}
	tw := tar.NewWriter(gz)

	fail := func(format string, args ...any) {
		_ = tw.Close()
		_ = gz.Close()
		_ = file.Close()
		t.Fatalf(format, args...)
	}
	writeDir := func(name string) {
		header := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
			ModTime:  fixtures.DefaultTime,
		}
		if err := tw.WriteHeader(header); err != nil {
			fail("write %s: %v", name, err)
		}
	}
	writeBytes := func(name string, body []byte) {
		header := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(body)),
			ModTime:  fixtures.DefaultTime,
		}
		if err := tw.WriteHeader(header); err != nil {
			fail("write %s: %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			fail("write %s: %v", name, err)
		}
	}

	mods := fixtures.DefaultMods()
	mods.Plugins[0].CRCs = []string{}
	writeDir("plugins/")
	writeBytes("plugins/"+fixtures.MetadataName, mods.RequiredJSON())

	plugin := "plugins/" + mods.Plugins[0].Name
	header := &tar.Header{
		Name:     plugin,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     payloadSize,
		ModTime:  fixtures.DefaultTime,
	}
	if err := tw.WriteHeader(header); err != nil {
		fail("write %s header: %v", plugin, err)
	}
	reader := &xorshiftReader{state: 0x243F6A8885A308D3}
	if _, err := io.CopyN(tw, reader, payloadSize); err != nil {
		fail("write %s payload: %v", plugin, err)
	}

	if err := tw.Close(); err != nil {
		fail("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		fail("close gzip: %v", err)
	}
	if err := file.Sync(); err != nil {
		fail("sync %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

type xorshiftReader struct {
	state uint64
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

func (c *cgroup) peak() uint64 {
	data, err := os.ReadFile(filepath.Join(c.dir, "memory.peak"))
	if err != nil {
		return 0
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return value
}
