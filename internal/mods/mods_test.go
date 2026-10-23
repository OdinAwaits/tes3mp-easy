package mods

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

const (
	testModBody   = "synthetic mod data\n"
	testMorrowind = "synthetic Morrowind.esm\n"
	testTribunal  = "synthetic Tribunal.esm\n"
	testBloodmoon = "synthetic Bloodmoon.esm\n"
)

func testInstallation(t *testing.T) morrowind.Installation {
	t.Helper()
	root := t.TempDir()
	dataFiles := filepath.Join(root, "Data Files")
	if err := os.MkdirAll(dataFiles, 0o700); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{
		"Morrowind.esm": testMorrowind,
		"Tribunal.esm":  testTribunal,
		"Bloodmoon.esm": testBloodmoon,
	}
	for name, body := range bodies {
		if err := os.WriteFile(filepath.Join(dataFiles, name),
			[]byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	installation, err := morrowind.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect Morrowind: %v", err)
	}
	return installation
}

func testModsArchive(t *testing.T) (fixtures.ModsArchive, []byte,
	fixtures.ModsManifest,
) {
	t.Helper()
	pack := fixtures.DefaultMods()
	pack.Required = []fixtures.RequiredFile{
		{Name: "Morrowind.esm", CRCs: []string{fixtures.CRC32Hex(testMorrowind)}},
		{Name: "Tribunal.esm", CRCs: []string{fixtures.CRC32Hex(testTribunal)}},
		{Name: "Bloodmoon.esm", CRCs: []string{fixtures.CRC32Hex(testBloodmoon)}},
		{Name: "mod.esp", CRCs: []string{fixtures.CRC32Hex(testModBody)}},
	}
	body := pack.Bytes(t)
	return pack, body, pack.Manifest(body)
}

type modsServer struct {
	*httptest.Server
	body     []byte
	manifest fixtures.ModsManifest
	sha      *string
	size     *int64
	archives int
}

func startModsServer(t *testing.T, body []byte,
	manifest fixtures.ModsManifest,
) *modsServer {
	t.Helper()
	server := &modsServer{
		body:     body,
		manifest: manifest,
	}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request,
	) {
		current := server.manifest
		payload := server.body
		switch r.URL.Path {
		case serverapi.ModsPath:
			sha := current.SHA256
			size := int64(len(payload))
			if server.sha != nil {
				sha = *server.sha
			}
			if server.size != nil {
				size = *server.size
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w,
				`{"name":%q,"sha256":%q,"size":%d,"url":%q}`,
				current.Name, sha, size, serverapi.ModsArchivePath)
		case serverapi.ModsArchivePath:
			server.archives++
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func testClient(t *testing.T, server *modsServer) *serverapi.Client {
	t.Helper()
	client, err := serverapi.New(serverapi.Options{
		BaseURL:   server.URL,
		Attempts:  1,
		RetryWait: 0,
	})
	if err != nil {
		t.Fatalf("serverapi.New: %v", err)
	}
	return client
}

func testOptions(t *testing.T, installation morrowind.Installation,
	client *serverapi.Client,
) Options {
	t.Helper()
	root := t.TempDir()
	return Options{
		WorkDir:   filepath.Join(root, "work"),
		CacheDir:  filepath.Join(root, "cache"),
		ServerID:  "203.0.113.10_25565",
		Morrowind: installation,
		Client:    client,
	}
}

func TestInstallCreatesDataFilesAndProfile(t *testing.T) {
	installation := testInstallation(t)
	pack, body, manifest := testModsArchive(t)
	server := startModsServer(t, body, manifest)
	options := testOptions(t, installation, testClient(t, server))
	var stages []Stage
	options.Progress = func(update Progress) { stages = append(stages, update.Stage) }
	result, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Skipped || !result.Downloaded || !result.State.Installed {
		t.Fatalf("result = %+v", result)
	}
	if result.State.Plugins != 1 || result.State.SHA256 != manifest.SHA256 {
		t.Fatalf("state = %+v", result.State)
	}
	plugin, err := os.ReadFile(filepath.Join(result.State.DataFiles, "mod.esp"))
	if err != nil || string(plugin) != testModBody {
		t.Fatalf("installed plugin = %q, %v", plugin, err)
	}
	if _, err := os.Stat(filepath.Join(result.State.DataFiles, metadataName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("requiredDataFiles.json leaked into Data Files: %v", err)
	}
	profile, err := os.ReadFile(result.State.Profile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(profile)
	for _, want := range []string{
		"fallback-archive=Morrowind.bsa",
		`data="` + filepath.ToSlash(installation.DataFiles) + `"`,
		`data="` + filepath.ToSlash(result.State.DataFiles) + `"`,
		"content=Morrowind.esm",
		"content=Tribunal.esm",
		"content=Bloodmoon.esm",
		"content=mod.esp",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("openmw.cfg is missing %q:\n%s", want, text)
		}
	}
	digest, err := os.ReadFile(filepath.Join(result.State.ServerDir, digestFile))
	if err != nil || strings.TrimSpace(string(digest)) != manifest.SHA256 {
		t.Fatalf("digest = %q, %v", digest, err)
	}
	if _, err := os.Stat(filepath.Join(options.CacheDir, pack.ArchiveName())); err != nil {
		t.Fatalf("cache archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.State.ServerDir, stagingDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging remains: %v", err)
	}
	for _, want := range []Stage{StageFetching, StageDownloading, StageVerifying,
		StageExtracting, StageInstalling} {
		if !containsStage(stages, want) {
			t.Errorf("progress stages %v do not include %s", stages, want)
		}
	}
}

func containsStage(stages []Stage, want Stage) bool {
	for _, stage := range stages {
		if stage == want {
			return true
		}
	}
	return false
}

func TestInstallSkipsCurrentAndReplacesChangedPack(t *testing.T) {
	installation := testInstallation(t)
	_, body, manifest := testModsArchive(t)
	server := startModsServer(t, body, manifest)
	options := testOptions(t, installation, testClient(t, server))
	first, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	second, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !second.Skipped || second.Downloaded {
		t.Fatalf("second result = %+v", second)
	}
	if server.archives != 1 {
		t.Fatalf("archive downloads = %d, want 1", server.archives)
	}

	updated := fixtures.DefaultMods()
	updated.Name = "mods-2026-10-23_13-00-00.tar.gz"
	updated.Plugins = []fixtures.Plugin{
		{Name: "new.esp", Body: "new plugin\n"},
	}
	updated.Required = []fixtures.RequiredFile{
		{Name: "Morrowind.esm", CRCs: []string{fixtures.CRC32Hex(testMorrowind)}},
		{Name: "Tribunal.esm", CRCs: []string{fixtures.CRC32Hex(testTribunal)}},
		{Name: "Bloodmoon.esm", CRCs: []string{fixtures.CRC32Hex(testBloodmoon)}},
		{Name: "new.esp", CRCs: []string{fixtures.CRC32Hex("new plugin\n")}},
	}
	updatedBody := updated.Bytes(t)
	updatedManifest := updated.Manifest(updatedBody)
	server.body = updatedBody
	server.manifest = updatedManifest

	third, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("third Install: %v", err)
	}
	if third.Skipped || !third.Downloaded {
		t.Fatalf("third result = %+v", third)
	}
	if _, err := os.Stat(filepath.Join(first.State.DataFiles, "mod.esp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old plugin remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(third.State.DataFiles, "new.esp")); err != nil {
		t.Fatalf("new plugin: %v", err)
	}
}

func TestInstallRejectsChecksumMismatchAndKeepsOldData(t *testing.T) {
	installation := testInstallation(t)
	_, body, manifest := testModsArchive(t)
	server := startModsServer(t, body, manifest)
	options := testOptions(t, installation, testClient(t, server))
	first, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}

	replacement := fixtures.DefaultMods()
	replacement.Name = "mods-2026-10-23_14-00-00.tar.gz"
	replacementBody := replacement.Bytes(t)
	replacementManifest := replacement.Manifest(replacementBody)
	server.body = replacementBody
	server.manifest = replacementManifest
	wrong := strings.Repeat("b", 64)
	server.sha = &wrong

	if _, err := Install(context.Background(), options); err == nil ||
		!strings.Contains(err.Error(), "sha256") {
		t.Fatalf("Install error = %v, want sha256 mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(first.State.DataFiles, "mod.esp")); err != nil {
		t.Fatalf("old plugin was removed: %v", err)
	}
	digest, err := os.ReadFile(filepath.Join(first.State.ServerDir, digestFile))
	if err != nil || strings.TrimSpace(string(digest)) != manifest.SHA256 {
		t.Fatalf("digest changed after failure: %q, %v", digest, err)
	}
	if _, err := os.Stat(filepath.Join(options.CacheDir, replacement.ArchiveName())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt cache entry remains: %v", err)
	}
}

func TestInstallRejectsCorruptArchive(t *testing.T) {
	installation := testInstallation(t)
	body := []byte("not a tar.gz archive")
	manifest := fixtures.ModsManifest{
		Name:   "mods-2026-10-23_15-00-00.tar.gz",
		SHA256: fixtures.SHA256Hex(body),
	}
	server := startModsServer(t, body, manifest)
	options := testOptions(t, installation, testClient(t, server))
	if _, err := Install(context.Background(), options); err == nil {
		t.Fatal("corrupt archive accepted")
	}
	state, err := Inspect(options.WorkDir, options.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Installed {
		t.Fatalf("corrupt archive was installed: %+v", state)
	}
}

func TestInstallRejectsBaseCRCMismatch(t *testing.T) {
	installation := testInstallation(t)
	pack := fixtures.DefaultMods()
	pack.Name = "mods-2026-10-23_16-00-00.tar.gz"
	pack.Required = []fixtures.RequiredFile{
		{Name: "Morrowind.esm", CRCs: []string{"0x00000000"}},
		{Name: "Tribunal.esm", CRCs: []string{fixtures.CRC32Hex(testTribunal)}},
		{Name: "Bloodmoon.esm", CRCs: []string{fixtures.CRC32Hex(testBloodmoon)}},
		{Name: "mod.esp", CRCs: []string{fixtures.CRC32Hex(testModBody)}},
	}
	body := pack.Bytes(t)
	manifest := pack.Manifest(body)
	server := startModsServer(t, body, manifest)
	options := testOptions(t, installation, testClient(t, server))
	if _, err := Install(context.Background(), options); err == nil ||
		!strings.Contains(err.Error(), "CRC mismatch") {
		t.Fatalf("Install error = %v, want CRC mismatch", err)
	}
	state, err := Inspect(options.WorkDir, options.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Installed {
		t.Fatalf("CRC-failed archive was installed: %+v", state)
	}
}

func TestInstallValidatesOptions(t *testing.T) {
	installation := testInstallation(t)
	client := testClient(t, startModsServer(t, nil,
		fixtures.ModsManifest{Name: "mods-2026-10-23_12-00-00.tar.gz"}))
	options := testOptions(t, installation, client)
	if _, err := Install(context.Background(), Options{}); err == nil {
		t.Fatal("empty options accepted")
	}
	bad := options
	bad.ServerID = "../escape"
	if _, err := Install(context.Background(), bad); err == nil {
		t.Fatal("path traversal server ID accepted")
	}
	bad = options
	bad.WorkDir = "relative"
	if _, err := Install(context.Background(), bad); err == nil {
		t.Fatal("relative work directory accepted")
	}
	bad = options
	bad.Morrowind.Files = nil
	if _, err := Install(context.Background(), bad); err == nil {
		t.Fatal("incomplete Morrowind installation accepted")
	}
}

func TestBuildProfilePreservesUserLines(t *testing.T) {
	existing := []byte("encoding=win1251\n" +
		"fallback=Fonts_Font_0,Fonts_Font_1\n" +
		"fallback-archive=Morrowind.bsa\n" +
		"data=\"/old\"\n" +
		"content=Old.esp\n")
	required := []archive.RequiredFile{
		{Name: "Morrowind.esm"},
		{Name: "Tribunal.esm"},
		{Name: "Bloodmoon.esm"},
		{Name: "mod.esp"},
	}
	profile := string(BuildProfile(existing, "/games/Morrowind/Data Files",
		"/work/servers/host_25565/data/Data Files", required))
	for _, want := range []string{
		"encoding=win1251",
		"fallback=Fonts_Font_0,Fonts_Font_1",
		"fallback-archive=Morrowind.bsa",
		`data="/games/Morrowind/Data Files"`,
		`data="/work/servers/host_25565/data/Data Files"`,
		"content=mod.esp",
	} {
		if !strings.Contains(profile, want) {
			t.Errorf("profile is missing %q:\n%s", want, profile)
		}
	}
	if strings.Contains(profile, "content=Old.esp") ||
		strings.Contains(profile, `data="/old"`) {
		t.Fatalf("old data/content lines survived:\n%s", profile)
	}
	if strings.Count(profile, "fallback-archive=Morrowind.bsa") != 1 {
		t.Fatalf("fallback archive was duplicated:\n%s", profile)
	}
}

func TestRecoverInstallationRestoresBackups(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDF := filepath.Join(dataDir, dataFilesSub)
	targetProfile := filepath.Join(root, profileFile)
	if err := os.MkdirAll(targetDF+".old", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDF+".old", "old.esp"),
		[]byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetProfile+".old", []byte("old profile\n"),
		0o600); err != nil {
		t.Fatal(err)
	}
	if err := recoverInstallation(targetDF, targetProfile); err != nil {
		t.Fatalf("recoverInstallation: %v", err)
	}
	if !regularDir(targetDF) || !regularFile(targetProfile) {
		t.Fatal("backups were not restored")
	}
}

func TestRecoverInstallationRejectsBackupSymlink(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDF := filepath.Join(dataDir, dataFilesSub)
	backup := targetDF + ".old"
	if err := os.Symlink(t.TempDir(), backup); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if err := recoverInstallation(targetDF, filepath.Join(root, profileFile)); err == nil {
		t.Fatal("symlink backup accepted")
	}
}

func TestSwapInstallationRollsBackOnDigestFailure(t *testing.T) {
	root := t.TempDir()
	targetDF := filepath.Join(root, "data", dataFilesSub)
	targetProfile := filepath.Join(root, profileFile)
	if err := os.MkdirAll(targetDF, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDF, "old.esp"), []byte("old"),
		0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetProfile, []byte("old profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	newDF := filepath.Join(root, "staging", dataFilesSub)
	newProfile := filepath.Join(root, "staging", profileFile)
	if err := os.MkdirAll(newDF, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDF, "new.esp"), []byte("new"),
		0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newProfile, []byte("new profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A directory target makes the final atomic rename fail after both new
	// trees were activated, exercising the rollback path.
	digestDir := filepath.Join(root, "digest-dir")
	if err := os.MkdirAll(digestDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := swapInstallation(newDF, newProfile, targetDF, targetProfile,
		digestDir, strings.Repeat("c", 64)); err == nil {
		t.Fatal("digest failure was ignored")
	}
	old, err := os.ReadFile(filepath.Join(targetDF, "old.esp"))
	if err != nil || string(old) != "old" {
		t.Fatalf("old Data Files not restored: %q, %v", old, err)
	}
	profile, err := os.ReadFile(targetProfile)
	if err != nil || string(profile) != "old profile\n" {
		t.Fatalf("old profile not restored: %q, %v", profile, err)
	}
}
