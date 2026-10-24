// Package release builds the reproducible server release archive used by
// bootstrap.sh. The archive contains bootstrap, systemd sources, the TES3MP
// version pin, documentation and both server binaries. Tar and gzip headers
// carry no timestamps or host-specific values.
package release

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// OpsBinary is the administrative CLI name inside the release.
	OpsBinary = "tes3mp-easy-ops"

	// PanelBinary is the HTTPS panel name inside the release.
	PanelBinary = "tes3mp-easy-panel"
)

var versionPattern = regexp.MustCompile(
	`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

// Binary is one prebuilt executable copied to bin/<Name>.
type Binary struct {
	Name string
	Path string
}

// Options describes one release build.
type Options struct {
	SourceDir string
	OutputDir string
	Version   string
	GOOS      string
	GOARCH    string
	Binaries  []Binary
}

// Result reports the generated files.
type Result struct {
	ArchivePath  string
	ChecksumPath string
	SHA256       string
	Size         int64
}

type asset struct {
	archivePath string
	sourcePath  string
	mode        fs.FileMode
	content     []byte
}

var fixedAssets = []struct {
	path string
	mode fs.FileMode
}{
	{"README.md", 0o644},
	{"bootstrap.sh", 0o755},
	{"rootctl/tes3mp-easy-rootctl", 0o755},
	{"sudoers.d/tes3mp-easy", 0o644},
	{"systemd/journald.conf.d/tes3mp-easy.conf", 0o644},
	{"systemd/tes3mp.service", 0o644},
	{"systemd/tes3mp-panel.service", 0o644},
	{"systemd/tes3mp-easy-backup.service", 0o644},
	{"systemd/tes3mp-easy-backup.timer", 0o644},
	{"sysctl.d/99-tes3mp-easy.conf", 0o644},
	{"server/tes3mp-version.txt", 0o644},
}

// ValidateVersion checks the version syntax used in archive names and build
// metadata.
func ValidateVersion(value string) error {
	if !versionPattern.MatchString(value) {
		return fmt.Errorf("invalid release version %q", value)
	}
	return nil
}

func normalize(options Options) (Options, error) {
	opts := options
	if opts.SourceDir == "" {
		opts.SourceDir = "."
	}
	if opts.OutputDir == "" {
		opts.OutputDir = "dist"
	}
	if opts.GOOS == "" {
		opts.GOOS = "linux"
	}
	if opts.GOARCH == "" {
		opts.GOARCH = "amd64"
	}
	if err := ValidateVersion(opts.Version); err != nil {
		return Options{}, err
	}
	if opts.GOOS != "linux" || opts.GOARCH != "amd64" {
		return Options{}, fmt.Errorf(
			"unsupported release target %s/%s (need linux/amd64)",
			opts.GOOS, opts.GOARCH)
	}
	var err error
	if opts.SourceDir, err = filepath.Abs(opts.SourceDir); err != nil {
		return Options{}, fmt.Errorf("resolve source directory: %w", err)
	}
	if opts.OutputDir, err = filepath.Abs(opts.OutputDir); err != nil {
		return Options{}, fmt.Errorf("resolve output directory: %w", err)
	}
	if len(opts.Binaries) == 0 {
		return Options{}, errors.New("at least one binary is required")
	}
	seen := make(map[string]bool, len(opts.Binaries))
	for i := range opts.Binaries {
		name := opts.Binaries[i].Name
		if name == "" || path.Base(name) != name || strings.ContainsAny(name, `/\`) {
			return Options{}, fmt.Errorf("invalid binary name %q", name)
		}
		if seen[name] {
			return Options{}, fmt.Errorf("duplicate binary name %q", name)
		}
		seen[name] = true
	}
	return opts, nil
}

// Create writes dist/<archive>.tar.gz and its .sha256 sidecar. The archive is
// replaced only after the compressed bytes were written and synced completely.
func Create(options Options) (Result, error) {
	opts, err := normalize(options)
	if err != nil {
		return Result{}, err
	}

	prefix := fmt.Sprintf("tes3mp-easy-%s-%s-%s",
		opts.Version, opts.GOOS, opts.GOARCH)
	assets, err := collectAssets(opts, prefix)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}

	archivePath := filepath.Join(opts.OutputDir, prefix+".tar.gz")
	checksumPath := archivePath + ".sha256"
	temp, err := os.CreateTemp(opts.OutputDir, "."+prefix+".tar.gz.tmp-*")
	if err != nil {
		return Result{}, fmt.Errorf("create archive temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()

	sum, size, err := writeArchive(temp, assets)
	if err != nil {
		return Result{}, err
	}
	if err := temp.Sync(); err != nil {
		return Result{}, fmt.Errorf("sync archive: %w", err)
	}
	if err := temp.Close(); err != nil {
		return Result{}, fmt.Errorf("close archive: %w", err)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return Result{}, fmt.Errorf("chmod archive: %w", err)
	}
	if err := os.Rename(tempPath, archivePath); err != nil {
		return Result{}, fmt.Errorf("replace archive: %w", err)
	}

	checksum := fmt.Sprintf("%s  %s\n", sum, filepath.Base(archivePath))
	if err := writeFileAtomic(checksumPath, []byte(checksum), 0o644); err != nil {
		return Result{}, err
	}
	return Result{
		ArchivePath:  archivePath,
		ChecksumPath: checksumPath,
		SHA256:       sum,
		Size:         size,
	}, nil
}

func collectAssets(opts Options, prefix string) ([]asset, error) {
	assets := make([]asset, 0, len(fixedAssets)+len(opts.Binaries)+4)
	seen := make(map[string]bool)

	add := func(rel string, mode fs.FileMode) error {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("unsafe release asset path %q", rel)
		}
		if seen[rel] {
			return fmt.Errorf("duplicate release asset %q", rel)
		}
		source := filepath.Join(opts.SourceDir, filepath.FromSlash(rel))
		info, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("release asset %s: %w", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release asset %s is a symlink", rel)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("release asset %s is not a regular file", rel)
		}
		seen[rel] = true
		assets = append(assets, asset{
			archivePath: path.Join(prefix, rel),
			sourcePath:  source,
			mode:        mode,
		})
		return nil
	}

	for _, item := range fixedAssets {
		if err := add(item.path, item.mode); err != nil {
			return nil, err
		}
	}
	seen["VERSION"] = true
	assets = append(assets, asset{
		archivePath: path.Join(prefix, "VERSION"),
		mode:        0o644,
		content:     []byte(opts.Version + "\n"),
	})

	docs, err := collectTree(filepath.Join(opts.SourceDir, "docs"), "docs")
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if err := add(doc, 0o644); err != nil {
			return nil, err
		}
	}

	for _, binary := range opts.Binaries {
		rel := path.Join("bin", binary.Name)
		source := binary.Path
		if source == "" {
			source = filepath.Join(opts.SourceDir, filepath.FromSlash(rel))
		}
		info, err := os.Lstat(source)
		if err != nil {
			return nil, fmt.Errorf("release binary %s: %w", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("release binary %s is a symlink", rel)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("release binary %s is not a regular file", rel)
		}
		if seen[rel] {
			return nil, fmt.Errorf("duplicate release asset %q", rel)
		}
		seen[rel] = true
		assets = append(assets, asset{
			archivePath: path.Join(prefix, rel),
			sourcePath:  source,
			mode:        0o755,
		})
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].archivePath < assets[j].archivePath
	})
	return assets, nil
}

func collectTree(root, prefix string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("release asset %s is a symlink", file)
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		files = append(files, path.Join(prefix, filepath.ToSlash(rel)))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect %s: %w", prefix, err)
	}
	sort.Strings(files)
	return files, nil
}

func writeArchive(dst io.Writer, assets []asset) (string, int64, error) {
	hash := sha256.New()
	counter := &countingWriter{writer: io.MultiWriter(dst, hash)}
	gz, err := gzip.NewWriterLevel(counter, gzip.BestCompression)
	if err != nil {
		return "", 0, fmt.Errorf("create gzip writer: %w", err)
	}
	gz.Header.ModTime = time.Unix(0, 0).UTC()
	gz.Header.OS = 255

	tw := tar.NewWriter(gz)
	for _, item := range assets {
		source, size, closeSource, err := openAsset(item)
		if err != nil {
			return "", 0, err
		}
		header := &tar.Header{
			Name:     item.archivePath,
			Mode:     int64(item.mode.Perm()),
			Size:     size,
			ModTime:  time.Unix(0, 0).UTC(),
			Typeflag: tar.TypeReg,
			Uname:    "root",
			Gname:    "root",
			Format:   tar.FormatUSTAR,
		}
		if err := tw.WriteHeader(header); err != nil {
			closeSource()
			return "", 0, fmt.Errorf("write tar header %s: %w", item.archivePath, err)
		}
		if _, err := io.Copy(tw, source); err != nil {
			closeSource()
			return "", 0, fmt.Errorf("write asset %s: %w", item.archivePath, err)
		}
		if err := closeSource(); err != nil {
			return "", 0, fmt.Errorf("close asset %s: %w", item.archivePath, err)
		}
	}
	if err := tw.Close(); err != nil {
		return "", 0, fmt.Errorf("close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", 0, fmt.Errorf("close gzip: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), counter.written, nil
}

func openAsset(item asset) (io.Reader, int64, func() error, error) {
	if item.sourcePath == "" {
		body := item.content
		return strings.NewReader(string(body)), int64(len(body)), func() error { return nil }, nil
	}
	file, err := os.Open(item.sourcePath)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("open asset %s: %w", item.sourcePath, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, nil, fmt.Errorf("stat asset %s: %w", item.sourcePath, err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, 0, nil, fmt.Errorf("asset %s is not a regular file", item.sourcePath)
	}
	return file, info.Size(), file.Close, nil
}

type countingWriter struct {
	writer  io.Writer
	written int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.written += int64(n)
	return n, err
}

func writeFileAtomic(name string, content []byte, mode fs.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if _, err := temp.Write(content); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return fmt.Errorf("chmod temporary file: %w", err)
	}
	if err := os.Rename(tempPath, name); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
