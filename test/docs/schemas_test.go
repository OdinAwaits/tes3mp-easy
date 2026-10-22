package docs

import (
	"encoding/json"
	"strings"
	"testing"
)

type fencedBlock struct {
	path    string
	line    int
	lang    string
	content string
}

func fencedBlocks(t *testing.T, path, text string) []fencedBlock {
	t.Helper()

	var (
		blocks  []fencedBlock
		inFence bool
		start   int
		lang    string
		lines   []string
	)
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				start = i + 1
				lang = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
				lines = nil
				continue
			}
			blocks = append(blocks, fencedBlock{
				path:    path,
				line:    start,
				lang:    lang,
				content: strings.Join(lines, "\n"),
			})
			inFence = false
			continue
		}
		if inFence {
			lines = append(lines, line)
		}
	}
	if inFence {
		t.Fatalf("%s:%d: unterminated fenced block", relativeDoc(t, path), start)
	}
	return blocks
}

func TestJSONExamplesAreValid(t *testing.T) {
	for _, path := range docFiles(t) {
		for _, block := range fencedBlocks(t, path, readDoc(t, path)) {
			if block.lang != "json" || strings.TrimSpace(block.content) == "" {
				continue
			}
			var value any
			if err := json.Unmarshal([]byte(block.content), &value); err != nil {
				t.Fatalf("%s:%d: invalid JSON example: %v",
					relativeDoc(t, path), block.line, err)
			}
		}
	}
}

func TestSchemasAreNotDuplicated(t *testing.T) {
	seen := make(map[string]fencedBlock)
	for _, path := range docFiles(t) {
		for _, block := range fencedBlocks(t, path, readDoc(t, path)) {
			if block.lang != "json" {
				continue
			}
			normalized := normalizeJSONBlock(block.content)
			if normalized == "" {
				continue
			}
			if previous, ok := seen[normalized]; ok {
				t.Fatalf(
					"duplicate JSON schema at %s:%d (already at %s:%d)",
					relativeDoc(t, path), block.line,
					relativeDoc(t, previous.path), previous.line)
			}
			seen[normalized] = block
		}
	}
	t.Logf("checked %d distinct JSON schemas", len(seen))
}

func normalizeJSONBlock(content string) string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
