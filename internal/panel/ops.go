package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/contract"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

const (
	// DefaultOpsBinary is the production ops executable. The panel invokes it
	// through a narrow sudoers rule instead of writing game data itself.
	DefaultOpsBinary = "/opt/tes3mp-easy/bin/tes3mp-easy-ops"

	defaultOpsUser         = "tes3mp"
	opsReadTimeout         = 5 * time.Second
	opsMutationTimeout     = 2 * time.Minute
	opsLongMutationTimeout = 10 * time.Minute
	opsStdoutLimit         = 1 << 20
	opsStderrLimit         = 64 << 10
)

// Status is the decoded `data` object of `ops status`.
type Status struct {
	Unit      string  `json:"unit"`
	State     string  `json:"state"`
	Health    string  `json:"health"`
	StartedAt *string `json:"started_at"`
}

// BackupItem is one item from `ops backups list`.
type BackupItem struct {
	Name    string  `json:"name"`
	Size    int64   `json:"size"`
	Mtime   string  `json:"mtime"`
	Current bool    `json:"current"`
	SHA256  *string `json:"sha256,omitempty"`
}

// Backups is the decoded `data` object of `ops backups list`.
type Backups struct {
	Type       string       `json:"type"`
	Current    *string      `json:"current"`
	Items      []BackupItem `json:"items"`
	NextBefore *string      `json:"next_before"`
}

// Backup is the decoded `data` object of `ops backups create`.
type Backup struct {
	Archive  string   `json:"archive,omitempty"`
	Size     int64    `json:"size,omitempty"`
	SHA256   string   `json:"sha256,omitempty"`
	Skipped  bool     `json:"skipped"`
	Reason   string   `json:"reason,omitempty"`
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

// ImportResult is the decoded `data` object of `ops mods import`.
type ImportResult struct {
	Archive  string   `json:"archive"`
	SHA256   string   `json:"sha256"`
	Plugins  int      `json:"plugins"`
	Scripts  int      `json:"scripts"`
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

// DeployResult combines the fields of `ops mods deploy` and `ops state deploy`.
// Fields that do not belong to the executed command remain zero-valued.
type DeployResult struct {
	Archive         string  `json:"archive"`
	SHA256          string  `json:"sha256,omitempty"`
	Previous        *string `json:"previous,omitempty"`
	StateBackup     string  `json:"state_backup,omitempty"`
	ModsArchive     string  `json:"mods_archive,omitempty"`
	PredeployBackup string  `json:"predeploy_backup,omitempty"`
	Restarted       bool    `json:"restarted"`
}

// ConfigSnapshot is the full decoded `data` object of `ops config get`. The
// public Ops interface exposes only Values through ConfigGet so the admin
// method when it also needs read_only keys.
type ConfigSnapshot struct {
	File     string         `json:"file"`
	Values   map[string]any `json:"values"`
	ReadOnly []string       `json:"read_only"`
}

// Doctor is the decoded `data` object of `ops doctor`.
type Doctor struct {
	Status string        `json:"status"`
	Checks []DoctorCheck `json:"checks"`
}

// DoctorCheck is one environment check reported by ops doctor.
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Ops is the panel-facing view of tes3mp-easy-ops. Handlers depend on this
// interface so tests can replace the subprocess with a fake.
type Ops interface {
	Status(ctx context.Context, unit string) (Status, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Logs(ctx context.Context, tail int) ([]string, error)
	BackupsList(ctx context.Context, typ string, limit int, before string) (Backups, error)
	BackupCreate(ctx context.Context, typ string, manual bool) (Backup, error)
	ModsImport(ctx context.Context, path string) (ImportResult, error)
	ModsDeploy(ctx context.Context, ref string) (DeployResult, error)
	StateDeploy(ctx context.Context, name string) (DeployResult, error)
	StaffSet(ctx context.Context, player string, rank int) error
	ConfigGet(ctx context.Context, file string) (map[string]any, error)
	ConfigSet(ctx context.Context, file, key string, value any) error
}

// OpsError is a failed ops invocation. Exit is authoritative for the HTTP
// status; Code and Message come from the JSON envelope when it is available.
type OpsError struct {
	Exit    ops.ExitCode
	Code    string
	Message string
	Details map[string]any
	Cause   error
}

// Error implements error.
func (e *OpsError) Error() string {
	if e == nil {
		return "ops command failed"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "ops command failed"
}

// Unwrap exposes the process/context error, if any.
func (e *OpsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// HTTPStatus maps the ops exit code to the public HTTP status from
// docs/v2/03-contracts.md §3.
func (e *OpsError) HTTPStatus() int {
	if e == nil {
		return 500
	}
	// A manually constructed error may carry only error.code. It is still a
	// failure, so never map the zero Exit value to 200.
	if e.Exit == ops.ExitOK {
		if e.Code != "" {
			return OpsHTTPStatusForCode(e.Code)
		}
		return 500
	}
	return opsHTTPStatusForExit(e.Exit)
}

// OpsHTTPStatus returns the HTTP status for an ops client error. Unknown errors
// and non-ops failures are internal server errors.
func OpsHTTPStatus(err error) int {
	if err == nil {
		return 200
	}
	var opsErr *OpsError
	if errors.As(err, &opsErr) {
		return opsErr.HTTPStatus()
	}
	return 500
}

// OpsHTTPStatusForCode maps a stable error.code when no process exit status is
// available (for example, while validating an envelope).
func OpsHTTPStatusForCode(code string) int {
	switch code {
	case ops.CodeInvalidArgument, ops.CodeUnsafePath:
		return 400
	case ops.CodeNotFound:
		return 404
	case ops.CodeLocked:
		return 409
	case ops.CodeNoSpace:
		return 507
	case ops.CodeServiceUnavailable, ops.CodeChecksFailed:
		return 503
	case ops.CodeArchiveInvalid, ops.CodeConfigInvalid:
		return 422
	default:
		return 500
	}
}

func opsHTTPStatusForExit(exit ops.ExitCode) int {
	switch exit {
	case ops.ExitOK:
		return 200
	case ops.ExitInvalidArgument, ops.ExitUnsafePath:
		return 400
	case ops.ExitNotFound:
		return 404
	case ops.ExitLocked:
		return 409
	case ops.ExitNoSpace:
		return 507
	case ops.ExitServiceUnavailable, ops.ExitChecksFailed:
		return 503
	case ops.ExitArchiveInvalid, ops.ExitConfigInvalid:
		return 422
	default:
		return 500
	}
}

// OpsRunner executes one process without a shell. A non-zero exit status is
// returned in exitCode with err == nil; err is reserved for start failures and
// context cancellation. Tests inject a fake runner.
type OpsRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)
}

// OpsClientConfig configures the production subprocess client. Direct is for
// tests and developer runs only; production must use the sudoers rule.
type OpsClientConfig struct {
	Binary string
	Root   string
	Direct bool
	// SudoPath and RunAs are used when Direct is false.
	SudoPath string
	RunAs    string

	ReadTimeout         time.Duration
	MutationTimeout     time.Duration
	LongMutationTimeout time.Duration

	Runner OpsRunner
	Logger *log.Logger
}

// OpsClient invokes tes3mp-easy-ops with fixed arguments and decodes its JSON
// envelope. It never uses a shell and never retries a mutation.
type OpsClient struct {
	binary string
	root   string
	direct bool

	sudoPath string
	runAs    string

	readTimeout         time.Duration
	mutationTimeout     time.Duration
	longMutationTimeout time.Duration

	runner OpsRunner
	logger *log.Logger
}

// NewOpsClient validates cfg and fills production defaults. The returned client
// is safe for concurrent use; the ops lock still serializes mutations.
func NewOpsClient(cfg OpsClientConfig) (*OpsClient, error) {
	if cfg.Binary == "" {
		cfg.Binary = DefaultOpsBinary
	}
	if cfg.Root == "" {
		cfg.Root = DefaultRoot
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = opsReadTimeout
	}
	if cfg.MutationTimeout == 0 {
		cfg.MutationTimeout = opsMutationTimeout
	}
	if cfg.LongMutationTimeout == 0 {
		cfg.LongMutationTimeout = opsLongMutationTimeout
	}
	if cfg.Runner == nil {
		cfg.Runner = execOpsRunner{}
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if !cfg.Direct {
		if cfg.SudoPath == "" {
			cfg.SudoPath = "sudo"
		}
		if cfg.RunAs == "" {
			cfg.RunAs = defaultOpsUser
		}
	}

	switch {
	case strings.TrimSpace(cfg.Binary) == "":
		return nil, errors.New("ops binary is empty")
	case strings.TrimSpace(cfg.Root) == "":
		return nil, errors.New("ops root is empty")
	case cfg.ReadTimeout < 0:
		return nil, errors.New("ops read timeout must not be negative")
	case cfg.MutationTimeout < 0:
		return nil, errors.New("ops mutation timeout must not be negative")
	case cfg.LongMutationTimeout < 0:
		return nil, errors.New("ops long mutation timeout must not be negative")
	case !cfg.Direct && strings.TrimSpace(cfg.SudoPath) == "":
		return nil, errors.New("sudo path is empty")
	case !cfg.Direct && strings.TrimSpace(cfg.RunAs) == "":
		return nil, errors.New("ops run-as user is empty")
	}

	return &OpsClient{
		binary:              cfg.Binary,
		root:                cfg.Root,
		direct:              cfg.Direct,
		sudoPath:            cfg.SudoPath,
		runAs:               cfg.RunAs,
		readTimeout:         cfg.ReadTimeout,
		mutationTimeout:     cfg.MutationTimeout,
		longMutationTimeout: cfg.LongMutationTimeout,
		runner:              cfg.Runner,
		logger:              cfg.Logger,
	}, nil
}

// NewDirectOpsClient is a convenience constructor for local use and tests. It
// does not prepend sudo.
func NewDirectOpsClient(binary, root string, runner OpsRunner) (*OpsClient, error) {
	return NewOpsClient(OpsClientConfig{
		Binary: binary,
		Root:   root,
		Direct: true,
		Runner: runner,
	})
}

type execOpsRunner struct{}

func (execOpsRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout := newCappedBuffer(opsStdoutLimit)
	stderr := newCappedBuffer(opsStderrLimit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout.Bytes(), stderr.Bytes(), -1, ctxErr
	}
	if err == nil {
		if stdout.Truncated() {
			return stdout.Bytes(), stderr.Bytes(), -1, errOpsOutputLimit
		}
		return stdout.Bytes(), stderr.Bytes(), 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code < 0 {
			code = int(ops.ExitInternal)
		}
		if stdout.Truncated() {
			return stdout.Bytes(), stderr.Bytes(), -1, errOpsOutputLimit
		}
		return stdout.Bytes(), stderr.Bytes(), code, nil
	}
	return stdout.Bytes(), stderr.Bytes(), -1, err
}

type opsEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *contract.Error `json:"error"`
}

// call runs one read/mutation command, enforces its timeout and returns the raw
// success data. operation is only used in logs and errors; args contains the
// full ops command array. The exit status, not the message text, decides the
// HTTP status.
func (c *OpsClient) call(ctx context.Context, timeout time.Duration,
	operation string, args ...string,
) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name, argv := c.argv(args)
	stdout, stderr, exitCode, runErr := c.runner.Run(callCtx, name, argv...)
	if len(stderr) > opsStderrLimit {
		stderr = stderr[:opsStderrLimit]
	}
	c.logStderr(operation, stderr)

	if ctxErr := callCtx.Err(); ctxErr != nil {
		return nil, c.contextError(operation, timeout, ctxErr)
	}
	if len(stdout) > opsStdoutLimit {
		return nil, c.protocolError(operation, errOpsOutputLimit)
	}
	if runErr != nil {
		return nil, &OpsError{
			Exit:    ops.ExitInternal,
			Code:    ops.CodeInternal,
			Message: fmt.Sprintf("ops %s: %v", operation, runErr),
			Cause:   runErr,
		}
	}
	if exitCode != 0 {
		// doctor is the one contract exception: exit 10 still carries
		// {"ok":true,"data":{...}} with the failed checks. Return the data so
		// Doctor can show them, and let Doctor build the checks_failed error.
		if exitCode == int(ops.ExitChecksFailed) {
			envelope, decodeErr := decodeOpsEnvelope(stdout)
			if decodeErr == nil && envelope.OK && len(envelope.Data) > 0 {
				return envelope.Data, nil
			}
		}
		return nil, c.exitError(operation, exitCode, stdout)
	}

	envelope, err := decodeOpsEnvelope(stdout)
	if err != nil {
		return nil, c.protocolError(operation, err)
	}
	if !envelope.OK {
		// A failed envelope must carry a non-zero exit status. Treat this as a
		// protocol error but still surface a message ops gave us.
		result := &OpsError{
			Exit:    ops.ExitInternal,
			Code:    ops.CodeInternal,
			Message: "ops command reported failure with exit status 0",
		}
		if envelope.Error != nil {
			if envelope.Error.Code != "" {
				result.Code = envelope.Error.Code
			}
			if envelope.Error.Message != "" {
				result.Message = envelope.Error.Message
			}
			result.Details = envelope.Error.Details
		}
		return nil, result
	}
	return envelope.Data, nil
}

func (c *OpsClient) argv(args []string) (string, []string) {
	argv := make([]string, 0, 4+len(args))
	argv = append(argv, "--root", c.root, "--json")
	argv = append(argv, args...)

	if c.direct {
		return c.binary, argv
	}
	sudoArgs := make([]string, 0, 4+len(argv))
	sudoArgs = append(sudoArgs, "-n", "-u", c.runAs, c.binary)
	sudoArgs = append(sudoArgs, argv...)
	return c.sudoPath, sudoArgs
}

func (c *OpsClient) exitError(command string, exitCode int, stdout []byte) *OpsError {
	exit := ops.ExitCode(exitCode)
	result := &OpsError{
		Exit:    exit,
		Code:    exit.ErrorCode(),
		Message: fmt.Sprintf("ops %s failed (exit %d)", command, exitCode),
	}

	envelope, err := decodeOpsEnvelope(stdout)
	if err != nil {
		c.logger.Printf("ops %s: decode failure envelope: %v", command, err)
		return result
	}
	if envelope.Error == nil {
		c.logger.Printf("ops %s: exit %d without error object", command, exitCode)
		return result
	}
	if envelope.Error.Code != "" {
		result.Code = envelope.Error.Code
	}
	if envelope.Error.Message != "" {
		result.Message = envelope.Error.Message
	}
	result.Details = envelope.Error.Details
	return result
}

func (c *OpsClient) contextError(command string, timeout time.Duration,
	err error,
) *OpsError {
	message := fmt.Sprintf("ops %s: canceled", command)
	if errors.Is(err, context.DeadlineExceeded) {
		message = fmt.Sprintf("ops %s: timed out after %s", command, timeout)
	}
	return &OpsError{
		Exit:    ops.ExitInternal,
		Code:    ops.CodeInternal,
		Message: message,
		Cause:   err,
	}
}

func (c *OpsClient) protocolError(command string, err error) *OpsError {
	c.logger.Printf("ops %s: invalid response: %v", command, err)
	return &OpsError{
		Exit:    ops.ExitInternal,
		Code:    ops.CodeInternal,
		Message: fmt.Sprintf("ops %s: invalid response", command),
		Cause:   err,
	}
}

func decodeOpsEnvelope(stdout []byte) (opsEnvelope, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return opsEnvelope{}, errors.New("stdout is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout))
	var envelope opsEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return opsEnvelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return opsEnvelope{}, errors.New("stdout contains more than one JSON object")
		}
		return opsEnvelope{}, fmt.Errorf("trailing data: %w", err)
	}

	switch {
	case envelope.OK && envelope.Error != nil:
		return opsEnvelope{}, errors.New("success envelope contains an error")
	case !envelope.OK && envelope.Error == nil:
		return opsEnvelope{}, errors.New("failure envelope has no error")
	}
	return envelope, nil
}

func (c *OpsClient) decodeData(command string, raw json.RawMessage, target any) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return c.protocolError(command, errors.New("response has no data"))
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return c.protocolError(command, fmt.Errorf("decode data: %w", err))
	}
	return nil
}

func (c *OpsClient) logStderr(command string, stderr []byte) {
	text := strings.TrimSpace(string(stderr))
	if text == "" {
		return
	}
	c.logger.Printf("ops %s: stderr: %s", command, text)
}

var errOpsOutputLimit = errors.New("ops stdout exceeds size limit")

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *cappedBuffer) Truncated() bool {
	return b.truncated
}

// Status reads the state of tes3mp.service or the backup timer.
func (c *OpsClient) Status(ctx context.Context, unit string) (Status, error) {
	args := []string{"status"}
	if unit != "" {
		args = append(args, "--unit", unit)
	}
	raw, err := c.call(ctx, c.readTimeout, "status", args...)
	if err != nil {
		return Status{}, err
	}
	var data Status
	if err := c.decodeData("status", raw, &data); err != nil {
		return Status{}, err
	}
	return data, nil
}

// Doctor runs environment checks. It returns the report even when the report
// status is error; in that case err is an *OpsError with exit 10 / HTTP 503.
func (c *OpsClient) Doctor(ctx context.Context) (Doctor, error) {
	raw, err := c.call(ctx, c.readTimeout, "doctor", "doctor")
	if err != nil {
		return Doctor{}, err
	}
	var report Doctor
	if err := c.decodeData("doctor", raw, &report); err != nil {
		return Doctor{}, err
	}
	if report.Checks == nil {
		report.Checks = []DoctorCheck{}
	}
	if report.Status == "error" {
		return report, &OpsError{
			Exit:    ops.ExitChecksFailed,
			Code:    ops.CodeChecksFailed,
			Message: "ops doctor found errors",
			Details: map[string]any{"checks": report.Checks},
		}
	}
	return report, nil
}

// Start asks ops to start tes3mp.service.
func (c *OpsClient) Start(ctx context.Context) error {
	_, err := c.call(ctx, c.mutationTimeout, "start", "start")
	return err
}

// Stop asks ops to stop tes3mp.service.
func (c *OpsClient) Stop(ctx context.Context) error {
	_, err := c.call(ctx, c.mutationTimeout, "stop", "stop")
	return err
}

// Restart asks ops to restart tes3mp.service.
func (c *OpsClient) Restart(ctx context.Context) error {
	_, err := c.call(ctx, c.mutationTimeout, "restart", "restart")
	return err
}

// Logs reads the last tail journald lines for tes3mp.service.
func (c *OpsClient) Logs(ctx context.Context, tail int) ([]string, error) {
	if tail < 1 {
		return nil, opsArgumentError("logs", "tail must be positive")
	}
	raw, err := c.call(ctx, c.readTimeout, "logs", "logs", "--tail", strconv.Itoa(tail))
	if err != nil {
		return nil, err
	}
	var data struct {
		Lines []string `json:"lines"`
	}
	if err := c.decodeData("logs", raw, &data); err != nil {
		return nil, err
	}
	return nonNilStrings(data.Lines), nil
}

// BackupsList returns one page of mods or state archives. A zero limit keeps
// the ops default (200).
func (c *OpsClient) BackupsList(ctx context.Context, typ string, limit int,
	before string,
) (Backups, error) {
	if typ == "" {
		return Backups{}, opsArgumentError("backups list", "type is required")
	}
	if limit < 0 {
		return Backups{}, opsArgumentError("backups list", "limit must not be negative")
	}
	if limit > 500 {
		return Backups{}, opsArgumentError("backups list", "limit must not exceed 500")
	}
	args := []string{"backups", "list", "--type", typ}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	if before != "" {
		args = append(args, "--before", before)
	}
	raw, err := c.call(ctx, c.readTimeout, "backups list", args...)
	if err != nil {
		return Backups{}, err
	}
	var data Backups
	if err := c.decodeData("backups list", raw, &data); err != nil {
		return Backups{}, err
	}
	if data.Items == nil {
		data.Items = []BackupItem{}
	}
	return data, nil
}

// BackupCreate creates a state archive; manual selects state-manual-*.
func (c *OpsClient) BackupCreate(ctx context.Context, typ string, manual bool) (Backup, error) {
	if typ == "" {
		return Backup{}, opsArgumentError("backups create", "type is required")
	}
	args := []string{"backups", "create", "--type", typ}
	if manual {
		args = append(args, "--manual")
	}
	raw, err := c.call(ctx, c.longMutationTimeout, "backups create", args...)
	if err != nil {
		return Backup{}, err
	}
	var data Backup
	if err := c.decodeData("backups create", raw, &data); err != nil {
		return Backup{}, err
	}
	data.Pruned = nonNilStrings(data.Pruned)
	data.Warnings = nonNilStrings(data.Warnings)
	return data, nil
}

// ModsImport validates and stores an uploaded mods archive.
func (c *OpsClient) ModsImport(ctx context.Context, path string) (ImportResult, error) {
	if path == "" {
		return ImportResult{}, opsArgumentError("mods import", "file path is required")
	}
	raw, err := c.call(ctx, c.longMutationTimeout, "mods import",
		"mods", "import", "--file", path)
	if err != nil {
		return ImportResult{}, err
	}
	var data ImportResult
	if err := c.decodeData("mods import", raw, &data); err != nil {
		return ImportResult{}, err
	}
	data.Pruned = nonNilStrings(data.Pruned)
	data.Warnings = nonNilStrings(data.Warnings)
	return data, nil
}

// ModsDeploy deploys "current", "latest" or an explicit archive name.
func (c *OpsClient) ModsDeploy(ctx context.Context, ref string) (DeployResult, error) {
	args := []string{"mods", "deploy"}
	switch ref {
	case "":
		return DeployResult{}, opsArgumentError("mods deploy",
			"current, latest or an archive name is required")
	case "current":
		args = append(args, "--current")
	case "latest":
		args = append(args, "--latest")
	default:
		args = append(args, "--archive", ref)
	}
	raw, err := c.call(ctx, c.longMutationTimeout, "mods deploy", args...)
	if err != nil {
		return DeployResult{}, err
	}
	var data DeployResult
	if err := c.decodeData("mods deploy", raw, &data); err != nil {
		return DeployResult{}, err
	}
	return data, nil
}

// StateDeploy restores "latest" or the named state archive.
func (c *OpsClient) StateDeploy(ctx context.Context, name string) (DeployResult, error) {
	args := []string{"state", "deploy"}
	switch name {
	case "":
		return DeployResult{}, opsArgumentError("state deploy",
			"latest or an archive name is required")
	case "latest":
		args = append(args, "--latest")
	default:
		args = append(args, "--archive", name)
	}
	raw, err := c.call(ctx, c.longMutationTimeout, "state deploy", args...)
	if err != nil {
		return DeployResult{}, err
	}
	var data DeployResult
	if err := c.decodeData("state deploy", raw, &data); err != nil {
		return DeployResult{}, err
	}
	return data, nil
}

// StaffSet changes one player's staff rank. The player name is passed as a
// single argv element, never through a shell.
func (c *OpsClient) StaffSet(ctx context.Context, player string, rank int) error {
	if player == "" {
		return opsArgumentError("staff set", "player is required")
	}
	_, err := c.call(ctx, c.mutationTimeout, "staff set",
		"staff", "set", "--player", player, "--rank", strconv.Itoa(rank))
	return err
}

// ConfigSnapshot reads values and read_only keys for one config file.
func (c *OpsClient) ConfigSnapshot(ctx context.Context, file string) (ConfigSnapshot, error) {
	if file == "" {
		return ConfigSnapshot{}, opsArgumentError("config get", "file is required")
	}
	raw, err := c.call(ctx, c.readTimeout, "config get",
		"config", "get", "--file", file)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	var data ConfigSnapshot
	if err := c.decodeData("config get", raw, &data); err != nil {
		return ConfigSnapshot{}, err
	}
	if data.Values == nil {
		data.Values = map[string]any{}
	}
	data.ReadOnly = nonNilStrings(data.ReadOnly)
	return data, nil
}

// ConfigGet returns only the editable values, matching the Ops interface.
func (c *OpsClient) ConfigGet(ctx context.Context, file string) (map[string]any, error) {
	data, err := c.ConfigSnapshot(ctx, file)
	if err != nil {
		return nil, err
	}
	return data.Values, nil
}

// ConfigSet writes one config key through ops. Strings are passed verbatim;
// numbers and booleans keep their literal representation.
func (c *OpsClient) ConfigSet(ctx context.Context, file, key string, value any) error {
	if file == "" {
		return opsArgumentError("config set", "file is required")
	}
	if key == "" {
		return opsArgumentError("config set", "key is required")
	}
	text, err := configValueString(value)
	if err != nil {
		return opsArgumentError("config set", err.Error())
	}
	_, err = c.call(ctx, c.mutationTimeout, "config set",
		"config", "set", "--file", file, "--key", key, "--value", text)
	return err
}

func configValueString(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		value := float64(typed)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "", errors.New("value must be a finite number")
		}
		return strconv.FormatFloat(value, 'g', -1, 32), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", errors.New("value must be a finite number")
		}
		return strconv.FormatFloat(typed, 'g', -1, 64), nil
	case json.Number:
		if _, err := typed.Float64(); err != nil {
			return "", fmt.Errorf("invalid number %q", typed.String())
		}
		return typed.String(), nil
	case nil:
		return "", errors.New("value is required; use an empty string to clear it")
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func opsArgumentError(operation, message string) *OpsError {
	return &OpsError{
		Exit:    ops.ExitInvalidArgument,
		Code:    ops.CodeInvalidArgument,
		Message: fmt.Sprintf("ops %s: %s", operation, message),
	}
}
