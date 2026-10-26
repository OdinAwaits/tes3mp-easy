package app

import (
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
)

func TestClientSettingsRoundTrip(t *testing.T) {
	service, _, _ := installedService(t)
	path, err := service.ClientSettingsPath()
	if err != nil {
		t.Fatalf("ClientSettingsPath: %v", err)
	}
	if !filepath.IsAbs(path) || filepath.Base(path) != openmwcfg.FileName {
		t.Fatalf("path = %q, want an absolute settings.cfg path", path)
	}
	configDir, err := service.OpenMWConfigDir()
	if err != nil {
		t.Fatalf("OpenMWConfigDir: %v", err)
	}
	if filepath.Dir(path) != configDir {
		t.Fatalf("dir = %q, want %q", filepath.Dir(path), configDir)
	}

	before, err := service.ClientSettings()
	if err != nil {
		t.Fatalf("ClientSettings: %v", err)
	}
	if before.Exists || before.Configured {
		t.Fatalf("state = %+v, want a missing file", before)
	}

	values := openmwcfg.Defaults()
	values.FramerateLimit = 120
	values.ScalingFactor = 1.25
	updated, err := service.UpdateClientSettings(values)
	if err != nil {
		t.Fatalf("UpdateClientSettings: %v", err)
	}
	if !updated.Configured || updated.Values != values {
		t.Fatalf("state = %+v, want %+v", updated, values)
	}

	reloaded, err := service.ClientSettings()
	if err != nil {
		t.Fatalf("ClientSettings: %v", err)
	}
	if reloaded.Values != values {
		t.Fatalf("values = %+v, want %+v", reloaded.Values, values)
	}
}
