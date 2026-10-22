package docs

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	inlineLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	refLinkPattern    = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*(\S+)`)
)

func TestLocalLinksExist(t *testing.T) {
	for _, path := range docFiles(t) {
		text := readDoc(t, path)
		dir := filepath.Dir(path)
		inFence := false

		for i, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}

			var targets []string
			for _, match := range inlineLinkPattern.FindAllStringSubmatch(line, -1) {
				targets = append(targets, match[1])
			}
			if match := refLinkPattern.FindStringSubmatch(line); match != nil {
				targets = append(targets, match[1])
			}
			for _, target := range targets {
				checkLocalLink(t, path, dir, i+1, target)
			}
		}
	}
}

func checkLocalLink(t *testing.T, docPath, dir string, line int, raw string) {
	t.Helper()

	target := linkPath(raw)
	if target == "" || strings.HasPrefix(target, "#") {
		return
	}

	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("%s:%d: parse link %q: %v", relativeDoc(t, docPath), line, raw, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return
	}
	if parsed.Path == "" {
		return
	}

	local, err := url.PathUnescape(parsed.Path)
	if err != nil {
		t.Fatalf("%s:%d: unescape link %q: %v", relativeDoc(t, docPath), line, raw, err)
	}
	if strings.HasPrefix(local, "/") {
		t.Fatalf("%s:%d: local link %q must be relative", relativeDoc(t, docPath), line, raw)
	}

	absolute := filepath.Join(dir, filepath.FromSlash(local))
	if _, err := os.Stat(absolute); err != nil {
		t.Fatalf("%s:%d: broken link %q: %v",
			relativeDoc(t, docPath), line, raw, err)
	}
}

func linkPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "<") {
		if end := strings.IndexByte(raw, '>'); end > 1 {
			return raw[1:end]
		}
	}
	if fields := strings.Fields(raw); len(fields) > 0 {
		return fields[0]
	}
	return raw
}
