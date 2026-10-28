package docs

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestStableReleaseLinks pins the moving-tag contract from BL-006: the publish
// job of the release workflow moves <channel>-latest, uploads archives without a
// version in the name and keeps the `Latest` badge on the versioned release,
// while README points at those stable URLs instead of /releases/latest.
func TestStableReleaseLinks(t *testing.T) {
	root := repoRoot(t)

	workflow := readDoc(t, filepath.Join(root, ".github", "workflows", "release.yml"))
	for _, want := range []string{
		"tag_name: ${{ needs.metadata.outputs.channel }}-latest",
		"make_latest: false",
		"overwrite_files: true",
		`stable="${asset/-${version}-/-}"`,
		"git/refs/${ref}",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf(".github/workflows/release.yml: missing %q", want)
		}
	}
	for _, name := range []string{
		"tes3mp-easy-${version}-linux-amd64.tar.gz",
		"tes3mp-easy-client-${version}-linux-amd64.tar.gz",
		"tes3mp-easy-client-${version}-windows-amd64.zip",
	} {
		if !strings.Contains(workflow, name) {
			t.Errorf(".github/workflows/release.yml: alias source %q is not listed", name)
		}
	}

	readme := readDoc(t, filepath.Join(root, "README.md"))
	if strings.Contains(readme, "/releases/latest") {
		t.Error("README.md: /releases/latest points at the newest channel, not at the client release")
	}
	for _, url := range []string{
		"https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-windows-amd64.zip",
		"https://github.com/Saagilat/tes3mp-easy/releases/download/client-latest/tes3mp-easy-client-linux-amd64.tar.gz",
		"https://github.com/Saagilat/tes3mp-easy/releases/download/server-latest/tes3mp-easy-linux-amd64.tar.gz",
	} {
		if !strings.Contains(readme, url) {
			t.Errorf("README.md: missing stable download link %q", url)
		}
	}

	// The alias names have no version, so install.sh must keep requiring a digit
	// right after "tes3mp-easy-" when it scans the release list.
	install := readDoc(t, filepath.Join(root, "install.sh"))
	if !strings.Contains(install, `SERVER_ASSET='https://[^"]+/tes3mp-easy-[0-9]`) {
		t.Error("install.sh: SERVER_ASSET must keep matching only versioned server archives")
	}
}
