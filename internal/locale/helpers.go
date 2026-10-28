package locale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/download"
)

func (o Options) withDefaults() Options {
	if strings.TrimSpace(o.Language) == "" {
		o.Language = LanguageRussian
	}
	o.Language, _ = ParseLanguage(o.Language)
	if o.Language == "" {
		o.Language = LanguageRussian
	}
	pack, _ := PackFor(o.Language)
	if o.URL == "" {
		o.URL = pack.URL
	}
	if o.SHA256 == "" {
		o.SHA256 = pack.SHA256
	}
	if o.VoicesURL == "" {
		o.VoicesURL = pack.VoicesURL
	}
	if o.VoicesSHA256 == "" {
		o.VoicesSHA256 = pack.VoicesSHA256
	}
	if o.Attempts == 0 {
		o.Attempts = 3
	}
	return o
}

func (o Options) validate() error {
	if _, err := validateWorkDir(o.WorkDir); err != nil {
		return err
	}
	if strings.TrimSpace(o.CacheDir) == "" {
		return errors.New("client cache directory is empty")
	}
	if !filepath.IsAbs(o.CacheDir) {
		return fmt.Errorf("client cache directory %q is not absolute", o.CacheDir)
	}
	pack, ok := PackFor(o.Language)
	if !ok {
		return fmt.Errorf("unsupported game language %q", o.Language)
	}
	if pack.ArchiveName == "" {
		return fmt.Errorf("game language %q has no downloadable pack", o.Language)
	}
	if len(o.SHA256) != 64 {
		return errors.New("localization SHA256 must be 64 hex characters")
	}
	if o.Voiceover {
		if len(o.VoicesSHA256) != 64 {
			return errors.New(
				"voice archive SHA256 is not pinned for this release")
		}
		if strings.TrimSpace(o.VoicesURL) == "" {
			return errors.New("voice archive URL is empty")
		}
	}
	if o.Attempts < 1 {
		return errors.New("localization download attempts must be positive")
	}
	if o.RetryWait < 0 {
		return errors.New("localization retry delay must not be negative")
	}
	return nil
}

func (o Options) fetch(ctx context.Context, rawURL, sha, dest,
	name string,
) (download.Result, error) {
	return download.Fetch(ctx, download.Options{
		URL:       rawURL,
		SHA256:    sha,
		Dest:      dest,
		Client:    o.Client,
		Attempts:  o.Attempts,
		RetryWait: o.RetryWait,
		Progress: func(update download.Progress) {
			o.report(Progress{
				Stage:      StageDownloading,
				Name:       name,
				Downloaded: update.Downloaded,
				Total:      update.Total,
			})
		},
	})
}

func (o Options) report(progress Progress) {
	if o.Progress != nil {
		o.Progress(progress)
	}
}

// mapEntry maps archive entries into a Data Files tree. The release archive
// stores its payload below Data Files; everything else (launcher files,
// documentation) is skipped.
func mapEntry(name string, directory bool) (string, bool, error) {
	match := name
	if directory {
		match = strings.TrimSuffix(match, "/")
	}
	lower := strings.ToLower(match)
	if lower == "data files" {
		if directory {
			return "", true, nil
		}
		return "", false, nil
	}
	if strings.HasPrefix(lower, "data files/") {
		return name[len("Data Files/"):], true, nil
	}
	return "", false, nil
}

func validateWorkDir(workDir string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return "", errors.New("client work directory is empty")
	}
	absolute, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("client work directory %q: %w", workDir, err)
	}
	return filepath.Clean(absolute), nil
}

func writeMarker(path string, meta marker) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode localization marker: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write localization marker: %w", err)
	}
	return nil
}

func countFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("localization contains a symlink: %s", path)
		}
		if entry.Type().IsRegular() {
			count++
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect extracted localization: %w", err)
	}
	return count, nil
}
