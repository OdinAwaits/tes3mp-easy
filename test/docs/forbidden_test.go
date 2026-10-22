package docs

import (
	"regexp"
	"strings"
	"testing"
)

var backendFieldPattern = regexp.MustCompile(
	`"(?:backend|backend_name|service_backend)"\s*:`)

// obsoleteBackendNames come from the removed Docker/SSH implementation. The
// words "Docker" and "docker.sock" are still allowed in explanatory Plan B and
// architecture text, but the old operational names must not return.
var obsoleteBackendNames = []string{
	"docker compose",
	"docker-compose",
	"export-server",
	"install-server",
	"start-server",
	"stop-server",
	"restart-server",
	"server-logs",
	"tmux",
	"pterodactyl",
}

func TestNoForbiddenSchemaFields(t *testing.T) {
	for _, path := range docFiles(t) {
		text := readDoc(t, path)
		for _, match := range backendFieldPattern.FindAllStringIndex(text, -1) {
			t.Fatalf("%s:%d: forbidden schema field %q",
				relativeDoc(t, path), lineNumber(text, match[0]),
				text[match[0]:match[1]])
		}
	}
}

func TestNoObsoleteBackendNames(t *testing.T) {
	for _, path := range docFiles(t) {
		text := strings.ToLower(readDoc(t, path))
		for _, name := range obsoleteBackendNames {
			index := strings.Index(text, name)
			if index < 0 {
				continue
			}
			t.Fatalf("%s:%d: obsolete backend name %q",
				relativeDoc(t, path), lineNumber(text, index), name)
		}
	}
}

func lineNumber(text string, offset int) int {
	if offset < 0 {
		return 1
	}
	if offset > len(text) {
		offset = len(text)
	}
	return strings.Count(text[:offset], "\n") + 1
}
