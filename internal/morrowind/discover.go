package morrowind

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// Registry reads a string value from the Windows registry. Tests provide a
// map-backed implementation; other platforms use an unavailable stub.
type Registry interface {
	ReadString(key, value string) (string, error)
}

// Options controls Morrowind discovery. Zero values use the current host.
type Options struct {
	// Root is the configured folder; it is checked before automatic candidates.
	Root string
	GOOS string

	Getenv   func(string) string
	Registry Registry
	ReadFile func(string) ([]byte, error)
	ReadDir  func(string) ([]os.DirEntry, error)
	Inspect  func(string) (Installation, error)
}

func (o Options) withDefaults() Options {
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if o.Registry == nil {
		o.Registry = systemRegistry()
	}
	if o.ReadFile == nil {
		o.ReadFile = os.ReadFile
	}
	if o.ReadDir == nil {
		o.ReadDir = os.ReadDir
	}
	if o.Inspect == nil {
		o.Inspect = Inspect
	}
	return o
}

// Discover checks the configured folder, then the documented Windows or Linux
// locations. It returns the first complete installation. An explicitly
// configured folder with an unexpected error (for example, permission denied)
// is returned as-is instead of being silently skipped.
func Discover(options Options) (Installation, error) {
	options = options.withDefaults()
	candidates := candidateRoots(options)

	explicit := cleanRoot(options.GOOS, options.Root)
	seenIncomplete := make([]*IncompleteError, 0, 1)
	for index, candidate := range candidates {
		installation, err := options.Inspect(candidate)
		if err == nil {
			return installation, nil
		}
		var incomplete *IncompleteError
		if errors.As(err, &incomplete) {
			seenIncomplete = append(seenIncomplete, incomplete)
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if index == 0 && explicit != "" && sameRoot(options.GOOS, candidate, explicit) {
			return Installation{}, err
		}
	}
	return Installation{}, &NotFoundError{
		Candidates: candidates,
		Incomplete: seenIncomplete,
	}
}

func candidateRoots(options Options) []string {
	var roots []string
	add := func(value string) {
		value = cleanRoot(options.GOOS, value)
		if value == "" || !isAbsolute(options.GOOS, value) {
			return
		}
		roots = append(roots, value)
	}
	add(options.Root)

	switch options.GOOS {
	case "windows":
		for _, root := range windowsCandidates(options) {
			add(root)
		}
	case "linux":
		for _, root := range linuxCandidates(options) {
			add(root)
		}
	}
	return uniqueRoots(options.GOOS, roots)
}

func sameRoot(goos, left, right string) bool {
	if goos == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func uniqueRoots(goos string, roots []string) []string {
	seen := make(map[string]struct{}, len(roots))
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		key := root
		if goos == "windows" {
			key = strings.ToLower(root)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, root)
	}
	return result
}

func linuxCandidates(options Options) []string {
	home := strings.TrimSpace(options.Getenv("HOME"))
	if home == "" {
		return nil
	}
	steamRoots := []string{
		joinRoot(options.GOOS, home, ".steam", "steam"),
		joinRoot(options.GOOS, home, ".local", "share", "Steam"),
		joinRoot(options.GOOS, home, ".steam", "root"),
		joinRoot(options.GOOS, home, ".var", "app",
			"com.valvesoftware.Steam", ".local", "share", "Steam"),
	}
	return steamCandidates(options, steamRoots)
}

func windowsCandidates(options Options) []string {
	var roots []string
	for _, key := range []string{
		`SOFTWARE\WOW6432Node\Bethesda Softworks\Morrowind`,
		`SOFTWARE\Bethesda Softworks\Morrowind`,
	} {
		if value, err := options.Registry.ReadString(key, "Install Path"); err == nil {
			roots = append(roots, value)
		}
	}

	steamRoots := []string{}
	if programFiles := strings.TrimSpace(options.Getenv("ProgramFiles(x86)")); programFiles != "" {
		steamRoots = append(steamRoots,
			joinRoot(options.GOOS, programFiles, "Steam"))
	}
	if programFiles := strings.TrimSpace(options.Getenv("ProgramFiles")); programFiles != "" {
		steamRoots = append(steamRoots,
			joinRoot(options.GOOS, programFiles, "Steam"))
	}
	steamRoots = append(steamRoots, `C:\Program Files (x86)\Steam`)
	roots = append(roots, steamCandidates(options, steamRoots)...)

	gogRoots := []string{
		`C:\GOG Games`,
		`C:\Program Files (x86)\GOG Galaxy\Games`,
		`C:\Program Files\GOG Galaxy\Games`,
	}
	for _, base := range gogRoots {
		roots = append(roots, joinRoot(options.GOOS, base, "Morrowind"))
		entries, err := options.ReadDir(base)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(
				strings.ToLower(entry.Name()), "morrowind") {
				continue
			}
			roots = append(roots, joinRoot(options.GOOS, base, entry.Name()))
		}
	}
	return roots
}

func steamCandidates(options Options, steamRoots []string) []string {
	var roots []string
	for _, steamRoot := range uniqueRoots(options.GOOS, steamRoots) {
		roots = append(roots,
			joinRoot(options.GOOS, steamRoot, "steamapps", "common", "Morrowind"))
		vdfPath := joinRoot(options.GOOS, steamRoot, "steamapps",
			"libraryfolders.vdf")
		data, err := options.ReadFile(vdfPath)
		if err != nil {
			continue
		}
		for _, library := range parseSteamLibraryPaths(data, options.GOOS) {
			roots = append(roots, joinRoot(options.GOOS, library,
				"steamapps", "common", "Morrowind"))
		}
	}
	return roots
}

func joinRoot(goos, base string, parts ...string) string {
	if goos != "windows" {
		return filepath.Join(append([]string{base}, parts...)...)
	}
	base = strings.ReplaceAll(base, "/", `\`)
	result := strings.TrimRight(base, `\`)
	for _, part := range parts {
		part = strings.Trim(strings.ReplaceAll(part, "/", `\`), `\`)
		if part == "" {
			continue
		}
		if result == "" {
			result = part
		} else {
			result += `\` + part
		}
	}
	return result
}

func cleanRoot(goos, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if goos != "windows" {
		return filepath.Clean(value)
	}
	slashed := strings.ReplaceAll(value, `\`, "/")
	cleaned := path.Clean(slashed)
	if len(cleaned) == 2 && cleaned[1] == ':' {
		cleaned += "/"
	}
	return strings.ReplaceAll(cleaned, "/", `\`)
}

func isAbsolute(goos, value string) bool {
	if goos != "windows" {
		return filepath.IsAbs(value)
	}
	if len(value) >= 3 && value[1] == ':' && value[2] == '\\' {
		c := value[0]
		return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
	}
	return strings.HasPrefix(value, `\\`)
}

var steamLibraryPair = regexp.MustCompile(
	`"(path|[0-9]+)"\s*"((?:\\.|[^"\\])*)"`)

// parseSteamLibraryPaths extracts library roots from both modern
// libraryfolders.vdf files ("path" keys) and the legacy numeric-key format.
// Only absolute paths are accepted, which filters out app-size values.
func parseSteamLibraryPaths(data []byte, goos string) []string {
	var paths []string
	for _, match := range steamLibraryPair.FindAllSubmatch(data, -1) {
		key := string(match[1])
		if key != "path" && !allDigits(key) {
			continue
		}
		value := cleanRoot(goos, unescapeVDF(string(match[2])))
		if isAbsolute(goos, value) {
			paths = append(paths, value)
		}
	}
	sort.Strings(paths)
	return uniqueRoots(goos, paths)
}

func unescapeVDF(value string) string {
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			index++
		}
		result.WriteByte(value[index])
	}
	return result.String()
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, current := range value {
		if current < '0' || current > '9' {
			return false
		}
	}
	return true
}
