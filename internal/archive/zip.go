package archive

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

// ZIP limits are intentionally independent from the tar limits: the official
// TES3MP 0.8.1 Win64 archive is about 65 MiB compressed and 182 MiB
// uncompressed, while the defaults below leave room for future releases.
const (
	MaxZipCompressedSize   int64 = 512 << 20
	MaxZipUncompressedSize int64 = 1 << 30
	MaxZipEntries                = 10_000
)

// ZipLimits controls ZIP extraction limits. The zero value is valid and is
// useful in tests that exercise a limit itself.
type ZipLimits struct {
	CompressedSize   int64
	UncompressedSize int64
	Entries          int
}

// DefaultZipLimits returns the production ZIP limits.
func DefaultZipLimits() ZipLimits {
	return ZipLimits{
		CompressedSize:   MaxZipCompressedSize,
		UncompressedSize: MaxZipUncompressedSize,
		Entries:          MaxZipEntries,
	}
}

func (l ZipLimits) validate() error {
	if l.CompressedSize < 0 || l.UncompressedSize < 0 || l.Entries < 0 {
		return errors.New("archive: ZIP limits must not be negative")
	}
	return nil
}

// ExtractZipFile extracts a ZIP archive into dest. dest is created if missing.
// The archive is read from disk, so the compressed-size limit can be checked
// before its central directory is parsed.
func ExtractZipFile(name, dest string) error {
	return ExtractZipFileWithLimits(name, dest, DefaultZipLimits())
}

// ExtractZipFileWithLimits is ExtractZipFile with explicit limits.
func ExtractZipFileWithLimits(name, dest string, limits ZipLimits) error {
	if err := limits.validate(); err != nil {
		return err
	}
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("archive: %s is not a regular file", name)
	}
	if info.Size() > limits.CompressedSize {
		return fmt.Errorf("%w: ZIP compressed size exceeds %d bytes",
			ErrLimit, limits.CompressedSize)
	}

	reader, err := zip.OpenReader(name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	defer reader.Close()
	return ExtractZip(&reader.Reader, dest, limits)
}

// ExtractZip extracts a ZIP archive from r into dest. Every entry is checked
// before it reaches the filesystem: absolute paths, traversal, backslashes,
// symlinks and other special files are rejected. The uncompressed total is
// enforced while streaming, not only from the central directory.
func ExtractZip(r *zip.Reader, dest string, limits ZipLimits) error {
	if r == nil {
		return errors.New("archive: nil ZIP reader")
	}
	if dest == "" {
		return errors.New("archive: ZIP destination is required")
	}
	if err := limits.validate(); err != nil {
		return err
	}
	if len(r.File) > limits.Entries {
		return fmt.Errorf("%w: ZIP has more than %d entries",
			ErrLimit, limits.Entries)
	}
	if err := ensureExtractDir(dest); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(r.File))
	files := make(map[string]struct{})
	dirs := make(map[string]struct{})
	var total int64
	buf := make([]byte, 32*1024)

	for _, file := range r.File {
		cleaned, err := pathguard.Path(file.Name)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnsafePath, err)
		}
		if _, exists := seen[cleaned]; exists {
			return archiveInvalidf("%q: duplicate ZIP entry", cleaned)
		}
		seen[cleaned] = struct{}{}

		mode := file.Mode()
		if file.FileInfo().IsDir() || mode.IsDir() {
			if conflict := parentFile(files, cleaned); conflict != "" {
				return archiveInvalidf(
					"%q conflicts with file %q", cleaned, conflict)
			}
			dirs[cleaned] = struct{}{}
			ensureParents(dirs, cleaned)
			if err := ensureExtractDir(filepath.Join(dest,
				filepath.FromSlash(cleaned))); err != nil {
				return err
			}
			continue
		}
		if mode&os.ModeSymlink != 0 {
			return archiveInvalidf("%q: symlink entries are not allowed", cleaned)
		}
		if !mode.IsRegular() {
			return archiveInvalidf(
				"%q: unsupported ZIP entry type %v", cleaned, mode)
		}
		if conflict := parentFile(files, cleaned); conflict != "" {
			return archiveInvalidf(
				"%q conflicts with file %q", cleaned, conflict)
		}
		if _, exists := dirs[cleaned]; exists {
			return archiveInvalidf(
				"%q conflicts with an earlier directory", cleaned)
		}

		if file.UncompressedSize64 > uint64(limits.UncompressedSize) ||
			total > limits.UncompressedSize-int64(file.UncompressedSize64) {
			return fmt.Errorf("%w: ZIP uncompressed size exceeds %d bytes",
				ErrLimit, limits.UncompressedSize)
		}

		target := filepath.Join(dest, filepath.FromSlash(cleaned))
		remaining := limits.UncompressedSize - total
		written, err := extractZipFile(file, target, remaining, buf)
		if err != nil {
			return err
		}
		total += written
		files[cleaned] = struct{}{}
		ensureParents(dirs, cleaned)
	}
	return nil
}

func extractZipFile(file *zip.File, target string, remaining int64,
	buf []byte,
) (int64, error) {
	if err := ensureExtractDir(filepath.Dir(target)); err != nil {
		return 0, err
	}
	source, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("%w: open %q: %v", ErrInvalid, file.Name, err)
	}
	defer source.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	limited := &io.LimitedReader{R: source, N: remaining + 1}
	written, copyErr := io.CopyBuffer(out, limited, buf)
	closeErr := out.Close()
	if copyErr != nil {
		return written, fmt.Errorf("%w: extract %q: %v",
			ErrInvalid, file.Name, copyErr)
	}
	if closeErr != nil {
		return written, closeErr
	}
	if written > remaining {
		return written, fmt.Errorf("%w: ZIP uncompressed size exceeds %d bytes",
			ErrLimit, remaining)
	}
	return written, nil
}

func ensureParents(dirs map[string]struct{}, name string) {
	for {
		index := strings.LastIndexByte(name, '/')
		if index < 0 {
			return
		}
		name = name[:index]
		if name == "" {
			return
		}
		dirs[name] = struct{}{}
	}
}

func parentFile(files map[string]struct{}, name string) string {
	for {
		index := strings.LastIndexByte(name, '/')
		if index < 0 {
			return ""
		}
		name = name[:index]
		if _, exists := files[name]; exists {
			return name
		}
	}
}
