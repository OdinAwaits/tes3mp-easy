// Package archive reads tar.gz archives as a stream and rejects entries that
// could escape the target directory.
package archive

import (
	"archive/tar"
	"bufio"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

// Archive limits from docs/v2/03-contracts.md §6.6.
const (
	MaxCompressedSize   int64 = 512 << 20
	MaxUncompressedSize int64 = 1 << 30
	MaxEntries                = 50_000
)

var (
	// ErrInvalid marks a malformed archive: bad gzip/tar data, disallowed entry
	// types, duplicates or trailing garbage.
	ErrInvalid = errors.New("invalid archive")
	// ErrLimit marks an archive that exceeds one of the configured limits.
	ErrLimit = errors.New("archive limit exceeded")
	// ErrUnsafePath is an alias for pathguard.ErrUnsafePath so callers can use
	// either package for errors.Is.
	ErrUnsafePath = pathguard.ErrUnsafePath
)

// EntryType is the subset of tar entry types accepted by Reader.
type EntryType uint8

const (
	// TypeFile is a regular file (tar TypeReg or the legacy TypeRegA).
	TypeFile EntryType = iota
	// TypeDir is a directory entry.
	TypeDir
)

// String implements fmt.Stringer for diagnostics.
func (t EntryType) String() string {
	switch t {
	case TypeFile:
		return "file"
	case TypeDir:
		return "directory"
	default:
		return "unknown"
	}
}

// Entry is the metadata of one archive entry. Content is available through
// Reader.Read after Next returns the entry.
type Entry struct {
	Name    string
	Type    EntryType
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
}

// Limits controls the archive limits. A zero value is valid and is useful in
// tests that exercise the limit itself.
type Limits struct {
	CompressedSize   int64
	UncompressedSize int64
	Entries          int
}

// DefaultLimits returns the production limits.
func DefaultLimits() Limits {
	return Limits{
		CompressedSize:   MaxCompressedSize,
		UncompressedSize: MaxUncompressedSize,
		Entries:          MaxEntries,
	}
}

func (l Limits) validate() error {
	if l.CompressedSize < 0 || l.UncompressedSize < 0 || l.Entries < 0 {
		return errors.New("archive: limits must not be negative")
	}
	return nil
}

type seenEntry struct {
	name string
	typ  EntryType
}

// Reader streams one tar.gz archive. It never buffers the archive itself; the
// caller decides how much of the current entry to read.
type Reader struct {
	br           *bufio.Reader
	gz           *gzip.Reader
	tr           *tar.Reader
	uncompressed *limitReader
	limits       Limits

	seen    map[string]seenEntry
	implied map[string]string

	entries int
	current *Entry
	closer  io.Closer

	err       error
	finished  bool
	finishErr error
	closed    bool
}

// Open opens a file and returns a streaming Reader for it.
func Open(name string) (*Reader, error) {
	return OpenWithLimits(name, DefaultLimits())
}

// OpenWithLimits is Open with explicit limits.
func OpenWithLimits(name string, limits Limits) (*Reader, error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("archive: %s is not a regular file", name)
	}
	if info.Size() > limits.CompressedSize {
		_ = f.Close()
		return nil, limitError("compressed size", limits.CompressedSize)
	}

	r, err := newReader(f, limits, f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return r, nil
}

// NewReader returns a streaming Reader for r.
func NewReader(r io.Reader) (*Reader, error) {
	return NewReaderWithLimits(r, DefaultLimits())
}

// NewReaderWithLimits is NewReader with explicit limits.
func NewReaderWithLimits(r io.Reader, limits Limits) (*Reader, error) {
	if r == nil {
		return nil, errors.New("archive: nil reader")
	}
	return newReader(r, limits, nil)
}

func newReader(r io.Reader, limits Limits, closer io.Closer) (*Reader, error) {
	return newStreamReader(r, limits, closer, true)
}

func newStreamReader(r io.Reader, limits Limits, closer io.Closer,
	compressed bool,
) (*Reader, error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}

	raw := &limitReader{
		r:     r,
		kind:  "archive size",
		limit: limits.CompressedSize,
	}
	br := bufio.NewReaderSize(raw, 32*1024)

	var gz *gzip.Reader
	var source io.Reader = br
	if compressed {
		var err error
		gz, err = gzip.NewReader(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("%w: empty gzip stream", ErrInvalid)
			}
			return nil, wrapReadError(err)
		}
		gz.Multistream(false)
		source = gz
	}

	uncompressed := &limitReader{
		r:     source,
		kind:  "uncompressed size",
		limit: limits.UncompressedSize,
	}

	return &Reader{
		br:           br,
		gz:           gz,
		tr:           tar.NewReader(uncompressed),
		uncompressed: uncompressed,
		limits:       limits,
		seen:         make(map[string]seenEntry),
		implied:      make(map[string]string),
		closer:       closer,
	}, nil
}

// OpenTar opens an uncompressed tar file. It is used for release assets such as
// the project localization archive, which are already compressed by the HTTP
// transport or stored as a plain tar.
func OpenTar(name string) (*Reader, error) {
	return OpenTarWithLimits(name, DefaultLimits())
}

// OpenTarWithLimits is OpenTar with explicit limits.
func OpenTarWithLimits(name string, limits Limits) (*Reader, error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("archive: %s is not a regular file", name)
	}
	if info.Size() > limits.CompressedSize {
		_ = f.Close()
		return nil, limitError("archive size", limits.CompressedSize)
	}
	r, err := newStreamReader(f, limits, f, false)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return r, nil
}

// NewTarReader returns a streaming Reader for an uncompressed tar stream.
func NewTarReader(r io.Reader) (*Reader, error) {
	return NewTarReaderWithLimits(r, DefaultLimits())
}

// NewTarReaderWithLimits is NewTarReader with explicit limits.
func NewTarReaderWithLimits(r io.Reader, limits Limits) (*Reader, error) {
	if r == nil {
		return nil, errors.New("archive: nil reader")
	}
	return newStreamReader(r, limits, nil, false)
}

// Next advances to the next archive entry. It returns io.EOF after the tar end
// marker and all trailing checks have passed.
func (r *Reader) Next() (*Entry, error) {
	if r.err != nil {
		return nil, r.err
	}

	hdr, err := r.tr.Next()
	if errors.Is(err, io.EOF) {
		if finishErr := r.finish(); finishErr != nil {
			r.err = finishErr
			return nil, r.err
		}
		r.err = io.EOF
		return nil, io.EOF
	}
	if err != nil {
		r.err = wrapReadError(err)
		return nil, r.err
	}

	if r.entries >= r.limits.Entries {
		r.err = entryLimitError(r.limits.Entries)
		return nil, r.err
	}
	if hdr.Size < 0 {
		r.err = fmt.Errorf("%w: entry %q has negative size", ErrInvalid, hdr.Name)
		return nil, r.err
	}
	if hdr.Size > r.limits.UncompressedSize {
		r.err = limitError("uncompressed size", r.limits.UncompressedSize)
		return nil, r.err
	}

	name, err := pathguard.Path(hdr.Name)
	if err != nil {
		r.err = err
		return nil, r.err
	}

	typ, ok := entryType(hdr)
	if !ok {
		r.err = fmt.Errorf("%w: entry %q has unsupported type %q",
			ErrInvalid, hdr.Name, hdr.Typeflag)
		return nil, r.err
	}

	canonical := strings.ToLower(name)
	if prev, ok := r.seen[canonical]; ok {
		if prev.name == name {
			r.err = fmt.Errorf("%w: duplicate entry %q", ErrInvalid, name)
		} else {
			r.err = fmt.Errorf("%w: ambiguous entry names %q and %q",
				ErrInvalid, prev.name, name)
		}
		return nil, r.err
	}

	for parent := path.Dir(name); parent != "." && parent != "/"; parent = path.Dir(parent) {
		if prev, ok := r.seen[strings.ToLower(parent)]; ok && prev.typ == TypeFile {
			r.err = fmt.Errorf("%w: entry %q is below file %q",
				ErrInvalid, name, prev.name)
			return nil, r.err
		}
	}
	if typ == TypeFile {
		if parent, ok := r.implied[canonical]; ok {
			r.err = fmt.Errorf("%w: file %q conflicts with directory %q",
				ErrInvalid, name, parent)
			return nil, r.err
		}
	}

	r.entries++
	r.seen[canonical] = seenEntry{name: name, typ: typ}
	for parent := path.Dir(name); parent != "." && parent != "/"; parent = path.Dir(parent) {
		key := strings.ToLower(parent)
		if _, ok := r.implied[key]; !ok {
			r.implied[key] = parent
		}
	}

	entry := Entry{
		Name:    name,
		Type:    typ,
		Size:    hdr.Size,
		Mode:    hdr.FileInfo().Mode(),
		ModTime: hdr.ModTime,
	}
	r.current = &entry
	return &entry, nil
}

// Read reads the content of the entry returned by the last Next call.
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.current == nil {
		return 0, errors.New("archive: Read called before Next")
	}

	n, err := r.tr.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = wrapReadError(err)
		return n, r.err
	}
	return n, err
}

// Entries returns the number of accepted entries so far.
func (r *Reader) Entries() int {
	return r.entries
}

// Close releases the gzip reader and the file opened by Open.
func (r *Reader) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true

	var err error
	if r.gz != nil {
		err = r.gz.Close()
	}
	if r.closer != nil {
		if closeErr := r.closer.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func entryType(hdr *tar.Header) (EntryType, bool) {
	switch hdr.Typeflag {
	case tar.TypeReg, tar.TypeRegA:
		return TypeFile, true
	case tar.TypeDir:
		return TypeDir, true
	default:
		return 0, false
	}
}

// finish drains the rest of the gzip stream. Zero padding after the tar end
// marker is valid (GNU tar pads to a 10 KiB record); any other trailing bytes,
// including a second gzip member, are rejected.
func (r *Reader) finish() error {
	if r.finished {
		return r.finishErr
	}
	r.finished = true

	buf := make([]byte, 32*1024)
	for {
		n, err := r.uncompressed.Read(buf)
		for _, b := range buf[:n] {
			if b != 0 {
				r.finishErr = fmt.Errorf("%w: trailing data after tar", ErrInvalid)
				return r.finishErr
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			r.finishErr = wrapReadError(err)
			return r.finishErr
		}
	}

	_, err := r.br.Peek(1)
	switch {
	case err == nil:
		r.finishErr = fmt.Errorf("%w: trailing data after gzip stream", ErrInvalid)
	case errors.Is(err, io.EOF):
		// Normal end of the compressed input.
	default:
		r.finishErr = wrapReadError(err)
	}
	return r.finishErr
}

func wrapReadError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.EOF), errors.Is(err, ErrLimit),
		errors.Is(err, ErrInvalid), errors.Is(err, pathguard.ErrUnsafePath):
		return err
	case errors.Is(err, gzip.ErrHeader), errors.Is(err, gzip.ErrChecksum),
		errors.Is(err, tar.ErrHeader), errors.Is(err, io.ErrUnexpectedEOF):
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	var corrupt flate.CorruptInputError
	if errors.As(err, &corrupt) {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return err
}

func limitError(kind string, limit int64) error {
	return fmt.Errorf("%w: %s exceeds %d bytes", ErrLimit, kind, limit)
}

func entryLimitError(limit int) error {
	return fmt.Errorf("%w: entries exceed %d", ErrLimit, limit)
}

// limitReader stops reading at limit and probes one extra byte to distinguish
// "exactly at the limit" from "over the limit".
type limitReader struct {
	r     io.Reader
	kind  string
	limit int64
	read  int64
}

func (r *limitReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.read >= r.limit {
		var probe [1]byte
		n, err := r.r.Read(probe[:])
		if n > 0 {
			return 0, limitError(r.kind, r.limit)
		}
		return 0, err
	}

	if remaining := r.limit - r.read; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := r.r.Read(p)
	r.read += int64(n)
	return n, err
}
