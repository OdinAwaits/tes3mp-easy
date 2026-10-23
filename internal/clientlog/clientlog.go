// Package clientlog writes the per-user client log.
//
// The log is intentionally small and self-contained: one append-only file in
// the client data directory, rotated once at a fixed size. It contains error
// details and paths useful for support but never server passwords or profile
// records.
package clientlog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// FileName is the primary client log name inside the logs directory.
	FileName = "client.log"

	// RotatedName keeps the previous log after a size-based rotation.
	RotatedName = FileName + ".1"

	// MaxSize is the size at which the log is rotated. A single client session
	// can therefore leave at most about MaxSize*2 bytes on disk.
	MaxSize int64 = 2 << 20

	// DirMode and FileMode keep diagnostics private to the current user.
	DirMode  os.FileMode = 0o700
	FileMode os.FileMode = 0o600
)

// Logger is a concurrency-safe append-only client log.
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	size    int64
	maxSize int64
}

// Open creates dir if needed and appends to dir/client.log. When the existing
// log has reached MaxSize it is moved to client.log.1 before opening a new one.
// A rotation failure is returned rather than silently discarding diagnostics.
func Open(dir string) (*Logger, error) {
	return open(dir, MaxSize)
}

func open(dir string, maxSize int64) (*Logger, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("client log directory is empty")
	}
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("client log directory %q is not absolute", dir)
	}
	if maxSize <= 0 {
		return nil, errors.New("client log size limit must be positive")
	}
	if err := os.MkdirAll(dir, DirMode); err != nil {
		return nil, fmt.Errorf("create client log directory: %w", err)
	}
	path := filepath.Join(filepath.Clean(dir), FileName)
	if err := rotatePath(path, maxSize); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, FileMode)
	if err != nil {
		return nil, fmt.Errorf("open client log: %w", err)
	}
	if err := file.Chmod(FileMode); err != nil {
		file.Close()
		return nil, fmt.Errorf("secure client log: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("inspect client log: %w", err)
	}
	logger := &Logger{
		file:    file,
		path:    path,
		size:    info.Size(),
		maxSize: maxSize,
	}
	logger.Infof("client log opened")
	return logger, nil
}

func rotatePath(path string, maxSize int64) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect client log: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("client log %q is a directory", path)
	}
	if info.Size() < maxSize {
		return nil
	}
	rotated := filepath.Join(filepath.Dir(path), RotatedName)
	if err := os.Remove(rotated); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous client log rotation: %w", err)
	}
	if err := os.Rename(path, rotated); err != nil {
		return fmt.Errorf("rotate client log: %w", err)
	}
	return nil
}

// Path returns the primary log path.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Infof writes an informational entry.
func (l *Logger) Infof(format string, args ...any) {
	l.log("INFO", format, args...)
}

// Warnf writes a warning entry.
func (l *Logger) Warnf(format string, args ...any) {
	l.log("WARN", format, args...)
}

// Errorf writes an error entry.
func (l *Logger) Errorf(format string, args ...any) {
	l.log("ERROR", format, args...)
}

func (l *Logger) log(level, format string, args ...any) {
	if l == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	message = strings.ReplaceAll(message, "\r", " ")
	message = strings.ReplaceAll(message, "\n", "\\n")
	line := fmt.Sprintf("%s | %-5s | %s\n",
		time.Now().UTC().Format(time.RFC3339), level, message)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if l.size >= l.maxSize && !l.rotateLocked() {
		return
	}
	written, err := l.file.WriteString(line)
	l.size += int64(written)
	if err != nil {
		// Logging must never disturb a setup or launch operation. The next
		// write may succeed and the on-disk path remains useful to support.
		return
	}
}

func (l *Logger) rotateLocked() bool {
	if err := l.file.Close(); err != nil {
		return false
	}
	l.file = nil
	if err := rotatePath(l.path, 0); err != nil {
		return false
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, FileMode)
	if err != nil {
		return false
	}
	if err := file.Chmod(FileMode); err != nil {
		file.Close()
		return false
	}
	l.file = file
	l.size = 0
	return true
}

// Close flushes and closes the log. It is safe to call more than once.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// Tail returns up to maxLines final lines of the log at path. A missing log is
// an empty result, not an error. The file is bounded by MaxSize, so reading it
// for the Logs dialog stays inexpensive.
func Tail(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return nil, errors.New("log line limit must be positive")
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open client log: %w", err)
	}
	defer file.Close()

	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(io.LimitReader(file, MaxSize+1))
	scanner.Buffer(make([]byte, 64*1024), int(MaxSize))
	for scanner.Scan() {
		if len(lines) == maxLines {
			copy(lines, lines[1:])
			lines[len(lines)-1] = scanner.Text()
			continue
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read client log: %w", err)
	}
	return lines, nil
}
