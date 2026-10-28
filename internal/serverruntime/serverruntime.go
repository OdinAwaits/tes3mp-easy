// Package serverruntime reads the metadata of the installed TES3MP server.
//
// bootstrap.sh unpacks the release into a root-owned /opt/tes3mp-easy/tes3mp and
// writes .tes3mp-install-info inside staging, before the atomic rename. The
// marker therefore exists only in a complete install, and it is the only place
// that records which release is actually on disk: the panel reads it instead of
// shelling out to ops, and the compiled-in expectation (internal/tes3mp.Version
// is the client release) is not consulted.
package serverruntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultRoot is the production runtime directory created by bootstrap.sh.
	DefaultRoot = "/opt/tes3mp-easy/tes3mp"

	// InfoFile is the install marker written by bootstrap.sh inside the runtime.
	InfoFile = ".tes3mp-install-info"

	// maxInfoBytes bounds the marker read; the real file is three short lines.
	maxInfoBytes = 4 << 10
)

// Info is the installed server release.
type Info struct {
	// Version is the TES3MP server release, for example "0.8.1".
	Version string
	// URL and SHA256 describe the archive the runtime was unpacked from.
	URL    string
	SHA256 string
}

// ReadInfo parses the install marker inside root. A missing or malformed marker
// is an error: callers show an unknown version instead of guessing, because a
// runtime directory without the marker predates the current bootstrap and its
// release is unknowable from the tree alone.
func ReadInfo(root string) (Info, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Info{}, errors.New("runtime root is empty")
	}
	if !filepath.IsAbs(root) {
		return Info{}, fmt.Errorf("runtime root %q is not absolute", root)
	}
	path := filepath.Join(root, InfoFile)
	marker, err := os.Lstat(path)
	if err != nil {
		return Info{}, fmt.Errorf("read TES3MP install marker: %w", err)
	}
	if !marker.Mode().IsRegular() {
		return Info{}, fmt.Errorf("TES3MP install marker %s is not a regular file", path)
	}
	if marker.Size() > maxInfoBytes {
		return Info{}, fmt.Errorf("TES3MP install marker is %d bytes, limit is %d",
			marker.Size(), maxInfoBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Info{}, fmt.Errorf("read TES3MP install marker: %w", err)
	}

	info := Info{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "TES3MP_VERSION":
			info.Version = strings.TrimSpace(value)
		case "TES3MP_URL":
			info.URL = strings.TrimSpace(value)
		case "TES3MP_SHA256":
			info.SHA256 = strings.TrimSpace(value)
		}
	}
	if info.Version == "" {
		return Info{}, fmt.Errorf("TES3MP install marker %s has no TES3MP_VERSION", path)
	}
	return info, nil
}
