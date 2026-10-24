package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ownerTes3MP      = "tes3mp"
	ownerTes3MPPanel = "tes3mp-panel"
	ownerRoot        = "root"
)

type ownerNamesFunc func(uid, gid uint32) (string, string, error)

type ownershipDir struct {
	rel   string
	user  string
	group string
	mode  uint32
}

// ownershipDirs is the exact directory layout and mode table from
// docs/v2/02-architecture.md §4 and §5.
var ownershipDirs = []ownershipDir{
	{rel: "", user: ownerRoot, group: ownerRoot, mode: 0o755},
	{rel: "configs", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: "players", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "world", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "mods", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "backups", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "panel-data", user: ownerTes3MPPanel, group: ownerTes3MP, mode: 0o750},
	{rel: "panel-data/tls", user: ownerTes3MPPanel, group: ownerTes3MP, mode: 0o700},
	{rel: "panel-data/acme", user: ownerTes3MPPanel, group: ownerTes3MP, mode: 0o700},
	{rel: "panel-data/uploads", user: ownerTes3MPPanel, group: ownerTes3MP, mode: 0o2770},
	{rel: ".locks", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: ".staging", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: ".config", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: ".config/openmw", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: ".local", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: ".local/share", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: ".local/share/openmw", user: ownerTes3MP, group: ownerTes3MP, mode: 0o700},
	{rel: "world/cell", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "world/world", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "world/map", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "world/recordstore", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "world/custom", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "mods/plugins", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "mods/scripts", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "backups/mods", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
	{rel: "backups/state", user: ownerTes3MP, group: ownerTes3MP, mode: 0o750},
}

type ownershipFile struct {
	rel   string
	user  string
	group string
	mode  uint32
}

var ownershipFiles = []ownershipFile{
	{rel: "backups/mods/current.txt", user: ownerTes3MP, group: ownerTes3MP, mode: 0o640},
	// The panel creates these with its primary group. Mode 0600 means the
	// group has no access; keep the check exact for the panel's own group.
	{rel: "panel-data/auth.json", user: ownerTes3MPPanel, group: ownerTes3MPPanel, mode: 0o600},
	{rel: "panel-data/initial-password.txt", user: ownerTes3MPPanel, group: ownerTes3MPPanel, mode: 0o600},
}

type ownerLookup func(label, wantUser, wantGroup string, info os.FileInfo) string

func checkOwnership(root string, names ownerNamesFunc, skipOwner bool) Check {
	if root == "" {
		return errorCheck("ownership", "root path is empty")
	}
	if names == nil {
		names = defaultOwnerNames
	}

	problems := make([]string, 0, len(ownershipDirs))
	lookup := newOwnerLookup(names, skipOwner)

	for _, spec := range ownershipDirs {
		path := root
		label := "/"
		if spec.rel != "" {
			path = filepath.Join(root, spec.rel)
			label = spec.rel + "/"
		}

		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			problems = append(problems, fmt.Sprintf("%s is missing", label))
			continue
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", label, err))
			continue
		case info.Mode()&os.ModeSymlink != 0:
			problems = append(problems, fmt.Sprintf("%s is a symlink", label))
			continue
		case !info.IsDir():
			problems = append(problems, fmt.Sprintf("%s is not a directory", label))
			continue
		}
		if problem := inspectOwnership(label, info, spec.mode,
			spec.user, spec.group, lookup); problem != "" {
			problems = append(problems, problem)
		}
	}

	for _, spec := range ownershipFiles {
		path := filepath.Join(root, spec.rel)
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", spec.rel, err))
			continue
		case info.Mode()&os.ModeSymlink != 0:
			problems = append(problems, fmt.Sprintf("%s is a symlink", spec.rel))
			continue
		case !info.Mode().IsRegular():
			problems = append(problems, fmt.Sprintf("%s is not a regular file", spec.rel))
			continue
		}
		if problem := inspectOwnership(spec.rel, info, spec.mode,
			spec.user, spec.group, lookup); problem != "" {
			problems = append(problems, problem)
		}
	}

	problems = append(problems, checkOwnedChildren(
		root, "configs", ownerTes3MP, ownerTes3MP, 0o600, lookup)...)
	// panel-data/tls is 0700 and owned by tes3mp-panel, so ops cannot list its
	// files without violating the privilege boundary. The directory mode and
	// owner are still checked above; uploads is group-accessible by design.
	problems = append(problems, checkOwnedChildren(
		root, "panel-data/uploads", ownerTes3MPPanel, ownerTes3MP, 0o640, lookup)...)

	if len(problems) > 0 {
		return errorCheck("ownership", "%s", strings.Join(problems, "; "))
	}
	return Check{Name: "ownership", Status: StatusOK}
}

// checkOwnedChildren verifies the direct files of a directory. The known
// layout keeps configs and uploads flat.
func checkOwnedChildren(root, rel, user, group string, mode uint32,
	lookup ownerLookup,
) []string {
	dir := filepath.Join(root, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("%s/: %v", rel, err)}
	}

	problems := make([]string, 0)
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		label := filepath.ToSlash(filepath.Join(rel, entry.Name()))
		info, err := os.Lstat(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", label, err))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			problems = append(problems, fmt.Sprintf("%s is a symlink", label))
			continue
		}
		if !info.Mode().IsRegular() {
			problems = append(problems, fmt.Sprintf("%s is not a regular file", label))
			continue
		}
		if problem := inspectOwnership(label, info, mode,
			user, group, lookup); problem != "" {
			problems = append(problems, problem)
		}
	}
	return problems
}

func inspectOwnership(label string, info os.FileInfo, mode uint32,
	user, group string, lookup ownerLookup,
) string {
	if problem := inspectMode(label, info, mode); problem != "" {
		return problem
	}
	return lookup(label, user, group, info)
}

func inspectMode(label string, info os.FileInfo, want uint32) string {
	meta, ok := metadataOf(info)
	if !ok {
		return fmt.Sprintf("%s: cannot inspect ownership", label)
	}
	if meta.Mode != want {
		return fmt.Sprintf("%s mode is %04o, want %04o", label, meta.Mode, want)
	}
	return ""
}

func newOwnerLookup(names ownerNamesFunc, skip bool) ownerLookup {
	accounts := make(map[uint32]ownerIdentity)
	groups := make(map[uint32]ownerIdentity)

	return func(label, wantUser, wantGroup string, info os.FileInfo) string {
		if skip {
			return ""
		}
		meta, ok := metadataOf(info)
		if !ok {
			return fmt.Sprintf("%s: cannot inspect owner", label)
		}

		account, loaded := accounts[meta.UID]
		if !loaded {
			userName, groupName, err := names(meta.UID, meta.GID)
			account = ownerIdentity{user: userName, group: groupName, err: err}
			accounts[meta.UID] = account
			if _, ok := groups[meta.GID]; !ok {
				groups[meta.GID] = ownerIdentity{group: groupName, err: err}
			}
		}
		if account.err != nil {
			return fmt.Sprintf("%s owner: %v", label, account.err)
		}

		group, loaded := groups[meta.GID]
		if !loaded {
			_, groupName, err := names(meta.UID, meta.GID)
			group = ownerIdentity{group: groupName, err: err}
			groups[meta.GID] = group
		}
		if group.err != nil {
			return fmt.Sprintf("%s group: %v", label, group.err)
		}

		if account.user != wantUser || group.group != wantGroup {
			return fmt.Sprintf("%s is %s:%s, want %s:%s",
				label, account.user, group.group, wantUser, wantGroup)
		}
		return ""
	}
}

type ownerIdentity struct {
	user  string
	group string
	err   error
}

func defaultOwnerNames(uid, gid uint32) (string, string, error) {
	account, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return "", "", err
	}
	group, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return "", "", err
	}
	return account.Username, group.Name, nil
}
