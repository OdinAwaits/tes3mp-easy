// Package systemdtest provides a recording systemd.Runner for unit and
// integration tests. It never executes a process.
package systemdtest

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

// Call is one recorded Runner invocation.
type Call struct {
	Name string
	Args []string
}

// Response is the canned result for one exact command line.
type Response struct {
	Stdout string
	Err    error
}

// Runner records calls and returns responses registered with Respond.
type Runner struct {
	mu        sync.Mutex
	responses map[string]Response
	calls     []Call
}

var _ systemd.Runner = (*Runner)(nil)

// New returns an empty fake runner.
func New() *Runner {
	return &Runner{responses: make(map[string]Response)}
}

// Respond registers stdout and err for an exact command. Registering the same
// command twice replaces the earlier response.
func (r *Runner) Respond(name string, args []string, stdout string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.responses == nil {
		r.responses = make(map[string]Response)
	}
	r.responses[responseKey(name, args)] = Response{
		Stdout: stdout,
		Err:    err,
	}
}

// Run implements systemd.Runner.
func (r *Runner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	r.calls = append(r.calls, Call{
		Name: name,
		Args: append([]string(nil), args...),
	})
	response, ok := r.responses[responseKey(name, args)]
	r.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("systemdtest: unexpected call: %s", formatCall(name, args))
	}
	return []byte(response.Stdout), response.Err
}

// Calls returns a copy of all recorded invocations.
func (r *Runner) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()

	calls := make([]Call, len(r.calls))
	for i, call := range r.calls {
		calls[i] = Call{
			Name: call.Name,
			Args: append([]string(nil), call.Args...),
		}
	}
	return calls
}

// Reset removes recorded calls and registered responses.
func (r *Runner) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = nil
	r.responses = make(map[string]Response)
}

func responseKey(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), "\x00")
}

func formatCall(name string, args []string) string {
	return strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
}
