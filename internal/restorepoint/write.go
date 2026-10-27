package restorepoint

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// SnapshotOptions describes a full snapshot of the live data root.
type SnapshotOptions struct {
	Root         string
	Now          time.Time
	Reason       string
	Label        string
	PanelVersion string
}

// CheckpointOptions describes a world checkpoint and its base snapshot.
type CheckpointOptions struct {
	Root              string
	Now               time.Time
	Reason            string
	Label             string
	Base              Manifest
	BaseName          string
	BaseArchiveSHA256 string
}

// WriteSnapshot streams a snapshot archive. The manifest is written first.
func WriteSnapshot(w io.Writer, opts SnapshotOptions) (Manifest, error) {
	if w == nil {
		return Manifest{}, errors.New("restore point: nil snapshot destination")
	}
	if !validSnapshotReason(opts.Reason) {
		return Manifest{}, invalidf("snapshot reason %q is not manual, mods or base",
			opts.Reason)
	}
	if len(opts.Label) > 64 {
		return Manifest{}, invalidf("snapshot label is longer than 64 characters")
	}

	state, _, err := scanState(opts.Root)
	if err != nil {
		return Manifest{}, err
	}
	mods, err := scanMods(opts.Root)
	if err != nil {
		return Manifest{}, err
	}
	configs, err := scanConfigs(opts.Root)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateLayerLimits(KindSnapshot, state, mods, configs); err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Format:        FormatSnapshot,
		Version:       Version,
		CreatedAt:     opts.Now.UTC(),
		Reason:        opts.Reason,
		Label:         opts.Label,
		PanelVersion:  opts.PanelVersion,
		ModsSHA256:    layerDigest("mods", mods),
		StateSHA256:   layerDigest("state", state),
		ConfigsSHA256: layerDigest("configs", configs),
	}
	tw, closeArchive, err := newTarWriter(w)
	if err != nil {
		return Manifest{}, err
	}
	fail := func(writeErr error) (Manifest, error) {
		_ = closeArchive()
		return Manifest{}, writeErr
	}

	if err := writeManifest(tw, manifest); err != nil {
		return fail(err)
	}
	if err := writeSnapshotState(tw, state, manifest.CreatedAt); err != nil {
		return fail(err)
	}
	if err := writeLayer(tw, "mods", mods, manifest.CreatedAt); err != nil {
		return fail(err)
	}
	if err := writeLayer(tw, "configs", configs, manifest.CreatedAt); err != nil {
		return fail(err)
	}
	if err := closeArchive(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// WriteCheckpoint streams a checkpoint archive from the live world. The base
// snapshot must already exist; its metadata is copied into the manifest.
func WriteCheckpoint(w io.Writer, opts CheckpointOptions) (Manifest, error) {
	if w == nil {
		return Manifest{}, errors.New("restore point: nil checkpoint destination")
	}
	if !validCheckpointReason(opts.Reason) {
		return Manifest{}, invalidf("checkpoint reason %q is not timer, manual or predeploy",
			opts.Reason)
	}
	if len(opts.Label) > 64 {
		return Manifest{}, invalidf("checkpoint label is longer than 64 characters")
	}
	if opts.BaseName == "" || !validSHA256(opts.BaseArchiveSHA256) ||
		!validSHA256(opts.Base.ModsSHA256) || !validSHA256(opts.Base.StateSHA256) {
		return Manifest{}, invalidf("checkpoint base metadata is incomplete")
	}

	state, players, err := scanState(opts.Root)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateLayerLimits(KindCheckpoint, state); err != nil {
		return Manifest{}, err
	}
	playerCount := players
	manifest := Manifest{
		Format:    FormatCheckpoint,
		Version:   Version,
		CreatedAt: opts.Now.UTC(),
		Reason:    opts.Reason,
		Label:     opts.Label,
		Base: &Base{
			Name:        opts.BaseName,
			SHA256:      opts.BaseArchiveSHA256,
			ModsSHA256:  opts.Base.ModsSHA256,
			StateSHA256: opts.Base.StateSHA256,
		},
		Players:     &playerCount,
		WorldSHA256: layerDigest("state", state),
	}

	tw, closeArchive, err := newTarWriter(w)
	if err != nil {
		return Manifest{}, err
	}
	fail := func(writeErr error) (Manifest, error) {
		_ = closeArchive()
		return Manifest{}, writeErr
	}

	if err := writeManifest(tw, manifest); err != nil {
		return fail(err)
	}
	if err := writeCheckpointState(tw, state, manifest.CreatedAt); err != nil {
		return fail(err)
	}
	if err := closeArchive(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validSnapshotReason(reason string) bool {
	return reason == "manual" || reason == "mods" || reason == "base"
}

func validCheckpointReason(reason string) bool {
	return reason == "timer" || reason == "manual" || reason == "predeploy"
}

func newTarWriter(w io.Writer) (*tar.Writer, func() error, error) {
	gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if err != nil {
		return nil, nil, err
	}
	tw := tar.NewWriter(gz)
	closeAll := func() error {
		tarErr := tw.Close()
		gzErr := gz.Close()
		return errors.Join(tarErr, gzErr)
	}
	return tw, closeAll, nil
}

func writeSnapshotState(tw *tar.Writer, files []layerFile, modTime time.Time) error {
	for _, dir := range []string{
		"state/", "state/players/", "state/world/",
		"state/world/cell/", "state/world/world/", "state/world/map/",
		"state/world/recordstore/", "state/world/custom/",
	} {
		if err := writeTarDir(tw, dir, modTime); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := writeTarSource(tw, "state/"+file.Path, file, modTime); err != nil {
			return err
		}
	}
	return nil
}

func writeCheckpointState(tw *tar.Writer, files []layerFile, modTime time.Time) error {
	for _, dir := range []string{
		"players/", "world/",
		"world/cell/", "world/world/", "world/map/",
		"world/recordstore/", "world/custom/",
	} {
		if err := writeTarDir(tw, dir, modTime); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := writeTarSource(tw, file.Path, file, modTime); err != nil {
			return err
		}
	}
	return nil
}

func writeLayer(tw *tar.Writer, layer string, files []layerFile, modTime time.Time) error {
	dirs := []string{layer + "/"}
	if layer == "mods" {
		dirs = append(dirs, "mods/plugins/", "mods/scripts/")
	}
	for _, dir := range dirs {
		if err := writeTarDir(tw, dir, modTime); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := writeTarSource(tw, layer+"/"+file.Path, file, modTime); err != nil {
			return err
		}
	}
	return nil
}

func writeTarDir(tw *tar.Writer, name string, modTime time.Time) error {
	header := &tar.Header{
		Name:     name,
		Typeflag: tar.TypeDir,
		Mode:     0o755,
		ModTime:  modTime.UTC(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func writeTarFile(w io.Writer, name string, mode os.FileMode, modTime time.Time, data []byte) error {
	tw, closeArchive, err := newTarWriter(w)
	if err != nil {
		return err
	}
	if err := writeTarBytes(tw, name, mode, modTime, data); err != nil {
		_ = closeArchive()
		return err
	}
	return closeArchive()
}

func writeTarBytes(tw *tar.Writer, name string, mode os.FileMode, modTime time.Time,
	data []byte,
) error {
	header := &tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     int64(mode.Perm()),
		Size:     int64(len(data)),
		ModTime:  modTime.UTC(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func writeTarSource(tw *tar.Writer, name string, file layerFile, modTime time.Time) error {
	header := &tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     file.Size,
		ModTime:  modTime.UTC(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	source, err := os.Open(file.source)
	if err != nil {
		return err
	}
	defer source.Close()

	written, err := io.Copy(tw, source)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if written != file.Size {
		return invalidf("%s: size changed while writing", file.source)
	}
	return nil
}
