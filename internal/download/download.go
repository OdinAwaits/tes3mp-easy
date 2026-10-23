// Package download fetches release assets and verifies their SHA256 before
// exposing them at the destination path.
//
// The package is shared by the font and localization installers. It never
// follows a symlink at the destination, bounds the response size, writes
// through internal/atomicfile and verifies the digest before the rename, so a
// partial or corrupted file is never observed by the caller.
package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

const (
	// DefaultMaxSize bounds one release asset. The Russian voice archive is
	// about 406 MiB; 1 GiB leaves room for future releases without allowing
	// an unbounded response.
	DefaultMaxSize int64 = 1 << 30

	defaultAttempts = 3
)

// Progress is one download update.
type Progress struct {
	Downloaded int64
	Total      int64
}

// Options describes one verified download. URL, SHA256 and Dest are required.
type Options struct {
	URL       string
	SHA256    string
	Dest      string
	MaxSize   int64
	Client    *http.Client
	Attempts  int
	RetryWait time.Duration

	Progress func(Progress)
}

// Result describes a cached or newly downloaded asset.
type Result struct {
	Path       string
	Size       int64
	Cached     bool
	Downloaded bool
}

// Fetch returns Dest, downloading URL first when the cached file is missing or
// does not match SHA256. A cached file with the wrong digest is removed and
// replaced; a network failure leaves Dest untouched.
func Fetch(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options = options.withDefaults()
	if err := options.validate(); err != nil {
		return Result{}, err
	}

	dest, err := validateDest(options.Dest)
	if err != nil {
		return Result{}, err
	}
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return Result{}, fmt.Errorf(
				"download destination %s is not a regular file", dest)
		}
		if digest, size, err := FileSHA256(dest); err == nil &&
			strings.EqualFold(digest, options.SHA256) {
			return Result{Path: dest, Size: size, Cached: true}, nil
		}
		if err := os.Remove(dest); err != nil {
			return Result{}, fmt.Errorf("remove stale download %s: %w", dest, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("inspect download destination %s: %w", dest, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return Result{}, fmt.Errorf("create download directory: %w", err)
	}

	var last error
	for attempt := 1; attempt <= options.Attempts; attempt++ {
		size, retry, err := options.fetchOnce(ctx, dest)
		if err == nil {
			return Result{Path: dest, Size: size, Downloaded: true}, nil
		}
		last = err
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if !retry || attempt == options.Attempts {
			break
		}
		timer := time.NewTimer(options.RetryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	return Result{}, fmt.Errorf("download %s after %d attempt(s): %w",
		options.URL, options.Attempts, last)
}

// FileSHA256 returns the lowercase hex digest and size of a regular file.
func FileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func (o Options) withDefaults() Options {
	if o.MaxSize == 0 {
		o.MaxSize = DefaultMaxSize
	}
	if o.Attempts == 0 {
		o.Attempts = defaultAttempts
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 30 * time.Minute}
	}
	return o
}

func (o Options) validate() error {
	parsed, err := url.Parse(o.URL)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if parsed.User != nil {
		return errors.New("download URL must not contain credentials")
	}
	if parsed.Fragment != "" {
		return errors.New("download URL must not contain a fragment")
	}
	if parsed.Hostname() == "" {
		return errors.New("download URL has no host")
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopback(parsed.Hostname()) {
			return errors.New("download requires HTTPS")
		}
	default:
		return fmt.Errorf("unsupported download URL scheme %q", parsed.Scheme)
	}
	if len(o.SHA256) != sha256.Size*2 {
		return errors.New("download SHA256 must be 64 hex characters")
	}
	if _, err := hex.DecodeString(o.SHA256); err != nil {
		return errors.New("download SHA256 must be hexadecimal")
	}
	if strings.TrimSpace(o.Dest) == "" {
		return errors.New("download destination is empty")
	}
	if o.MaxSize < 1 {
		return errors.New("download size limit must be positive")
	}
	if o.Attempts < 1 {
		return errors.New("download attempts must be positive")
	}
	if o.RetryWait < 0 {
		return errors.New("download retry delay must not be negative")
	}
	return nil
}

func validateDest(dest string) (string, error) {
	if !filepath.IsAbs(dest) {
		return "", fmt.Errorf("download destination %q is not absolute", dest)
	}
	clean := filepath.Clean(dest)
	if err := pathguard.Base(filepath.Base(clean)); err != nil {
		return "", fmt.Errorf("download destination: %w", err)
	}
	return clean, nil
}

func (o Options) fetchOnce(ctx context.Context, dest string) (int64, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL, nil)
	if err != nil {
		return 0, false, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("User-Agent", "tes3mp-easy-client")
	response, err := o.Client.Do(request)
	if err != nil {
		return 0, true, fmt.Errorf("request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return 0, response.StatusCode >= 500,
			fmt.Errorf("server returned %s", response.Status)
	}
	total := response.ContentLength
	if total > o.MaxSize {
		return 0, false, fmt.Errorf("response is %d bytes, limit is %d",
			total, o.MaxSize)
	}

	writer, err := atomicfile.New(dest, 0o600)
	if err != nil {
		return 0, false, err
	}
	hash := sha256.New()
	counter := &progressWriter{
		writer: io.MultiWriter(writer, hash),
		total:  total,
		report: o.Progress,
	}
	limited := &io.LimitedReader{R: response.Body, N: o.MaxSize + 1}
	written, copyErr := io.CopyBuffer(counter, limited, make([]byte, 64*1024))
	if copyErr != nil {
		return written, true, errors.Join(
			fmt.Errorf("read response: %w", copyErr),
			writer.Abort(),
		)
	}
	if written > o.MaxSize {
		return written, false, errors.Join(
			fmt.Errorf("response exceeds %d bytes", o.MaxSize),
			writer.Abort(),
		)
	}
	if total >= 0 && written != total {
		return written, true, errors.Join(
			fmt.Errorf("response is %d bytes, Content-Length is %d", written, total),
			writer.Abort(),
		)
	}
	counter.flush()
	digest := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(digest, o.SHA256) {
		return written, false, errors.Join(
			fmt.Errorf("SHA256 mismatch: got %s", digest),
			writer.Abort(),
		)
	}
	if err := writer.Commit(); err != nil {
		return 0, false, err
	}
	return written, false, nil
}

type progressWriter struct {
	writer io.Writer
	done   int64
	total  int64
	last   time.Time
	report func(Progress)
}

func (w *progressWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	w.done += int64(n)
	now := time.Now()
	if w.report != nil && (w.last.IsZero() ||
		now.Sub(w.last) >= 200*time.Millisecond ||
		(w.total > 0 && w.done >= w.total)) {
		w.last = now
		w.report(Progress{Downloaded: w.done, Total: w.total})
	}
	return n, err
}

func (w *progressWriter) flush() {
	if w.report != nil {
		w.report(Progress{Downloaded: w.done, Total: w.total})
	}
}

func isLoopback(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
