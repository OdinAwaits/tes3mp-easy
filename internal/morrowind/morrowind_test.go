package morrowind

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeMorrowind(t *testing.T, dataDir string, files ...string) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInspect(t *testing.T) {
	root := makeMorrowind(t, "Data Files", RequiredFiles...)
	installation, err := Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if installation.Root != root ||
		installation.DataFiles != filepath.Join(root, "Data Files") {
		t.Fatalf("installation = %+v", installation)
	}
	for _, name := range RequiredFiles {
		if installation.Files[name] == "" {
			t.Fatalf("missing path for %s", name)
		}
	}
}

func TestInspectCaseInsensitive(t *testing.T) {
	root := makeMorrowind(t, "data files",
		"morrowind.ESM", "TRIBUNAL.esm", "Bloodmoon.ESM")
	installation, err := Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if filepath.Base(installation.DataFiles) != "data files" {
		t.Fatalf("DataFiles = %q", installation.DataFiles)
	}
}

func TestInspectReportsMissingFiles(t *testing.T) {
	root := makeMorrowind(t, "Data Files", "Morrowind.esm")
	_, err := Inspect(root)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Inspect = %v, want %v", err, ErrIncomplete)
	}
	var incomplete *IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("error type = %T", err)
	}
	want := []string{"Data Files/Tribunal.esm", "Data Files/Bloodmoon.esm"}
	if strings.Join(incomplete.Missing, ",") != strings.Join(want, ",") {
		t.Fatalf("missing = %v, want %v", incomplete.Missing, want)
	}
}

func TestInspectRejectsMissingDataFilesAndFile(t *testing.T) {
	tests := []struct {
		name string
		root func(t *testing.T) string
	}{
		{
			name: "root is a file",
			root: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "Morrowind")
				if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
		},
		{
			name: "missing Data Files",
			root: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name: "empty root",
			root: func(t *testing.T) string { return "" },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Inspect(test.root(t)); err == nil {
				t.Fatal("Inspect succeeded")
			}
		})
	}
}
