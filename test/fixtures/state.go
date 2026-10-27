package fixtures

import (
	"archive/tar"
	"testing"
)

const (
	// StateMetadataName is requiredDataFiles.json at the state archive root.
	StateMetadataName = MetadataName
	// StateCurrentName is current.txt at the state archive root.
	StateCurrentName = CurrentName
)

// StateFile is one synthetic player or world file.
type StateFile struct {
	Name string
	Body string
}

// StateArchive describes a synthetic state tar.gz archive.
type StateArchive struct {
	// Name is the archive filename. The default is StateName(DefaultTime).
	Name string
	// Mods supplies requiredDataFiles.json and current.txt.
	Mods ModsManifest
	// Players are written to players/.
	Players []StateFile
	// World maps a required world subdirectory to files below it.
	World map[string][]StateFile
	// Configs are written below configs/ when non-empty.
	Configs []StateFile
	// OmitPlayersDir omits players/, producing an invalid archive.
	OmitPlayersDir bool
	// OmitWorldDir omits world/, producing an invalid archive.
	OmitWorldDir bool
	// OmitWorldDirs omits selected world/<name>/ directories.
	OmitWorldDirs []string
	// OmitRequired omits requiredDataFiles.json.
	OmitRequired bool
	// OmitCurrent omits current.txt.
	OmitCurrent bool
	// FlatWorld writes cell/... instead of world/cell/... (invalid shape).
	FlatWorld bool
	// Extra entries are appended verbatim for failure-mode fixtures.
	Extra []Entry
}

var fixtureWorldDirs = []string{"cell", "world", "map", "recordstore", "custom"}

// DefaultState returns the empty but valid state archive for the given mods
// manifest: players/ and all five world/<name>/ directories, both metadata
// files, and no state files.
func DefaultState(mods ModsManifest) StateArchive {
	return StateArchive{Mods: mods}
}

// FullState returns a state archive with one synthetic player and one file in
// every world subdirectory.
func FullState(mods ModsManifest) StateArchive {
	state := DefaultState(mods)
	state.Players = []StateFile{{Name: "player.json", Body: "{}\n"}}
	state.World = make(map[string][]StateFile, len(fixtureWorldDirs))
	for _, name := range fixtureWorldDirs {
		state.World[name] = []StateFile{{Name: name + ".json", Body: "{}\n"}}
	}
	return state
}

// ArchiveName returns the effective archive filename.
func (s StateArchive) ArchiveName() string {
	if s.Name != "" {
		return s.Name
	}
	return StateName(DefaultTime)
}

// Entries returns state entries in deterministic order.
func (s StateArchive) Entries() []Entry {
	var entries []Entry
	if !s.OmitCurrent {
		entries = append(entries, Entry{
			Name: StateCurrentName,
			Body: CurrentLine(s.Mods.SHA256, s.Mods.Name),
		})
	}
	if len(s.Configs) > 0 {
		entries = append(entries, Entry{Name: "configs/", Type: tar.TypeDir})
		for _, file := range s.Configs {
			entries = append(entries, Entry{
				Name: "configs/" + file.Name,
				Body: file.Body,
			})
		}
	}
	if !s.OmitPlayersDir {
		entries = append(entries, Entry{Name: "players/", Type: tar.TypeDir})
	}
	if !s.OmitWorldDir && !s.FlatWorld {
		entries = append(entries, Entry{Name: "world/", Type: tar.TypeDir})
	}

	omitted := make(map[string]bool, len(s.OmitWorldDirs))
	for _, name := range s.OmitWorldDirs {
		omitted[name] = true
	}
	for _, name := range fixtureWorldDirs {
		if omitted[name] {
			continue
		}
		prefix := "world/"
		if s.FlatWorld {
			prefix = ""
		}
		entries = append(entries, Entry{
			Name: prefix + name + "/",
			Type: tar.TypeDir,
		})
	}

	if !s.OmitRequired {
		entries = append(entries, Entry{
			Name: StateMetadataName,
			Body: string(requiredJSONFrom(s.Mods.Required)),
		})
	}

	for _, player := range s.Players {
		entries = append(entries, Entry{
			Name: "players/" + player.Name,
			Body: player.Body,
		})
	}
	for _, name := range fixtureWorldDirs {
		if omitted[name] {
			continue
		}
		prefix := "world/" + name + "/"
		if s.FlatWorld {
			prefix = name + "/"
		}
		for _, file := range s.World[name] {
			entries = append(entries, Entry{
				Name: prefix + file.Name,
				Body: file.Body,
			})
		}
	}
	entries = append(entries, s.Extra...)
	return entries
}

// Bytes returns the gzip-compressed archive.
func (s StateArchive) Bytes(t testing.TB) []byte {
	t.Helper()
	return TarGz(t, s.Entries())
}

// Write writes the archive to path.
func (s StateArchive) Write(t testing.TB, path string) {
	t.Helper()
	WriteTarGz(t, path, s.Entries())
}
