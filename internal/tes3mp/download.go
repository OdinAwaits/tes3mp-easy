package tes3mp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("server returned HTTP %d %s",
		e.code, http.StatusText(e.code))
}

func (o Options) download(ctx context.Context, rawURL, destination string) error {
	if err := validateURL(rawURL); err != nil {
		return err
	}
	client := o.httpClient()
	var last error
	attempts := 0
	for attempt := 1; attempt <= o.Attempts; attempt++ {
		attempts = attempt
		err := o.downloadOnce(ctx, client, rawURL, destination)
		if err == nil {
			return nil
		}
		last = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retryableDownload(err) || attempt == o.Attempts {
			break
		}
		timer := time.NewTimer(o.RetryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("download TES3MP after %d attempt(s): %w",
		attempts, last)
}

func (o Options) httpClient() *http.Client {
	client := *o.Client
	previous := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := validateURL(request.URL.String()); err != nil {
			return err
		}
		if previous != nil {
			return previous(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &client
}

func (o Options) downloadOnce(ctx context.Context, client *http.Client,
	rawURL, destination string,
) error {
	if err := validateURL(rawURL); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create TES3MP request: %w", err)
	}
	request.Header.Set("User-Agent", "tes3mp-easy-client")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download TES3MP: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return &statusError{code: response.StatusCode}
	}
	if response.ContentLength > MaxDownloadSize {
		return fmt.Errorf("TES3MP archive is larger than %d bytes",
			MaxDownloadSize)
	}

	writer, err := atomicfile.New(destination, 0o600)
	if err != nil {
		return err
	}
	progress := &progressWriter{
		writer: writer,
		total:  response.ContentLength,
		report: o.Progress,
	}
	limited := &io.LimitedReader{R: response.Body, N: MaxDownloadSize + 1}
	buf := make([]byte, 64*1024)
	written, copyErr := io.CopyBuffer(progress, limited, buf)
	if copyErr != nil {
		return errors.Join(
			fmt.Errorf("download TES3MP: %w", copyErr),
			writer.Abort(),
		)
	}
	if written > MaxDownloadSize {
		return errors.Join(
			fmt.Errorf("TES3MP archive is larger than %d bytes",
				MaxDownloadSize),
			writer.Abort(),
		)
	}
	progress.flush()
	if err := writer.Commit(); err != nil {
		return err
	}
	return nil
}

func retryableDownload(err error) bool {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var status *statusError
	if errors.As(err, &status) {
		return status.code == http.StatusRequestTimeout ||
			status.code == http.StatusTooManyRequests ||
			status.code >= 500
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return false
	}
	return true
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
		w.report(Progress{
			Stage:      StageDownloading,
			Downloaded: w.done,
			Total:      w.total,
		})
	}
	return n, err
}

func (w *progressWriter) flush() {
	if w.report == nil {
		return
	}
	w.report(Progress{
		Stage:      StageDownloading,
		Downloaded: w.done,
		Total:      w.total,
	})
}
