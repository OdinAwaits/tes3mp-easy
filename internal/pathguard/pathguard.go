// Package pathguard validates untrusted names and archive paths before they
// reach the filesystem.
package pathguard

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrUnsafePath marks every validation failure from this package.
var ErrUnsafePath = errors.New("unsafe path")

// Error describes why a name or path is unsafe.
type Error struct {
	Name   string
	Reason string
}

// Error implements error.
func (e *Error) Error() string {
	if e.Name == "" {
		return "unsafe path: " + e.Reason
	}
	return fmt.Sprintf("unsafe path %q: %s", e.Name, e.Reason)
}

// Unwrap allows errors.Is(err, ErrUnsafePath).
func (e *Error) Unwrap() error {
	return ErrUnsafePath
}

// Base validates a single path component: an archive filename, a plugin name or
// a script name. It rejects separators, NUL, "." and "..".
func Base(name string) error {
	switch {
	case name == "":
		return &Error{Name: name, Reason: "empty name"}
	case strings.ContainsRune(name, '\x00'):
		return &Error{Name: name, Reason: "contains NUL"}
	case name == "." || name == "..":
		return &Error{Name: name, Reason: "relative path component"}
	case strings.ContainsAny(name, `/\`):
		return &Error{Name: name, Reason: "path separator"}
	case hasDrivePrefix(name):
		return &Error{Name: name, Reason: "absolute path"}
	}
	return nil
}

// Path validates an entry name from a tar archive and returns its cleaned
// slash-separated form. The returned path is relative, has no empty components
// and contains no ".." elements.
func Path(name string) (string, error) {
	switch {
	case name == "":
		return "", &Error{Name: name, Reason: "empty path"}
	case strings.ContainsRune(name, '\x00'):
		return "", &Error{Name: name, Reason: "contains NUL"}
	case strings.ContainsRune(name, '\\'):
		return "", &Error{Name: name, Reason: "backslash separator"}
	case strings.HasPrefix(name, "/"):
		return "", &Error{Name: name, Reason: "absolute path"}
	case hasDrivePrefix(name):
		return "", &Error{Name: name, Reason: "absolute path"}
	}

	trimmed := strings.TrimRight(name, "/")
	if trimmed == "" {
		return "", &Error{Name: name, Reason: "empty path"}
	}
	for _, part := range strings.Split(trimmed, "/") {
		if part == ".." {
			return "", &Error{Name: name, Reason: "parent path component"}
		}
	}

	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", &Error{Name: name, Reason: "relative path component"}
	}
	return cleaned, nil
}

func hasDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	c := name[0]
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
