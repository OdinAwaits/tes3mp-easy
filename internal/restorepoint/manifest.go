package restorepoint

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

const (
	// FormatSnapshot identifies a full snapshot manifest.
	FormatSnapshot = "tes3mp-easy-snapshot"
	// FormatCheckpoint identifies a checkpoint manifest.
	FormatCheckpoint = "tes3mp-easy-checkpoint"
	// Version is the only manifest version understood by this package.
	Version = 1

	manifestName    = "manifest.json"
	MaxManifestSize = 1 << 20
)

// Base describes the snapshot a checkpoint continues.
type Base struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	ModsSHA256  string `json:"mods_sha256"`
	StateSHA256 string `json:"state_sha256"`
}

// Manifest is the first entry of every snapshot and checkpoint. Snapshot and
// checkpoint fields are disjoint; omitempty keeps the JSON from mixing them.
type Manifest struct {
	Format    string    `json:"format"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Reason    string    `json:"reason"`
	Label     string    `json:"label"`

	// Snapshot fields.
	PanelVersion  string `json:"panel_version,omitempty"`
	ModsSHA256    string `json:"mods_sha256,omitempty"`
	StateSHA256   string `json:"state_sha256,omitempty"`
	ConfigsSHA256 string `json:"configs_sha256,omitempty"`

	// Checkpoint fields.
	Base        *Base  `json:"base,omitempty"`
	Players     *int   `json:"players,omitempty"`
	WorldSHA256 string `json:"world_sha256,omitempty"`
}

// IsSnapshot reports whether the manifest describes a full snapshot.
func (m Manifest) IsSnapshot() bool {
	return m.Format == FormatSnapshot
}

// File is one regular file in a layer.
type File struct {
	Path   string
	Size   int64
	SHA256 string
}

// Digest returns sha256(layer "\n" + Σ path "\0" size "\0" file_sha256 "\n")
// over files sorted by path.
func Digest(layer string, files []File) string {
	sorted := append([]File(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	sum := sha256.New()
	_, _ = io.WriteString(sum, layer+"\n")
	for _, file := range sorted {
		_, _ = io.WriteString(sum, file.Path)
		_, _ = sum.Write([]byte{0})
		_, _ = io.WriteString(sum, fmt.Sprintf("%d", file.Size))
		_, _ = sum.Write([]byte{0})
		_, _ = io.WriteString(sum, file.SHA256)
		_, _ = sum.Write([]byte{'\n'})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// Limits returns the per-kind archive limits from §6.7.
func Limits(kind Kind) archive.Limits {
	if kind == KindSnapshot {
		return archive.Limits{
			CompressedSize:   1 << 30,
			UncompressedSize: 2 << 30,
			Entries:          100_000,
		}
	}
	return archive.Limits{
		CompressedSize:   256 << 20,
		UncompressedSize: 512 << 20,
		Entries:          50_000,
	}
}

// ReadManifest reads only the first entry of an archive. It still rejects an
// unsafe or oversized manifest before decoding it.
func ReadManifest(path string) (Manifest, error) {
	kind := parsedKindFor(path)
	reader, err := archive.OpenWithLimits(path, Limits(kind))
	if err != nil {
		return Manifest{}, err
	}
	defer reader.Close()

	entry, err := reader.Next()
	if err != nil {
		return Manifest{}, err
	}
	if entry.Type != archive.TypeFile || entry.Name != manifestName {
		return Manifest{}, invalidf("%s: first entry must be %s", path, manifestName)
	}
	if entry.Size > MaxManifestSize {
		return Manifest{}, fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, manifestName, MaxManifestSize)
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxManifestSize+1))
	if err != nil {
		return Manifest{}, err
	}
	if int64(len(data)) > MaxManifestSize {
		return Manifest{}, fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, manifestName, MaxManifestSize)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, invalidf("%s: %v", manifestName, err)
	}
	if err := manifest.validate(kind); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func parsedKindFor(path string) Kind {
	parsed, err := ParseName(filepathBase(path))
	if err != nil {
		return KindCheckpoint
	}
	return parsed.Kind
}

func filepathBase(path string) string {
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		return path[index+1:]
	}
	return path
}

func (m Manifest) validate(kind Kind) error {
	if m.Version != Version {
		return invalidf("%s: unsupported version %d", manifestName, m.Version)
	}
	if len(m.Label) > 64 {
		return invalidf("%s: label is longer than 64 characters", manifestName)
	}
	if kind == KindSnapshot {
		if m.Format != FormatSnapshot {
			return invalidf("%s: format %q is not a snapshot", manifestName, m.Format)
		}
		if !validSHA256(m.ModsSHA256) || !validSHA256(m.StateSHA256) ||
			!validSHA256(m.ConfigsSHA256) {
			return invalidf("%s: snapshot digests must be 64 lowercase hex characters",
				manifestName)
		}
		if m.Base != nil || m.Players != nil || m.WorldSHA256 != "" {
			return invalidf("%s: snapshot contains checkpoint fields", manifestName)
		}
		return nil
	}

	if m.Format != FormatCheckpoint {
		return invalidf("%s: format %q is not a checkpoint", manifestName, m.Format)
	}
	if m.Base == nil || m.Players == nil || !validSHA256(m.WorldSHA256) {
		return invalidf("%s: checkpoint is missing base, players or world_sha256",
			manifestName)
	}
	if m.Base.Name == "" || !validSHA256(m.Base.SHA256) ||
		!validSHA256(m.Base.ModsSHA256) || !validSHA256(m.Base.StateSHA256) {
		return invalidf("%s: checkpoint base is invalid", manifestName)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func writeManifest(tw *tar.Writer, manifest Manifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > MaxManifestSize {
		return fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, manifestName, MaxManifestSize)
	}
	return writeTarBytes(tw, manifestName, 0o644, manifest.CreatedAt, data)
}

func fileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return false, invalidf("%s: not a regular file", path)
	default:
		return true, nil
	}
}
