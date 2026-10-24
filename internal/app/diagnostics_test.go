package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/config"
)

func TestBuildDiagnosticReportRedactsPasswords(t *testing.T) {
	report := BuildDiagnosticReport(ReportInput{
		Generated:  time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
		AppVersion: "1.2.3",
		GOOS:       "linux",
		GOARCH:     "amd64",
		Failure: Failure{
			Kind:   FailureNetwork,
			Code:   CodeNetwork,
			Detail: "connect failed with password=hunter2 and hunter2",
		},
		ConfigPath: "/home/user/config.json",
		WorkDir:    "/home/user/work",
		LogPath:    "/home/user/logs/client.log",
		Servers: []config.Server{{
			Name:     "Friends",
			Address:  "203.0.113.10",
			Port:     25565,
			Password: "hunter2",
		}, {
			Name:    "IPv6",
			Address: "2001:db8::1",
			Port:    25565,
		}},
		LogTail: []string{
			"INFO ready",
			"password = hunter2",
			"ERROR auth hunter2 rejected",
		},
	})
	for _, secret := range []string{"hunter2", "password = hunter2",
		"password=hunter2"} {
		if strings.Contains(report, secret) {
			t.Fatalf("report leaks %q:\n%s", secret, report)
		}
	}
	for _, want := range []string{
		"code: T3E-NET-01",
		"platform: linux/amd64",
		"203.0.113.10:25565", "[2001:db8::1]:25565", "password=set",
		"[redacted]", "recent log:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report does not contain %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "2001:db8::1:25565") {
		t.Fatalf("report contains an ambiguous IPv6 endpoint:\n%s", report)
	}
}

func TestRedactSecretsLongestFirst(t *testing.T) {
	got := RedactSecrets("secret secret-long", "secret", "secret-long")
	if strings.Contains(got, "secret-long") || strings.Contains(got, "secret") {
		t.Fatalf("RedactSecrets = %q", got)
	}
}

func TestExplainAssignsStableCodes(t *testing.T) {
	tests := []struct {
		err  error
		code FailureCode
	}{
		{err: ErrMorrowindNotConfigured, code: CodeMorrowindNotSet},
		{err: ErrModsNotInstalled, code: CodeModsNotInstalled},
		{err: ErrLocalizationNotInstalled, code: CodeLocalization},
		{err: WrapWorkDir(errors.New("bad")), code: CodeWorkDir},
		{err: ErrGameExited, code: CodeGameExited},
	}
	for _, test := range tests {
		if got := Explain(test.err).Code; got != test.code {
			t.Errorf("Explain(%v).Code = %q, want %q", test.err, got, test.code)
		}
	}
}
