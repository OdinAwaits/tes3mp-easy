package restorepoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

// Extract verifies and unpacks a snapshot or checkpoint into an empty
// directory. Snapshot layers keep their state/, mods/ and configs/ prefixes;
// checkpoint files are written at the top level.
func Extract(path, dest string) (Manifest, error) {
	if dest == "" {
		return Manifest{}, errors.New("restore point: extraction destination is required")
	}
	kind := parsedKindFor(path)
	reader, err := archive.OpenWithLimits(path, Limits(kind))
	if err != nil {
		return Manifest{}, err
	}
	defer reader.Close()

	if err := ensureEmptyDir(dest); err != nil {
		return Manifest{}, err
	}

	var (
		manifest Manifest
		layers   = make(map[string][]File)
		dirs     = make(map[string]bool)
		sawMeta  bool
	)

	entry, err := reader.Next()
	if err != nil {
		return Manifest{}, err
	}
	if entry.Type != archive.TypeFile || entry.Name != manifestName {
		return Manifest{}, invalidf("%s: first entry must be %s", path, manifestName)
	}
	if entry.Size > MaxManifestSize {
		return Manifest{}, fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, manifestName, MaxManifestSize)
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxManifestSize+1))
	if err != nil {
		return Manifest{}, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, invalidf("%s: %v", manifestName, err)
	}
	if err := manifest.validate(kind); err != nil {
		return Manifest{}, err
	}
	sawMeta = true

	for {
		entry, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, err
		}
		relative, layer, err := classifyEntry(kind, entry)
		if err != nil {
			return Manifest{}, err
		}
		target := filepath.Join(dest, filepath.FromSlash(entry.Name))
		if entry.Type == archive.TypeDir {
			dirs[relative] = true
			if err := os.MkdirAll(target, 0o755); err != nil {
				return Manifest{}, err
			}
			continue
		}

		file, err := extractFile(target, reader, entry.Size)
		if err != nil {
			return Manifest{}, err
		}
		file.Path = relative
		if layer != "" {
			layers[layer] = append(layers[layer], file)
		} else {
			layers["state"] = append(layers["state"], file)
		}
	}

	if !sawMeta {
		return Manifest{}, invalidf("%s: %s is missing", path, manifestName)
	}
	if err := verifyExtracted(kind, dest, manifest, dirs, layers); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ensureEmptyDir(dest string) error {
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("%s: extraction directory is not empty", dest)
	}
	return nil
}

func extractFile(target string, r io.Reader, size int64) (File, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return File{}, err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return File{}, err
	}
	sum := sha256.New()
	written, copyErr := io.CopyBuffer(io.MultiWriter(file, sum), r,
		make([]byte, 32*1024))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(target)
		return File{}, errors.Join(copyErr, closeErr)
	}
	if written != size {
		_ = os.Remove(target)
		return File{}, invalidf("%s: size mismatch", target)
	}
	return File{Size: written, SHA256: hex.EncodeToString(sum.Sum(nil))}, nil
}

// classifyEntry validates one tar entry against the layout and returns either
// the layer-relative file path or, for a directory, its canonical directory
// key. The layer is empty for checkpoint state files.
func classifyEntry(kind Kind, entry *archive.Entry) (string, string, error) {
	name := entry.Name
	if entry.Type == archive.TypeDir {
		valid := map[string]bool{}
		if kind == KindSnapshot {
			for _, dir := range []string{
				"state", "state/players", "state/world", "mods",
				"mods/plugins", "mods/scripts", "configs",
			} {
				valid[dir] = true
			}
			for _, dir := range stateWorldDirs {
				valid["state/world/"+dir] = true
			}
		} else {
			valid["players"] = true
			valid["world"] = true
			for _, dir := range stateWorldDirs {
				valid["world/"+dir] = true
			}
		}
		if !valid[name] {
			return "", "", invalidf("%q: unexpected directory", name)
		}
		return name, "", nil
	}

	if kind == KindSnapshot {
		return classifySnapshotFile(name)
	}
	relative, ok := classifyCheckpointFile(name)
	if !ok {
		return "", "", invalidf("%q: unexpected checkpoint file", name)
	}
	return relative, "", nil
}

func classifySnapshotFile(name string) (string, string, error) {
	switch {
	case name == "state/requiredDataFiles.json":
		return "requiredDataFiles.json", "state", nil
	case strings.HasPrefix(name, "state/players/"):
		rest := strings.TrimPrefix(name, "state/players/")
		if rest == "" || strings.Contains(rest, "/") {
			return "", "", invalidf("%q: nested state file", name)
		}
		return "players/" + rest, "state", nil
	case strings.HasPrefix(name, "state/world/"):
		rest := strings.TrimPrefix(name, "state/world/")
		dir, file, ok := strings.Cut(rest, "/")
		if !ok || file == "" || strings.Contains(file, "/") || !isWorldDir(dir) {
			return "", "", invalidf("%q: unexpected world file", name)
		}
		return "world/" + dir + "/" + file, "state", nil
	case strings.HasPrefix(name, "mods/plugins/"):
		rest := strings.TrimPrefix(name, "mods/plugins/")
		if rest == "" || strings.Contains(rest, "/") {
			return "", "", invalidf("%q: nested mods file", name)
		}
		return "plugins/" + rest, "mods", nil
	case strings.HasPrefix(name, "mods/scripts/"):
		rest := strings.TrimPrefix(name, "mods/scripts/")
		if rest == "" || strings.Contains(rest, "/") {
			return "", "", invalidf("%q: nested mods file", name)
		}
		return "scripts/" + rest, "mods", nil
	case strings.HasPrefix(name, "configs/"):
		rest := strings.TrimPrefix(name, "configs/")
		if !isConfigFile(rest) {
			return "", "", invalidf("%q: unexpected config file", name)
		}
		return rest, "configs", nil
	default:
		return "", "", invalidf("%q: unexpected snapshot file", name)
	}
}

func classifyCheckpointFile(name string) (string, bool) {
	switch {
	case name == "requiredDataFiles.json":
		return name, true
	case strings.HasPrefix(name, "players/"):
		rest := strings.TrimPrefix(name, "players/")
		return name, rest != "" && !strings.Contains(rest, "/")
	case strings.HasPrefix(name, "world/"):
		rest := strings.TrimPrefix(name, "world/")
		dir, file, ok := strings.Cut(rest, "/")
		return name, ok && file != "" && !strings.Contains(file, "/") && isWorldDir(dir)
	default:
		return "", false
	}
}

func isWorldDir(name string) bool {
	for _, dir := range stateWorldDirs {
		if name == dir {
			return true
		}
	}
	return false
}

func isConfigFile(name string) bool {
	for _, file := range configFiles {
		if name == file {
			return true
		}
	}
	return false
}

func verifyExtracted(kind Kind, dest string, manifest Manifest,
	dirs map[string]bool, layers map[string][]File,
) error {
	state := layers["state"]
	if kind == KindSnapshot {
		for _, dir := range []string{
			"state", "state/players", "state/world", "mods",
			"mods/plugins", "mods/scripts", "configs",
		} {
			if !dirs[dir] {
				return invalidf("snapshot is missing %s/", dir)
			}
		}
		for _, dir := range stateWorldDirs {
			if !dirs["state/world/"+dir] {
				return invalidf("snapshot is missing state/world/%s/", dir)
			}
		}
		if err := verifyRequiredData(filepath.Join(dest, "state"), "state"); err != nil {
			return err
		}
		if err := verifyRequiredData(
			filepath.Join(dest, "mods", "plugins"), "mods"); err != nil {
			return err
		}
		for _, name := range configFiles {
			found := false
			for _, file := range layers["configs"] {
				if file.Path == name {
					found = true
					break
				}
			}
			if !found {
				return invalidf("snapshot is missing configs/%s", name)
			}
		}
		if got := Digest("state", state); got != manifest.StateSHA256 {
			return invalidf("snapshot state digest mismatch")
		}
		if got := Digest("mods", layers["mods"]); got != manifest.ModsSHA256 {
			return invalidf("snapshot mods digest mismatch")
		}
		if got := Digest("configs", layers["configs"]); got != manifest.ConfigsSHA256 {
			return invalidf("snapshot configs digest mismatch")
		}
		return nil
	}

	for _, dir := range []string{"players", "world"} {
		if !dirs[dir] {
			return invalidf("checkpoint is missing %s/", dir)
		}
	}
	for _, dir := range stateWorldDirs {
		if !dirs["world/"+dir] {
			return invalidf("checkpoint is missing world/%s/", dir)
		}
	}
	if err := verifyRequiredData(dest, "state"); err != nil {
		return err
	}
	if got := Digest("state", state); got != manifest.WorldSHA256 {
		return invalidf("checkpoint world digest mismatch")
	}
	players := 0
	for _, file := range state {
		if strings.HasPrefix(file.Path, "players/") {
			players++
		}
	}
	if manifest.Players != nil && *manifest.Players != players {
		return invalidf("checkpoint players count mismatch")
	}
	return nil
}

func verifyRequiredData(dir, layer string) error {
	path := filepath.Join(dir, requiredDataFilesName)
	data, err := os.ReadFile(path)
	if err != nil {
		return invalidf("%s layer is missing %s", layer, requiredDataFilesName)
	}
	if _, err := archive.ParseRequiredDataFiles(data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
