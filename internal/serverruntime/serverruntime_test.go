package serverruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMarker(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, InfoFile), []byte(body), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return root
}

func TestReadInfoParsesMarker(t *testing.T) {
	root := writeMarker(t, "TES3MP_VERSION=0.8.1\n"+
		"TES3MP_URL=https://example.invalid/tes3mp-server-0.8.1.tar.gz\n"+
		"TES3MP_SHA256=aa64a11f\n")

	info, err := ReadInfo(root)
	if err != nil {
		t.Fatalf("ReadInfo: %v", err)
	}
	if info.Version != "0.8.1" {
		t.Fatalf("Version = %q, want 0.8.1", info.Version)
	}
	if info.URL != "https://example.invalid/tes3mp-server-0.8.1.tar.gz" {
		t.Fatalf("URL = %q", info.URL)
	}
	if info.SHA256 != "aa64a11f" {
		t.Fatalf("SHA256 = %q", info.SHA256)
	}
}

func TestReadInfoIgnoresUnknownKeysAndNoise(t *testing.T) {
	root := writeMarker(t, "# managed by bootstrap.sh\n"+
		"\n"+
		"  TES3MP_VERSION = 0.9.0 \r\n"+
		"TES3MP_BUILD=local\n"+
		"garbage line\n")

	info, err := ReadInfo(root)
	if err != nil {
		t.Fatalf("ReadInfo: %v", err)
	}
	if info.Version != "0.9.0" {
		t.Fatalf("Version = %q, want 0.9.0", info.Version)
	}
	if info.URL != "" || info.SHA256 != "" {
		t.Fatalf("unexpected fields: %+v", info)
	}
}

func TestReadInfoRejectsMissingMarker(t *testing.T) {
	if _, err := ReadInfo(t.TempDir()); err == nil {
		t.Fatal("ReadInfo succeeded without a marker")
	}
}

func TestReadInfoRejectsVersionlessMarker(t *testing.T) {
	root := writeMarker(t, "TES3MP_URL=https://example.invalid/x.tar.gz\n")
	if _, err := ReadInfo(root); err == nil {
		t.Fatal("ReadInfo succeeded without TES3MP_VERSION")
	}
}

func TestReadInfoRejectsSymlinkMarker(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "marker")
	if err := os.WriteFile(target, []byte("TES3MP_VERSION=0.8.1\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, InfoFile)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := ReadInfo(root); err == nil {
		t.Fatal("ReadInfo followed a symlinked marker")
	}
}

func TestReadInfoRejectsOversizedMarker(t *testing.T) {
	root := writeMarker(t, "TES3MP_VERSION=0.8.1\n"+
		strings.Repeat("#", maxInfoBytes)+"\n")
	if _, err := ReadInfo(root); err == nil {
		t.Fatal("ReadInfo accepted an oversized marker")
	}
}

func TestReadInfoRejectsEmptyRoot(t *testing.T) {
	if _, err := ReadInfo("  "); err == nil {
		t.Fatal("ReadInfo accepted an empty root")
	}
}

func TestReadInfoRejectsRelativeRoot(t *testing.T) {
	if _, err := ReadInfo("opt/tes3mp-easy/tes3mp"); err == nil {
		t.Fatal("ReadInfo accepted a relative root")
	}
}
