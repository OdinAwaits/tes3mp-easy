package docs

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const maxStaticFixtureSize int64 = 8 << 20

var (
	baseGameFiles = map[string]bool{
		"morrowind.esm": true,
		"tribunal.esm":  true,
		"bloodmoon.esm": true,
	}
	gameDataExtensions = map[string]bool{
		".bsa": true,
		".sav": true,
		".ess": true,
	}
)

func TestStaticFixturesContainNoGameOrUserData(t *testing.T) {
	root := repoRoot(t)
	checked := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "example-client", "releases", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !hasPathComponent(rel, "testdata") {
			return nil
		}
		checked++

		info, err := entry.Info()
		if err != nil {
			return err
		}
		name := strings.ToLower(entry.Name())
		ext := strings.ToLower(filepath.Ext(name))
		switch {
		case baseGameFiles[name]:
			t.Errorf("%s: base Morrowind data must not be stored in testdata", rel)
		case gameDataExtensions[ext]:
			t.Errorf("%s: game or user data file is not allowed in testdata", rel)
		case info.Size() > maxStaticFixtureSize:
			t.Errorf("%s: static fixture is %d bytes; generate archives in tests",
				rel, info.Size())
		case ext == ".json" && hasPathComponent(rel, "players") && info.Size() > 4<<10:
			t.Errorf("%s: player fixture is too large to be synthetic", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	if checked == 0 {
		t.Log("no static testdata directories; fixtures are generated in tests")
	}
}

func hasPathComponent(path, want string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == want {
			return true
		}
	}
	return false
}
