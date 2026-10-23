package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

const (
	backupTypeMods  = "mods"
	backupTypeState = "state"

	defaultBackupsLimit = 200
	maxBackupsLimit     = 500

	backupsCurrentFile = "current.txt"
)

type backupsData struct {
	Type       string        `json:"type"`
	Current    *string       `json:"current"`
	Items      []backupsItem `json:"items"`
	NextBefore *string       `json:"next_before"`
}

type backupsItem struct {
	Name    string  `json:"name"`
	Size    int64   `json:"size"`
	MTime   string  `json:"mtime"`
	Current bool    `json:"current"`
	SHA256  *string `json:"sha256,omitempty"`
}

type backupsListOptions struct {
	root       string
	typ        string
	limit      int
	before     string
	beforeTime time.Time
	sha256     bool
}

type backupEntry struct {
	name  string
	size  int64
	mtime time.Time
}

func runBackups(opts options, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"backups: subcommand is required; want list"))
	}

	switch args[0] {
	case "list":
		return runBackupsList(opts, args[1:], stdout, stderr)
	case "create":
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInternal,
				`command "backups create" is not implemented yet`))
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("backups: unknown subcommand %q", args[0])))
	}
}

func runBackupsList(opts options, args []string, stdout, stderr io.Writer) int {
	listOpts, parseErr := parseBackupsListArgs(opts.root, args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	data, listErr := listBackups(listOpts)
	if listErr != nil {
		return writeError(stdout, stderr, opts.json, listErr)
	}

	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeBackupsHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseBackupsListArgs(root string, args []string) (backupsListOptions, *ops.Error) {
	fs := flag.NewFlagSet("backups list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	typ := fs.String("type", "", "")
	limit := fs.Int("limit", defaultBackupsLimit, "")
	before := fs.String("before", "", "")
	withSHA256 := fs.Bool("sha256", false, "")

	if err := fs.Parse(args); err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: %v", err))
	}
	if fs.NArg() != 0 {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: unexpected argument %q", fs.Arg(0)))
	}

	if *typ != backupTypeMods && *typ != backupTypeState {
		if *typ == "" {
			return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
				"backups list: --type is required (mods or state)")
		}
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: unsupported type %q; want mods or state",
				*typ))
	}
	if *limit < 1 || *limit > maxBackupsLimit {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: --limit must be between 1 and %d",
				maxBackupsLimit))
	}

	beforeSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "before" {
			beforeSet = true
		}
	})

	options := backupsListOptions{
		root:   root,
		typ:    *typ,
		limit:  *limit,
		before: *before,
		sha256: *withSHA256,
	}
	if !beforeSet {
		return options, nil
	}
	if options.before == "" {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			"backups list: --before must not be empty")
	}

	if err := pathguard.Base(options.before); err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitUnsafePath,
			fmt.Sprintf("backups list: --before: %v", err))
	}
	if err := validateBackupsCursor(options.typ, options.before); err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: --before: %v", err))
	}

	parsed, err := archive.ParseArchiveName(options.before)
	if err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: --before: %v", err))
	}
	options.beforeTime = parsed.Time.UTC()
	return options, nil
}

func validateBackupsCursor(typ, name string) error {
	switch typ {
	case backupTypeMods:
		return archive.ValidateModsArchiveName(name)
	case backupTypeState:
		return archive.ValidateStateArchiveName(name)
	default:
		return fmt.Errorf("unsupported type %q", typ)
	}
}

func listBackups(options backupsListOptions) (backupsData, *ops.Error) {
	dir := filepath.Join(options.root, "backups", options.typ)

	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return backupsData{}, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("backups list: read %s: %v", dir, err))
	}

	archives := make([]backupEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return backupsData{}, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("backups list: stat %s: %v",
					filepath.Join(dir, entry.Name()), infoErr))
		}
		if !info.Mode().IsRegular() {
			continue
		}

		parsed, parseErr := archive.ParseArchiveName(entry.Name())
		if parseErr != nil || !backupsKindMatches(options.typ, parsed.Kind) {
			continue
		}
		archives = append(archives, backupEntry{
			name:  entry.Name(),
			size:  info.Size(),
			mtime: info.ModTime().UTC(),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		if !archives[i].mtime.Equal(archives[j].mtime) {
			return archives[i].mtime.After(archives[j].mtime)
		}
		return archives[i].name > archives[j].name
	})

	var current *string
	if options.typ == backupTypeMods {
		value, currentErr := archive.ReadCurrentFile(
			filepath.Join(dir, backupsCurrentFile))
		switch {
		case errors.Is(currentErr, fs.ErrNotExist):
			// A fresh installation has no current.txt yet.
		case currentErr != nil:
			return backupsData{}, ops.NewError(ops.ExitArchiveInvalid,
				fmt.Sprintf("backups list: %v", currentErr))
		default:
			name := value.Name
			current = &name
		}
	}

	archives = applyBackupsCursor(archives, options)
	if len(archives) > options.limit {
		archives = archives[:options.limit]
	}

	items := make([]backupsItem, 0, len(archives))
	for _, entry := range archives {
		item := backupsItem{
			Name:    entry.name,
			Size:    entry.size,
			MTime:   entry.mtime.Format(time.RFC3339),
			Current: current != nil && entry.name == *current,
		}
		items = append(items, item)
	}

	if options.sha256 {
		for i := range items {
			sum, hashErr := backupSHA256(filepath.Join(dir, items[i].Name))
			if hashErr != nil {
				return backupsData{}, ops.NewError(ops.ExitInternal,
					fmt.Sprintf("backups list: sha256 %s: %v",
						items[i].Name, hashErr))
			}
			items[i].SHA256 = &sum
		}
	}

	var nextBefore *string
	if len(items) == options.limit {
		name := items[len(items)-1].Name
		nextBefore = &name
	}

	return backupsData{
		Type:       options.typ,
		Current:    current,
		Items:      items,
		NextBefore: nextBefore,
	}, nil
}

func backupsKindMatches(typ string, kind archive.ArchiveKind) bool {
	switch typ {
	case backupTypeMods:
		return kind.IsMods()
	case backupTypeState:
		return kind.IsState()
	default:
		return false
	}
}

// applyBackupsCursor keeps only entries older than --before. When the cursor
// archive still exists, its position in the mtime/name order is authoritative.
// If retention removed it between pages, the UTC timestamp embedded in its name
// is used as a stable fallback.
func applyBackupsCursor(entries []backupEntry, options backupsListOptions) []backupEntry {
	if options.before == "" {
		return entries
	}
	for i, entry := range entries {
		if entry.name == options.before {
			return entries[i+1:]
		}
	}

	older := make([]backupEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.mtime.Before(options.beforeTime) ||
			(entry.mtime.Equal(options.beforeTime) &&
				entry.name < options.before) {
			older = append(older, entry)
		}
	}
	return older
}

func backupSHA256(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()

	sum := sha256.New()
	if _, err := io.CopyBuffer(sum, file, make([]byte, 32*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func writeBackupsHuman(w io.Writer, data backupsData) error {
	current := "-"
	if data.Current != nil {
		current = *data.Current
	}
	nextBefore := "-"
	if data.NextBefore != nil {
		nextBefore = *data.NextBefore
	}

	var out strings.Builder
	fmt.Fprintf(&out, "type: %s\n", data.Type)
	fmt.Fprintf(&out, "current: %s\n", current)
	for _, item := range data.Items {
		marker := "-"
		if item.Current {
			marker = "current"
		}
		fmt.Fprintf(&out, "%s\t%d\t%s\t%s",
			item.Name, item.Size, item.MTime, marker)
		if item.SHA256 != nil {
			fmt.Fprintf(&out, "\t%s", *item.SHA256)
		}
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "next_before: %s\n", nextBefore)
	_, err := io.WriteString(w, out.String())
	return err
}
