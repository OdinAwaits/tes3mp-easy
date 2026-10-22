package doctor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// requiredDataDirs mirrors the layout in docs/v2/02-architecture.md §5. Only
// directories written by the game service need a write-access check; panel-data
// is owned by the panel account and must merely be readable by ops.
var requiredDataDirs = []struct {
	name     string
	writable bool
}{
	{name: "configs", writable: true},
	{name: "players", writable: true},
	{name: "world", writable: true},
	{name: "mods", writable: true},
	{name: "backups", writable: true},
	{name: "panel-data", writable: false},
}

func checkRoot(root string) Check {
	if root == "" {
		return errorCheck("root", "root path is empty")
	}

	info, err := os.Stat(root)
	if err != nil {
		return errorCheck("root", "cannot access %s: %v", root, err)
	}
	if !info.IsDir() {
		return errorCheck("root", "%s is not a directory", root)
	}
	if err := readableDir(root); err != nil {
		return errorCheck("root", "%s is not readable: %v", root, err)
	}

	problems := make([]string, 0, len(requiredDataDirs))
	for _, required := range requiredDataDirs {
		path := filepath.Join(root, required.name)
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			problems = append(problems, fmt.Sprintf("%s/ is missing", required.name))
			continue
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s/: %v", required.name, err))
			continue
		case info.Mode()&os.ModeSymlink != 0:
			problems = append(problems, fmt.Sprintf("%s/ is a symlink", required.name))
			continue
		case !info.IsDir():
			problems = append(problems, fmt.Sprintf("%s/ is not a directory", required.name))
			continue
		}

		if err := readableDir(path); err != nil {
			problems = append(problems,
				fmt.Sprintf("%s/ is not readable: %v", required.name, err))
			continue
		}
		if required.writable {
			if err := writableDir(path); err != nil {
				problems = append(problems,
					fmt.Sprintf("%s/ is not writable: %v", required.name, err))
			}
		}
	}

	if len(problems) > 0 {
		return errorCheck("root", "%s", strings.Join(problems, "; "))
	}
	return Check{Name: "root", Status: StatusOK}
}

// readableDir verifies that path can be opened and that one directory entry can
// be read. It does not modify the directory.
func readableDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func errorCheck(name, format string, args ...any) Check {
	return Check{Name: name, Status: StatusError, Message: fmt.Sprintf(format, args...)}
}

func warnCheck(name, format string, args ...any) Check {
	return Check{Name: name, Status: StatusWarn, Message: fmt.Sprintf(format, args...)}
}

func okCheck(name string, format string, args ...any) Check {
	message := ""
	if format != "" {
		message = fmt.Sprintf(format, args...)
	}
	return Check{Name: name, Status: StatusOK, Message: message}
}
