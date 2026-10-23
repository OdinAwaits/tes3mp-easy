package ui

import (
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

func TestRunRejectsNilService(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("Run(nil) returned nil")
	}
}

func TestPortText(t *testing.T) {
	if got := portText(0); got != "" {
		t.Fatalf("portText(0) = %q", got)
	}
	if got := portText(-1); got != "" {
		t.Fatalf("portText(-1) = %q", got)
	}
	if got := portText(25565); got != "25565" {
		t.Fatalf("portText(25565) = %q", got)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := map[int64]string{
		0:       "0 B",
		1023:    "1023 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		2 << 20: "2.0 MiB",
		3 << 30: "3.0 GiB",
	}
	for value, want := range tests {
		if got := formatBytes(value); got != want {
			t.Fatalf("formatBytes(%d) = %q, want %q", value, got, want)
		}
	}
}

func TestProgressFraction(t *testing.T) {
	tests := []struct {
		name   string
		update tes3mp.Progress
		want   float64
	}{
		{name: "other stage", update: tes3mp.Progress{
			Stage: tes3mp.StageExtracting}, want: 1,
		},
		{name: "unknown total", update: tes3mp.Progress{
			Stage: tes3mp.StageDownloading, Downloaded: 10}, want: 1,
		},
		{name: "half", update: tes3mp.Progress{
			Stage: tes3mp.StageDownloading, Downloaded: 5, Total: 10}, want: 0.5,
		},
		{name: "clamped", update: tes3mp.Progress{
			Stage: tes3mp.StageDownloading, Downloaded: 20, Total: 10}, want: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := progressFraction(test.update); got != test.want {
				t.Fatalf("progressFraction() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestProgressLabel(t *testing.T) {
	translator := i18n.New(i18n.English)
	got := progressLabel(translator, tes3mp.Progress{
		Stage: tes3mp.StageDownloading, Downloaded: 1 << 20, Total: 2 << 20,
	})
	if got != "Downloading TES3MP: 1.0 MiB of 2.0 MiB" {
		t.Fatalf("download label = %q", got)
	}
	for stage, want := range map[tes3mp.Stage]string{
		tes3mp.StageVerifying:  "Checking the TES3MP archive...",
		tes3mp.StageExtracting: "Extracting TES3MP...",
		tes3mp.StageInstalling: "Finishing the TES3MP installation...",
	} {
		if got := progressLabel(translator, tes3mp.Progress{Stage: stage}); got != want {
			t.Fatalf("progressLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestModsProgress(t *testing.T) {
	translator := i18n.New(i18n.English)
	update := mods.Progress{
		Stage: mods.StageDownloading, Downloaded: 1 << 20, Total: 2 << 20,
	}
	if got := modsProgressLabel(translator, update); got !=
		"Downloading mods: 1.0 MiB of 2.0 MiB" {
		t.Fatalf("mods progress label = %q", got)
	}
	if got := modsProgressFraction(update); got != 0.5 {
		t.Fatalf("mods progress fraction = %v", got)
	}
	if got := modsProgressFraction(mods.Progress{
		Stage: mods.StageExtracting}); got != 1 {
		t.Fatalf("extracting fraction = %v", got)
	}
	for stage, want := range map[mods.Stage]string{
		mods.StageFetching:   "Checking server mods...",
		mods.StageVerifying:  "Checking the mods archive...",
		mods.StageExtracting: "Extracting mods...",
		mods.StageInstalling: "Installing mods...",
	} {
		if got := modsProgressLabel(translator, mods.Progress{Stage: stage}); got != want {
			t.Fatalf("modsProgressLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("abcdef"); got != "abcdef" {
		t.Fatalf("shortSHA = %q", got)
	}
	if got := shortSHA("abcdefghijklmnop"); got != "abcdefghijkl…" {
		t.Fatalf("shortSHA = %q", got)
	}
}

func TestLocaleProgress(t *testing.T) {
	translator := i18n.New(i18n.English)
	update := locale.Progress{
		Stage: locale.StageDownloading, Downloaded: 1 << 20, Total: 2 << 20,
	}
	if got := localeProgressLabel(translator, update); got !=
		"Downloading localization: 1.0 MiB of 2.0 MiB" {
		t.Fatalf("locale progress label = %q", got)
	}
	if got := localeProgressFraction(update); got != 0.5 {
		t.Fatalf("locale progress fraction = %v", got)
	}
	if got := localeProgressFraction(locale.Progress{
		Stage: locale.StageExtracting}); got != 1 {
		t.Fatalf("extracting fraction = %v", got)
	}
	if got := localeProgressLabel(translator, locale.Progress{
		Stage: locale.StageDownloading, Downloaded: 10}); got !=
		"Downloading localization: 10 B of ?" {
		t.Fatalf("unknown total label = %q", got)
	}
	for stage, want := range map[locale.Stage]string{
		locale.StageExtracting: "Extracting localization...",
		locale.StageInstalling: "Applying localization...",
	} {
		if got := localeProgressLabel(translator,
			locale.Progress{Stage: stage}); got != want {
			t.Fatalf("localeProgressLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}
