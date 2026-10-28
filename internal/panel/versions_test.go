package panel

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/serverruntime"
)

func writeInstallMarker(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir runtime root: %v", err)
	}
	path := filepath.Join(root, serverruntime.InfoFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write install marker: %v", err)
	}
}

// The admin header must answer "which server release is deployed?" on every
// admin page, not only on the dashboard.
func TestAdminHeaderShowsInstalledServerVersion(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	env.h.cfg.RuntimeRoot = t.TempDir()
	writeInstallMarker(t, env.h.cfg.RuntimeRoot, "TES3MP_VERSION=0.8.1\n")

	want := "TES3MP 0.8.1 · panel " + Version + " · client " + ClientVersion
	for _, path := range []string{"/admin", "/admin/logs", "/admin/config"} {
		rec := env.request(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, want) {
			t.Fatalf("%s does not contain %q:\n%s", path, want, body)
		}
	}
}

func TestAdminHeaderShowsDashWithoutInstallMarker(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	// An empty runtime root is a valid state: an install that predates the
	// marker keeps working, the header just cannot name its release.
	env.h.cfg.RuntimeRoot = t.TempDir()

	rec := env.request(t, http.MethodGet, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin = %d: %s", rec.Code, rec.Body.String())
	}
	want := "TES3MP — · panel " + Version + " · client " + ClientVersion
	if body := rec.Body.String(); !strings.Contains(body, want) {
		t.Fatalf("dashboard does not contain %q:\n%s", want, body)
	}
}
