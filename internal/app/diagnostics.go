package app

import (
	"fmt"
	"net"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/config"
)

// ReportInput is the renderer-independent data copied into a support report.
type ReportInput struct {
	Generated  time.Time
	AppVersion string
	GOOS       string
	GOARCH     string
	Failure    Failure
	ConfigPath string
	WorkDir    string
	LogPath    string
	Servers    []config.Server
	LogTail    []string
}

var configPasswordLine = regexp.MustCompile(`(?im)^(\s*password\s*=\s*).*$`)

// RedactSecrets removes known passwords and password-like config lines from
// text that may be pasted into a public support thread.
func RedactSecrets(text string, secrets ...string) string {
	unique := make(map[string]struct{}, len(secrets))
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		unique[secret] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for secret := range unique {
		ordered = append(ordered, secret)
	}
	sort.Slice(ordered, func(left, right int) bool {
		return len(ordered[left]) > len(ordered[right])
	})
	for _, secret := range ordered {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	return configPasswordLine.ReplaceAllString(text, "${1}[redacted]")
}

// BuildDiagnosticReport renders a deterministic text report. It intentionally
// includes addresses and paths for support, but never a server password.
func BuildDiagnosticReport(input ReportInput) string {
	generated := input.Generated
	if generated.IsZero() {
		generated = time.Now()
	}
	var builder strings.Builder
	builder.WriteString("tes3mp-easy diagnostic report\n")
	fmt.Fprintf(&builder, "generated: %s\n", generated.UTC().Format(time.RFC3339))
	fmt.Fprintf(&builder, "app: %s\n", strings.TrimSpace(input.AppVersion))
	fmt.Fprintf(&builder, "platform: %s/%s\n", input.GOOS, input.GOARCH)
	fmt.Fprintf(&builder, "code: %s\n", input.Failure.Code)
	fmt.Fprintf(&builder, "kind: %s\n", input.Failure.Kind)
	if detail := strings.TrimSpace(input.Failure.Detail); detail != "" {
		builder.WriteString("detail:\n")
		builder.WriteString(detail)
		builder.WriteString("\n")
	}
	builder.WriteString("\nenvironment:\n")
	fmt.Fprintf(&builder, "  config: %s\n", input.ConfigPath)
	fmt.Fprintf(&builder, "  work: %s\n", input.WorkDir)
	fmt.Fprintf(&builder, "  log: %s\n", input.LogPath)
	if len(input.Servers) > 0 {
		builder.WriteString("\nservers:\n")
		for _, server := range input.Servers {
			password := "not set"
			if server.Password != "" {
				password = "set"
			}
			fmt.Fprintf(&builder, "  - %s | %s | panel=%s | password=%s\n",
				strings.TrimSpace(server.Name),
				net.JoinHostPort(server.Address, strconv.Itoa(server.Port)),
				server.EffectivePanelURL(), password)
		}
	}
	builder.WriteString("\nrecent log:\n")
	if len(input.LogTail) == 0 {
		builder.WriteString("  (empty)\n")
	} else {
		for _, line := range input.LogTail {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	secrets := make([]string, 0, len(input.Servers))
	for _, server := range input.Servers {
		secrets = append(secrets, server.Password)
	}
	return RedactSecrets(builder.String(), secrets...)
}

// DiagnosticReport builds a support report for err and the supplied log tail.
func (s *Service) DiagnosticReport(err error, logTail []string) string {
	if s == nil {
		return ""
	}
	cfg := s.Config()
	return BuildDiagnosticReport(ReportInput{
		Generated:  time.Now(),
		AppVersion: Version,
		GOOS:       s.goos,
		GOARCH:     runtime.GOARCH,
		Failure:    Explain(err),
		ConfigPath: s.ConfigPath(),
		WorkDir:    s.WorkDir(),
		LogPath:    s.LogPath(),
		Servers:    cfg.Servers,
		LogTail:    logTail,
	})
}
