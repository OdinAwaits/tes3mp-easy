package serverapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// DownloadProgress is one archive download update.
type DownloadProgress struct {
	Downloaded int64
	Total      int64
}

// DownloadMods downloads the current mods archive to dest, enforcing the size
// reported by the API. The caller must verify SHA256 after the function returns;
// this layer guarantees that the body is complete and bounded.
func (c *Client) DownloadMods(ctx context.Context, mods Mods, dest string,
	progress func(DownloadProgress),
) (int64, error) {
	if c == nil {
		return 0, errors.New("server API client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !filepath.IsAbs(dest) {
		return 0, fmt.Errorf("mods cache path %q is not absolute", dest)
	}
	if mods.Size < 1 || mods.Size > archive.MaxCompressedSize {
		return 0, fmt.Errorf("mods size %d is invalid", mods.Size)
	}
	archiveURL, err := c.ArchiveURL(mods)
	if err != nil {
		return 0, err
	}

	var last error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		written, err := c.downloadModsOnce(ctx, archiveURL, mods.Size, dest,
			progress)
		if err == nil {
			return written, nil
		}
		last = err
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if !retryable(err) || attempt == c.attempts {
			break
		}
		timer := time.NewTimer(c.retryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
	return 0, fmt.Errorf("download mods after %d attempt(s): %w",
		c.attempts, last)
}

func (c *Client) downloadModsOnce(ctx context.Context, archiveURL *url.URL,
	size int64, dest string, report func(DownloadProgress),
) (int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		archiveURL.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("create mods request: %w", err)
	}
	request.Header.Set("Accept", "application/gzip")
	request.Header.Set("User-Agent", "tes3mp-easy-client")

	response, err := c.http.Do(request)
	if err != nil {
		return 0, fmt.Errorf("download mods: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
		return 0, parseAPIError(response.StatusCode, body)
	}
	if response.ContentLength != size {
		return 0, fmt.Errorf(
			"mods archive Content-Length is %d, API reported %d",
			response.ContentLength, size)
	}

	writer, err := atomicfile.New(dest, 0o600)
	if err != nil {
		return 0, err
	}
	counter := &downloadWriter{
		writer: writer,
		total:  size,
		report: report,
	}
	limited := &io.LimitedReader{R: response.Body, N: size + 1}
	buf := make([]byte, 64*1024)
	written, copyErr := io.CopyBuffer(counter, limited, buf)
	if copyErr != nil {
		return written, errors.Join(
			fmt.Errorf("download mods: %w", copyErr),
			writer.Abort(),
		)
	}
	if written != size {
		return written, errors.Join(
			fmt.Errorf("mods archive is %d bytes, API reported %d", written, size),
			writer.Abort(),
		)
	}
	counter.flush()
	if err := writer.Commit(); err != nil {
		return 0, err
	}
	return written, nil
}

type downloadWriter struct {
	writer io.Writer
	done   int64
	total  int64
	last   time.Time
	report func(DownloadProgress)
}

func (w *downloadWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	w.done += int64(n)
	now := time.Now()
	if w.report != nil && (w.last.IsZero() ||
		now.Sub(w.last) >= 200*time.Millisecond ||
		(w.total > 0 && w.done >= w.total)) {
		w.last = now
		w.report(DownloadProgress{Downloaded: w.done, Total: w.total})
	}
	return n, err
}

func (w *downloadWriter) flush() {
	if w.report == nil {
		return
	}
	w.report(DownloadProgress{Downloaded: w.done, Total: w.total})
}
