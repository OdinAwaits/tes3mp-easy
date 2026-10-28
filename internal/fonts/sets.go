package fonts

import "strings"

// Set is one selectable font pack. Every set has exactly three pinned files in
// the same order: the interface font, the Daedric slot and the console font.
// All sets cover Latin and Cyrillic, so switching a set never brings back the
// boxes instead of Cyrillic.
type Set struct {
	ID string
	// Name is the human name of the set. Family names are not translated, so
	// the label lives next to the pinned files instead of in the i18n catalog.
	Name   string
	Assets []Asset
}

// Set identifiers stored in config.json as font_set.
const (
	SetNoto       = "noto"
	SetInter      = "inter"
	SetPlex       = "plex"
	SetRoboto     = "roboto"
	SetOpenSans   = "opensans"
	SetMontserrat = "montserrat"
	SetFira       = "fira"
	SetArimo      = "arimo"
	SetSourceSans = "sourcesans"
	SetGolos      = "golos"

	// DefaultSet is used for an empty or unknown font_set. It is the set the
	// client shipped before the catalog existed, so legacy configs keep their
	// fonts without a migration step.
	DefaultSet = SetNoto
)

// fontFiles pins every font the catalog may install. Files that several sets
// share are declared once, so a URL or a digest never drifts between sets.
var (
	notoSans = Asset{
		Name:   "NotoSans.ttf",
		URL:    fontRoot + "/notosans/NotoSans%5Bwdth,wght%5D.ttf",
		SHA256: "bfb7bb691513f12e734dc346c03a03f784912432d7e3fa8e56efcf906fe86b3d",
	}
	notoSerif = Asset{
		Name:   "NotoSerif.ttf",
		URL:    fontRoot + "/notoserif/NotoSerif%5Bwdth,wght%5D.ttf",
		SHA256: "4d8e6761424656867019081a1a01336f3cb086982682698714054fc33f782713",
	}
	robotoMono = Asset{
		Name:   "RobotoMono.ttf",
		URL:    fontRoot + "/robotomono/RobotoMono%5Bwght%5D.ttf",
		SHA256: "66a80e79d17e4c7cabd162e2916578a4cc08fd19eef6e2a643305eae9c567b2b",
	}
	inter = Asset{
		Name:   "Inter.ttf",
		URL:    fontRoot + "/inter/Inter%5Bopsz,wght%5D.ttf",
		SHA256: "29160a80ff49ddcab2c97711247e08b1fab27a484a329ce8b813d820dc559031",
	}
	sourceSerif4 = Asset{
		Name:   "SourceSerif4.ttf",
		URL:    fontRoot + "/sourceserif4/SourceSerif4%5Bopsz,wght%5D.ttf",
		SHA256: "97b2d4da6e3cb494b5a1e66ae176914d852ccabef49e0c02c0df25f3e39aca0b",
	}
	jetBrainsMono = Asset{
		Name:   "JetBrainsMono.ttf",
		URL:    fontRoot + "/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf",
		SHA256: "48715a42ec242c21e9f02692891e147d022299a52e48d5e413e1a942193ffeda",
	}
	plexSans = Asset{
		Name:   "IBMPlexSans.ttf",
		URL:    fontRoot + "/ibmplexsans/IBMPlexSans%5Bwdth,wght%5D.ttf",
		SHA256: "3b031aa4216174205bd8471f88a49b91f093169e9e87bd5262242bc5967fe2e3",
	}
	plexSerif = Asset{
		Name:   "IBMPlexSerif.ttf",
		URL:    fontRoot + "/ibmplexserif/IBMPlexSerif-Regular.ttf",
		SHA256: "e882efa9c41949a528ac2369079ec5ef050c1c996bbd0bacce3c3326d44cf80d",
	}
	plexMono = Asset{
		Name:   "IBMPlexMono.ttf",
		URL:    fontRoot + "/ibmplexmono/IBMPlexMono-Regular.ttf",
		SHA256: "6a3412f058c7d8dfd9170c41e85ade48e5156ecb89356110ca57a0a27734af46",
	}
	roboto = Asset{
		Name:   "Roboto.ttf",
		URL:    fontRoot + "/roboto/Roboto%5Bwdth,wght%5D.ttf",
		SHA256: "d7598e12c5dbef095ff8272cfc55da0250bd07fbdecbac8a530b9b277872a134",
	}
	lora = Asset{
		Name:   "Lora.ttf",
		URL:    fontRoot + "/lora/Lora%5Bwght%5D.ttf",
		SHA256: "822a6621ccbe8d97d20ac88c1c41f5615c9c2c202eaa75f272cd452aac6475a7",
	}
	cousine = Asset{
		Name:   "Cousine.ttf",
		URL:    fontRoot + "/cousine/Cousine-Regular.ttf",
		SHA256: "1da22250675fc4c42fcf3a9736c44bc0570516105331443b663fd5cfbd1412fe",
	}
	openSans = Asset{
		Name:   "OpenSans.ttf",
		URL:    fontRoot + "/opensans/OpenSans%5Bwdth,wght%5D.ttf",
		SHA256: "36643644f318a812aab2d2ed3bb98f8cf0872527f835fe9398d95fe6b9adb878",
	}
	sourceCodePro = Asset{
		Name:   "SourceCodePro.ttf",
		URL:    fontRoot + "/sourcecodepro/SourceCodePro%5Bwght%5D.ttf",
		SHA256: "b400fc584e10aff25d0e775ce181b4fc1c5ea1b5dc37b81aeb2084375b945790",
	}
	montserrat = Asset{
		Name:   "Montserrat.ttf",
		URL:    fontRoot + "/montserrat/Montserrat%5Bwght%5D.ttf",
		SHA256: "0f7b311b2f3279e4eef9b2f968bcdbab6e28f4daeb1f049f4f278a902bcd82f7",
	}
	playfair = Asset{
		Name:   "PlayfairDisplay.ttf",
		URL:    fontRoot + "/playfairdisplay/PlayfairDisplay%5Bwght%5D.ttf",
		SHA256: "c40f2293766a503bc70cce9e512ef844a4ccb7cbcde792fe2ea31d191917d8d6",
	}
	firaSans = Asset{
		Name:   "FiraSans.ttf",
		URL:    fontRoot + "/firasans/FiraSans-Regular.ttf",
		SHA256: "c29556a2719bf613ef3d5e070e40d903a8965d9c081beca1375dc1e6e0f93c23",
	}
	firaMono = Asset{
		Name:   "FiraMono.ttf",
		URL:    fontRoot + "/firamono/FiraMono-Regular.ttf",
		SHA256: "2e00b0cf3106a3d792f35711b7722740b76c02ee7300ea8c60f940584b7a8ac9",
	}
	arimo = Asset{
		Name:   "Arimo.ttf",
		URL:    fontRoot + "/arimo/Arimo%5Bwght%5D.ttf",
		SHA256: "e43898b143ec826ac8cb4034816458a7047fbe0836558de2a1f8c6223ae3e0ca",
	}
	sourceSans3 = Asset{
		Name:   "SourceSans3.ttf",
		URL:    fontRoot + "/sourcesans3/SourceSans3%5Bwght%5D.ttf",
		SHA256: "042fe2cc0b933e328410d7acbd0aa6a1873dca5aef81875f4bc214b08825c7b9",
	}
	golosText = Asset{
		Name:   "GolosText.ttf",
		URL:    fontRoot + "/golostext/GolosText%5Bwght%5D.ttf",
		SHA256: "17bb58fb69aec2dfb047a2ebf52534023e9b688c97a6b7ac795b0a72912c2063",
	}
)

// sets is the catalog in display order. The first entry is the default set and
// reuses Assets, so the legacy getters keep working unchanged.
var sets = []Set{
	{ID: SetNoto, Name: "Noto Sans · Noto Serif · Roboto Mono",
		Assets: Assets},
	{ID: SetInter, Name: "Inter · Source Serif 4 · JetBrains Mono",
		Assets: []Asset{inter, sourceSerif4, jetBrainsMono}},
	{ID: SetPlex, Name: "IBM Plex Sans · IBM Plex Serif · IBM Plex Mono",
		Assets: []Asset{plexSans, plexSerif, plexMono}},
	{ID: SetRoboto, Name: "Roboto · Lora · Cousine",
		Assets: []Asset{roboto, lora, cousine}},
	{ID: SetOpenSans, Name: "Open Sans · Noto Serif · Source Code Pro",
		Assets: []Asset{openSans, notoSerif, sourceCodePro}},
	{ID: SetMontserrat, Name: "Montserrat · Playfair Display · JetBrains Mono",
		Assets: []Asset{montserrat, playfair, jetBrainsMono}},
	{ID: SetFira, Name: "Fira Sans · Lora · Fira Mono",
		Assets: []Asset{firaSans, lora, firaMono}},
	{ID: SetArimo, Name: "Arimo · Noto Serif · Source Code Pro",
		Assets: []Asset{arimo, notoSerif, sourceCodePro}},
	{ID: SetSourceSans, Name: "Source Sans 3 · Playfair Display · Cousine",
		Assets: []Asset{sourceSans3, playfair, cousine}},
	{ID: SetGolos, Name: "Golos Text · Source Serif 4 · JetBrains Mono",
		Assets: []Asset{golosText, sourceSerif4, jetBrainsMono}},
}

// Sets returns the catalog. The slices are copies, so a caller cannot change
// the pinned assets of a set.
func Sets() []Set {
	catalog := make([]Set, 0, len(sets))
	for _, set := range sets {
		catalog = append(catalog, Set{
			ID:     set.ID,
			Name:   set.Name,
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
				Name:   set.Name,
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
