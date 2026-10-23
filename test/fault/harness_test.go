package fault

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"
)

const runTimeout = 2 * time.Minute

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

func command(ctx context.Context, root string, extraEnv map[string]string,
	args ...string,
) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	full := append([]string{"--root", root}, args...)
	cmd := exec.CommandContext(ctx, opsBinary, full...)
	cmd.Dir = root
	cmd.Env = environment(extraEnv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	return cmd, &stdout, &stderr
}

func runRoot(t *testing.T, root string, extraEnv map[string]string,
	args ...string,
) result {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	cmd, stdout, stderr := command(ctx, root, extraEnv, args...)
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("ops %v timed out after %v", args, runTimeout)
	}

	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run ops %v: %v", args, err)
		}
		exitCode = exitErr.ExitCode()
	}

	return result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

func environment(extra map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	values["GOMAXPROCS"] = "1"
	values["TZ"] = "UTC"
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
	if _, ok := raw.(map[string]any); !ok {
		t.Fatalf("stdout JSON is %T, want object; stdout=%q", raw, r.Stdout)
	}

	var env envelope
	if err := json.Unmarshal([]byte(r.Stdout), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env
}

func requireFailureCode(t *testing.T, env envelope, code string) {
	t.Helper()
	if env.OK || env.Error == nil {
		t.Fatalf("envelope = %+v, want failure", env)
	}
	if env.Error.Code != code {
		t.Fatalf("error.code = %q, want %q; message=%q",
			env.Error.Code, code, env.Error.Message)
	}
}
