package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"
)

const runTimeout = 15 * time.Second

type harness struct {
	binary  string
	fakeBin string
}

type result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *envelopeError  `json:"error"`
}

type envelopeError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// Run executes the binary with a fresh temp root.
func (h *harness) Run(t *testing.T, args ...string) result {
	t.Helper()
	return h.RunRoot(t, t.TempDir(), args...)
}

// RunEnv executes the binary with a fresh temp root and extra environment.
func (h *harness) RunEnv(t *testing.T, env map[string]string, args ...string) result {
	t.Helper()
	return h.RunRootEnv(t, t.TempDir(), env, args...)
}

// RunRoot executes the binary with --root root.
func (h *harness) RunRoot(t *testing.T, root string, args ...string) result {
	t.Helper()
	return h.RunRootEnv(t, root, nil, args...)
}

// RunRootEnv executes the binary with --root root and extra environment.
func (h *harness) RunRootEnv(t *testing.T, root string,
	env map[string]string, args ...string,
) result {
	t.Helper()
	full := append([]string{"--root", root}, args...)
	return h.run(t, root, full, env)
}

// RunArgs executes the binary without injecting --root.
func (h *harness) RunArgs(t *testing.T, args ...string) result {
	t.Helper()
	return h.run(t, t.TempDir(), args, nil)
}

// RunArgsEnv executes the binary without injecting --root and with extra
// environment. It lets tests place global flags after the command.
func (h *harness) RunArgsEnv(t *testing.T, env map[string]string,
	args ...string,
) result {
	t.Helper()
	return h.run(t, t.TempDir(), args, env)
}

func (h *harness) run(t *testing.T, dir string, args []string,
	extraEnv map[string]string,
) result {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.binary, args...)
	cmd.Dir = dir
	cmd.Env = h.environment(extraEnv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %v timed out after %v", h.binary, args, runTimeout)
	}

	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %s %v: %v", h.binary, args, err)
		}
		exitCode = exitErr.ExitCode()
	}

	return result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

func (h *harness) environment(extra map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	values["GOMAXPROCS"] = "1"
	values["TZ"] = "UTC"
	if h.fakeBin != "" {
		if path := values["PATH"]; path != "" {
			values["PATH"] = h.fakeBin + string(os.PathListSeparator) + path
		} else {
			values["PATH"] = h.fakeBin
		}
	}
	for key, value := range extra {
		values[key] = value
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func (r result) requireExit(t *testing.T, want int) {
	t.Helper()
	if r.ExitCode != want {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			r.ExitCode, want, r.Stdout, r.Stderr)
	}
}

func (r result) decodeEnvelope(t *testing.T) envelope {
	t.Helper()

	if strings.TrimSpace(r.Stdout) == "" {
		t.Fatalf("stdout is empty; stderr=%q", r.Stderr)
	}

	decoder := json.NewDecoder(strings.NewReader(r.Stdout))
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		t.Fatalf("stdout is not JSON: %v; stdout=%q", err, r.Stdout)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout contains more than one JSON value: %v; stdout=%q",
			err, r.Stdout)
	}

	object, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("stdout JSON is %T, want object", raw)
	}
	assertNoForbiddenFields(t, object, "$")

	var env envelope
	if err := json.Unmarshal([]byte(r.Stdout), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if _, ok := object["ok"]; !ok {
		t.Fatalf("envelope has no ok field: %s", r.Stdout)
	}
	if env.OK {
		if env.Error != nil {
			t.Fatalf("success envelope has error: %s", r.Stdout)
		}
		if _, ok := object["data"]; !ok {
			t.Fatalf("success envelope has no data: %s", r.Stdout)
		}
		return env
	}

	if env.Error == nil {
		t.Fatalf("failure envelope has no error: %s", r.Stdout)
	}
	if _, ok := object["data"]; ok {
		t.Fatalf("failure envelope contains data: %s", r.Stdout)
	}
	return env
}

func (r result) requireJSONError(t *testing.T, exit int, code string) envelope {
	t.Helper()

	r.requireExit(t, exit)
	if r.Stderr != "" {
		t.Fatalf("stderr = %q, want empty in JSON mode", r.Stderr)
	}
	env := r.decodeEnvelope(t)
	if env.OK {
		t.Fatalf("ok = true, want false; stdout=%q", r.Stdout)
	}
	if env.Error.Code != code {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, code)
	}
	if strings.TrimSpace(env.Error.Message) == "" {
		t.Fatalf("error.message is empty; stdout=%q", r.Stdout)
	}
	return env
}

func (r result) requireHumanError(t *testing.T, exit int, message string) {
	t.Helper()

	r.requireExit(t, exit)
	if r.Stdout != "" {
		t.Fatalf("stdout = %q, want empty in human mode", r.Stdout)
	}
	if !strings.Contains(r.Stderr, message) {
		t.Fatalf("stderr = %q, want substring %q", r.Stderr, message)
	}
}

func assertNoForbiddenFields(t *testing.T, value any, path string) {
	t.Helper()

	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "backend" {
				t.Fatalf("forbidden field %s.backend in JSON contract", path)
			}
			assertNoForbiddenFields(t, child, path+"."+key)
		}
	case []any:
		for i, child := range v {
			assertNoForbiddenFields(t, child, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}
