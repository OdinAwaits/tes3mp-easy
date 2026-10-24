// Package serverapi is the client for the public tes3mp-easy panel API.
//
// It only performs unauthenticated GET requests defined by
// docs/v2/03-contracts.md §7. Server passwords, admin sessions and mutation
// endpoints never reach this package.
package serverapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

const (
	// ServerPath is the public server information endpoint.
	ServerPath = "/api/v1/server"
	// ModsPath is the public current mods endpoint.
	ModsPath = "/api/v1/mods/current"
	// ModsArchivePath is the only archive URL accepted from Mods.URL.
	ModsArchivePath = "/api/v1/mods/current/archive"

	maxResponseSize int64 = 1 << 20
)

var (
	// ErrNoMods reports the documented 404 no_mods response.
	ErrNoMods = errors.New("current mods archive is unavailable")
	// ErrInvalidResponse reports a 200 response that is not the documented JSON.
	ErrInvalidResponse = errors.New("panel returned an invalid response")
)

// invalidResponse wraps ErrInvalidResponse while keeping the validation detail
// available in the log and diagnostic report. Callers use errors.Is instead of
// matching message text.
func invalidResponse(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidResponse, fmt.Sprintf(format, args...))
}

// Server is the public GET /api/v1/server response.
type Server struct {
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	Port         int     `json:"port"`
	Status       string  `json:"status"`
	ModsSHA256   *string `json:"mods_sha256"`
	PanelVersion string  `json:"panel_version"`
}

// Mods is the public GET /api/v1/mods/current response.
type Mods struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	URL    string `json:"url"`
}

// APIError is a structured public API failure. Message is the safe message from
// the panel response, never a password or an internal command line.
type APIError struct {
	Status  int
	Code    string
	Message string
}

// Error implements error.
func (e *APIError) Error() string {
	code := strings.TrimSpace(e.Code)
	if code == "" {
		code = "http_error"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = http.StatusText(e.Status)
	}
	return fmt.Sprintf("panel API: %s (HTTP %d): %s", code, e.Status, message)
}

// Is allows errors.Is(err, ErrNoMods).
func (e *APIError) Is(target error) bool {
	return target == ErrNoMods && e.Code == "no_mods"
}

// Options configure New. BaseURL is required; zero timeouts and attempt counts
// receive production defaults.
type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	Attempts   int
	RetryWait  time.Duration
}

// Client is a public panel API client. It is safe for concurrent use.
type Client struct {
	base      *url.URL
	http      *http.Client
	attempts  int
	retryWait time.Duration
}

// New validates the panel URL and creates a client. Public panels must use
// HTTPS; plain HTTP is accepted only for loopback development panels.
func New(options Options) (*Client, error) {
	base, err := parseBaseURL(options.BaseURL)
	if err != nil {
		return nil, err
	}
	attempts := options.Attempts
	if attempts == 0 {
		attempts = 3
	}
	if attempts < 1 {
		return nil, errors.New("server API attempts must be positive")
	}
	retryWait := options.RetryWait
	if retryWait == 0 {
		retryWait = time.Second
	}
	if retryWait < 0 {
		return nil, errors.New("server API retry delay must not be negative")
	}

	client := http.Client{Timeout: 15 * time.Minute}
	if options.HTTPClient != nil {
		client = *options.HTTPClient
	}
	previous := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := validateRequestURL(request.URL); err != nil {
			return err
		}
		if !strings.EqualFold(request.URL.Host, base.Host) {
			return errors.New("panel API: redirect must stay on the panel host")
		}
		if previous != nil {
			return previous(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}

	return &Client{
		base:      base,
		http:      &client,
		attempts:  attempts,
		retryWait: retryWait,
	}, nil
}

// BaseURL returns the canonical panel URL without a trailing slash.
func (c *Client) BaseURL() string {
	if c == nil || c.base == nil {
		return ""
	}
	return strings.TrimSuffix(c.base.String(), "/")
}

// Server fetches GET /api/v1/server.
func (c *Client) Server(ctx context.Context) (Server, error) {
	if c == nil {
		return Server{}, errors.New("server API client is nil")
	}
	var response Server
	if err := c.getJSON(ctx, ServerPath, &response); err != nil {
		return Server{}, err
	}
	if strings.TrimSpace(response.Name) == "" {
		return Server{}, invalidResponse("server response has an empty name")
	}
	if strings.TrimSpace(response.Address) == "" {
		return Server{}, invalidResponse("server response has an empty address")
	}
	if response.Port < 1 || response.Port > 65535 {
		return Server{}, invalidResponse(
			"server response has invalid port %d", response.Port)
	}
	if strings.TrimSpace(response.Status) == "" {
		return Server{}, invalidResponse("server response has an empty status")
	}
	if response.ModsSHA256 != nil {
		sha := strings.ToLower(strings.TrimSpace(*response.ModsSHA256))
		if !validSHA256(sha) {
			return Server{}, invalidResponse(
				"server response has an invalid mods_sha256")
		}
		response.ModsSHA256 = &sha
	}
	return response, nil
}

// Mods fetches GET /api/v1/mods/current and validates the archive metadata
// before the caller downloads anything.
func (c *Client) Mods(ctx context.Context) (Mods, error) {
	if c == nil {
		return Mods{}, errors.New("server API client is nil")
	}
	var response Mods
	if err := c.getJSON(ctx, ModsPath, &response); err != nil {
		return Mods{}, err
	}
	response.Name = strings.TrimSpace(response.Name)
	if err := archive.ValidateModsArchiveName(response.Name); err != nil {
		return Mods{}, invalidResponse("invalid mods name: %v", err)
	}
	response.SHA256 = strings.ToLower(strings.TrimSpace(response.SHA256))
	if !validSHA256(response.SHA256) {
		return Mods{}, invalidResponse("invalid mods sha256")
	}
	if response.Size < 1 || response.Size > archive.MaxCompressedSize {
		return Mods{}, invalidResponse(
			"mods size %d is outside 1..%d", response.Size, archive.MaxCompressedSize)
	}
	if _, err := c.ArchiveURL(response); err != nil {
		return Mods{}, err
	}
	return response, nil
}

// ArchiveURL validates and resolves Mods.URL. Only the fixed current archive
// path on the configured panel is accepted, so a compromised response cannot
// point the client at an arbitrary host.
func (c *Client) ArchiveURL(mods Mods) (*url.URL, error) {
	if c == nil || c.base == nil {
		return nil, errors.New("server API client is nil")
	}
	raw := strings.TrimSpace(mods.URL)
	if raw == "" {
		return nil, invalidResponse("mods archive URL is empty")
	}
	reference, err := url.Parse(raw)
	if err != nil {
		return nil, invalidResponse("invalid mods archive URL: %v", err)
	}
	if reference.User != nil || reference.Fragment != "" ||
		reference.RawQuery != "" {
		return nil, invalidResponse(
			"mods archive URL must not contain credentials, query or fragment")
	}
	resolved := c.base.ResolveReference(reference)
	if err := validateRequestURL(resolved); err != nil {
		return nil, invalidResponse("%v", err)
	}
	if !strings.EqualFold(resolved.Scheme, c.base.Scheme) ||
		!strings.EqualFold(resolved.Host, c.base.Host) {
		return nil, invalidResponse(
			"mods archive URL must stay on the panel host")
	}
	if resolved.EscapedPath() != ModsArchivePath {
		return nil, invalidResponse(
			"mods archive URL must be %s", ModsArchivePath)
	}
	return resolved, nil
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	requestURL := c.base.JoinPath(path)
	var last error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		err := c.getJSONOnce(ctx, requestURL, target)
		if err == nil {
			return nil
		}
		last = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retryable(err) || attempt == c.attempts {
			break
		}
		timer := time.NewTimer(c.retryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

func (c *Client) getJSONOnce(ctx context.Context, requestURL *url.URL,
	target any,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("panel API: create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "tes3mp-easy-client")

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("panel API: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return fmt.Errorf("panel API: read response: %w", err)
	}
	if int64(len(body)) > maxResponseSize {
		return invalidResponse("response is too large")
	}
	if response.StatusCode != http.StatusOK {
		return parseAPIError(response.StatusCode, body)
	}

	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid JSON: %v", ErrInvalidResponse, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data after JSON", ErrInvalidResponse)
	}
	return nil
}

func parseAPIError(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil &&
		envelope.Error.Code != "" {
		return &APIError{
			Status:  status,
			Code:    envelope.Error.Code,
			Message: envelope.Error.Message,
		}
	}
	return &APIError{Status: status}
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusRequestTimeout ||
			apiErr.Status == http.StatusTooManyRequests ||
			apiErr.Status >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func parseBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("panel URL is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("panel URL: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, errors.New("panel URL must use https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, errors.New("panel URL has no host")
	}
	if parsed.User != nil {
		return nil, errors.New("panel URL must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("panel URL must not contain a query or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, errors.New("panel URL must not contain a path")
	}
	if parsed.Scheme == "http" && !isLoopback(parsed.Hostname()) {
		return nil, errors.New("panel URL must use https outside localhost")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	return parsed, nil
}

func validateRequestURL(parsed *url.URL) error {
	if parsed == nil {
		return errors.New("panel API: nil URL")
	}
	if parsed.User != nil {
		return errors.New("panel API: URL must not contain credentials")
	}
	if parsed.Fragment != "" {
		return errors.New("panel API: URL must not contain a fragment")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		if isLoopback(parsed.Hostname()) {
			return nil
		}
		return errors.New("panel API: HTTP is allowed only for loopback")
	default:
		return fmt.Errorf("panel API: unsupported URL scheme %q", parsed.Scheme)
	}
}

func isLoopback(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		switch {
		case char >= '0' && char <= '9':
		case char >= 'a' && char <= 'f':
		default:
			return false
		}
	}
	return true
}
