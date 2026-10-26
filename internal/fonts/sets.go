package fonts

import "strings"

// Set is one selectable font pack. Every set has exactly three pinned files in
// the same order: the interface font, the Daedric slot and the console font.
// All sets cover Latin and Cyrillic, so switching a set never brings back the
// boxes instead of Cyrillic.
type Set struct {
	ID     string
	Assets []Asset
}

// Set identifiers stored in config.json as font_set.
const (
	SetNoto  = "noto"
	SetInter = "inter"
	SetPlex  = "plex"

	// DefaultSet is used for an empty or unknown font_set. It is the set the
	// client shipped before the catalog existed, so legacy configs keep their
	// fonts without a migration step.
	DefaultSet = SetNoto
)

// interAssets mirrors the Noto/OFL families with a more geometric sans, a
// bookish serif and a coding mono.
var interAssets = []Asset{
	{
		Name:   "Inter.ttf",
		URL:    fontRoot + "/inter/Inter%5Bopsz,wght%5D.ttf",
		SHA256: "29160a80ff49ddcab2c97711247e08b1fab27a484a329ce8b813d820dc559031",
	},
	{
		Name:   "SourceSerif4.ttf",
		URL:    fontRoot + "/sourceserif4/SourceSerif4%5Bopsz,wght%5D.ttf",
		SHA256: "97b2d4da6e3cb494b5a1e66ae176914d852ccabef49e0c02c0df25f3e39aca0b",
	},
	{
		Name:   "JetBrainsMono.ttf",
		URL:    fontRoot + "/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf",
		SHA256: "48715a42ec242c21e9f02692891e147d022299a52e48d5e413e1a942193ffeda",
	},
}

// plexAssets is the IBM Plex family: the most "technical" of the three sets.
var plexAssets = []Asset{
	{
		Name:   "IBMPlexSans.ttf",
		URL:    fontRoot + "/ibmplexsans/IBMPlexSans%5Bwdth,wght%5D.ttf",
		SHA256: "3b031aa4216174205bd8471f88a49b91f093169e9e87bd5262242bc5967fe2e3",
	},
	{
		Name:   "IBMPlexSerif.ttf",
		URL:    fontRoot + "/ibmplexserif/IBMPlexSerif-Regular.ttf",
		SHA256: "e882efa9c41949a528ac2369079ec5ef050c1c996bbd0bacce3c3326d44cf80d",
	},
	{
		Name:   "IBMPlexMono.ttf",
		URL:    fontRoot + "/ibmplexmono/IBMPlexMono-Regular.ttf",
		SHA256: "6a3412f058c7d8dfd9170c41e85ade48e5156ecb89356110ca57a0a27734af46",
	},
}

// sets is the catalog in display order. The first entry is the default set and
// reuses Assets, so the legacy getters keep working unchanged.
var sets = []Set{
	{ID: SetNoto, Assets: Assets},
	{ID: SetInter, Assets: interAssets},
	{ID: SetPlex, Assets: plexAssets},
}

// Sets returns the catalog. The slices are copies, so a caller cannot change
// the pinned assets of a set.
func Sets() []Set {
	catalog := make([]Set, 0, len(sets))
	for _, set := range sets {
		catalog = append(catalog, Set{
			ID:     set.ID,
			Assets: append([]Asset(nil), set.Assets...),
		})
	}
	return catalog
}

// SetIDs returns the catalog identifiers in display order.
func SetIDs() []string {
	ids := make([]string, 0, len(sets))
	for _, set := range sets {
		ids = append(ids, set.ID)
	}
	return ids
}

// LookupSet resolves a font_set value. Matching is case-insensitive because the
// value round-trips through a hand-editable JSON file.
func LookupSet(id string) (Set, bool) {
	wanted := strings.ToLower(strings.TrimSpace(id))
	for _, set := range sets {
		if set.ID == wanted {
			return Set{
				ID:     set.ID,
				Assets: append([]Asset(nil), set.Assets...),
			}, true
		}
	}
	return Set{}, false
}

// KnownSet reports whether id names a catalog set. The empty string is known:
// it means the default set.
func KnownSet(id string) bool {
	if strings.TrimSpace(id) == "" {
		return true
	}
	_, ok := LookupSet(id)
	return ok
}

// ResolveSet returns the set for id and falls back to DefaultSet for an empty or
// unknown value, so a hand-edited config never breaks the launcher.
func ResolveSet(id string) Set {
	if set, ok := LookupSet(id); ok {
		return set
	}
	set, _ := LookupSet(DefaultSet)
	return set
}

// ManagedAssetNames lists every font file the launcher may have written for any
// set. It bounds the cleanup after a set switch: only files the launcher itself
// installs can be removed.
func ManagedAssetNames() []string {
	seen := make(map[string]struct{}, len(sets)*3)
	names := make([]string, 0, len(sets)*3)
	for _, set := range sets {
		for _, asset := range set.Assets {
			if _, ok := seen[asset.Name]; ok {
				continue
			}
			seen[asset.Name] = struct{}{}
			names = append(names, asset.Name)
		}
	}
	return names
}
