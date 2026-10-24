package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(name), err)
	}
	if err := os.WriteFile(name, []byte(body), mode); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func sourceTree(t *testing.T) (string, []Binary) {
	t.Helper()
	root := t.TempDir()
	for _, item := range fixedAssets {
		writeFile(t, filepath.Join(root, filepath.FromSlash(item.path)),
			"asset "+item.path+"\n", 0o644)
	}
	writeFile(t, filepath.Join(root, "docs", "v2", "README.md"), "# docs\n", 0o644)
	writeFile(t, filepath.Join(root, "docs", "reference", "modding.md"), "# mods\n", 0o644)
	ops := filepath.Join(root, "build", OpsBinary)
	panel := filepath.Join(root, "build", PanelBinary)
	writeFile(t, ops, "ops binary\n", 0o755)
	writeFile(t, panel, "panel binary\n", 0o755)
	return root, []Binary{
		{Name: OpsBinary, Path: ops},
		{Name: PanelBinary, Path: panel},
	}
}

func TestCreateIsReproducible(t *testing.T) {
	source, binaries := sourceTree(t)
	options := Options{
		SourceDir: source,
		OutputDir: t.TempDir(),
		Version:   "1.2.3",
		Binaries:  binaries,
	}
	first, err := Create(options)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	options.OutputDir = t.TempDir()
	second, err := Create(options)
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	firstData, err := os.ReadFile(first.ArchivePath)
	if err != nil {
		t.Fatalf("read first archive: %v", err)
	}
	secondData, err := os.ReadFile(second.ArchivePath)
	if err != nil {
		t.Fatalf("read second archive: %v", err)
	}
	if !bytes.Equal(firstData, secondData) {
		t.Fatal("release archive is not reproducible")
	}
	if got := len(firstData); got == 0 {
		t.Fatal("empty archive")
	}
	if first.Size != int64(len(firstData)) {
		t.Fatalf("result size = %d, archive = %d", first.Size, len(firstData))
	}
	sum := sha256.Sum256(firstData)
	if first.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %s, want %s", first.SHA256, hex.EncodeToString(sum[:]))
	}
	checksum, err := os.ReadFile(first.ChecksumPath)
	if err != nil {
		t.Fatalf("read checksum: %v", err)
	}
	wantChecksum := first.SHA256 + "  " + filepath.Base(first.ArchivePath) + "\n"
	if string(checksum) != wantChecksum {
		t.Fatalf("checksum file = %q, want %q", checksum, wantChecksum)
	}
	if filepath.Base(first.ArchivePath) != "tes3mp-easy-1.2.3-linux-amd64.tar.gz" {
		t.Fatalf("archive name = %s", filepath.Base(first.ArchivePath))
	}
	if info, err := os.Stat(first.ArchivePath); err != nil {
		t.Fatalf("stat archive: %v", err)
	} else if info.Mode().Perm() != 0o644 {
		t.Fatalf("archive mode = %04o, want 0644", info.Mode().Perm())
	}

	entries := readEntries(t, first.ArchivePath)
	prefix := "tes3mp-easy-1.2.3-linux-amd64/"
	for _, want := range []string{
		prefix + "VERSION",
		prefix + "bootstrap.sh",
		prefix + "bin/" + OpsBinary,
		prefix + "bin/" + PanelBinary,
		prefix + "systemd/tes3mp.service",
		prefix + "docs/v2/README.md",
	} {
		if _, ok := entries[want]; !ok {
			t.Fatalf("archive has no entry %s", want)
		}
	}
	if entries[prefix+"VERSION"].body != "1.2.3\n" {
		t.Fatalf("VERSION = %q", entries[prefix+"VERSION"].body)
	}
	if entries[prefix+"bootstrap.sh"].mode != 0o755 {
		t.Fatalf("bootstrap mode = %04o", entries[prefix+"bootstrap.sh"].mode)
	}
	if entries[prefix+"bin/"+OpsBinary].mode != 0o755 {
		t.Fatalf("ops mode = %04o", entries[prefix+"bin/"+OpsBinary].mode)
	}
	for name, entry := range entries {
		if entry.modTime != 0 || entry.uid != 0 || entry.gid != 0 {
			t.Fatalf("%s has non-zero metadata: %+v", name, entry)
		}
	}
}

type entry struct {
	body    string
	mode    int64
	modTime int64
	uid     int
	gid     int
}

func readEntries(t *testing.T, archive string) map[string]entry {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	entries := make(map[string]entry)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		entries[header.Name] = entry{
			body:    string(data),
			mode:    header.Mode,
			modTime: header.ModTime.Unix(),
			uid:     header.Uid,
			gid:     header.Gid,
		}
	}
	return entries
}

func TestCreateRejectsMissingBinary(t *testing.T) {
	source, binaries := sourceTree(t)
	if err := os.Remove(binaries[0].Path); err != nil {
		t.Fatalf("remove binary: %v", err)
	}
	_, err := Create(Options{
		SourceDir: source,
		Version:   "1.2.3",
		Binaries:  binaries,
	})
	if err == nil || !strings.Contains(err.Error(), "release binary") {
		t.Fatalf("error = %v, want missing binary", err)
	}
}

func TestCreateRejectsSymlinkAsset(t *testing.T) {
	source, binaries := sourceTree(t)
	readme := filepath.Join(source, "README.md")
	if err := os.Remove(readme); err != nil {
		t.Fatalf("remove README: %v", err)
	}
	if err := os.Symlink("bootstrap.sh", readme); err != nil {
		t.Fatalf("symlink README: %v", err)
	}
	_, err := Create(Options{
		SourceDir: source,
		Version:   "1.2.3",
		Binaries:  binaries,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v, want symlink rejection", err)
	}
}

func TestCreateRejectsUnsupportedTarget(t *testing.T) {
	source, binaries := sourceTree(t)
	_, err := Create(Options{
		SourceDir: source,
		Version:   "1.2.3",
		GOOS:      "windows",
		GOARCH:    "amd64",
		Binaries:  binaries,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported release target") {
		t.Fatalf("error = %v, want unsupported target", err)
	}
}
