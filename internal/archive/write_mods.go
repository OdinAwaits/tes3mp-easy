package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// WriteModsDir writes a mods archive from a validated live mods tree. The
// caller owns the destination and its fsync/rename. Entries are sorted so the
// archive is deterministic apart from mtimes.
func WriteModsDir(w io.Writer, modsDir string, modTime time.Time) error {
	if w == nil {
		return errors.New("archive: nil mods destination")
	}
	if _, err := ValidateModsDir(modsDir); err != nil {
		return err
	}
	if modTime.IsZero() {
		modTime = time.Now().UTC()
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	fail := func(err error) error {
		_ = tw.Close()
		_ = gz.Close()
		return err
	}

	for _, dir := range []string{"plugins", "scripts"} {
		if err := tw.WriteHeader(&tar.Header{
			Name:     dir + "/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
			ModTime:  modTime.UTC(),
		}); err != nil {
			return fail(err)
		}
		entries, err := os.ReadDir(filepath.Join(modsDir, dir))
		if err != nil {
			return fail(err)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, entry := range entries {
			if err := writeModsFile(tw, modsDir, dir, entry, modTime); err != nil {
				return fail(err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func writeModsFile(tw *tar.Writer, modsDir, dir string, entry fs.DirEntry,
	modTime time.Time,
) error {
	source := filepath.Join(modsDir, dir, entry.Name())
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file", source)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:     dir + "/" + entry.Name(),
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     info.Size(),
		ModTime:  modTime.UTC(),
	}); err != nil {
		return err
	}
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(tw, file)
	return err
}
