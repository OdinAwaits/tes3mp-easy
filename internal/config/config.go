// Package config persists the client settings and server profiles.
//
// The client keeps one configuration file in its per-user config directory.
// The file contains server passwords and is always written with mode 0600. The
// package deliberately has no knowledge of Fyne so profile handling can be
// tested without a display.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

const (
	// CurrentVersion is the configuration schema version written by the client.
	CurrentVersion = 1

	// FileMode keeps server passwords private to the current user.
	FileMode os.FileMode = 0o600

	// DirMode is used for client-owned directories.
	DirMode os.FileMode = 0o700

	// DefaultServerPort is the TES3MP port when an address omits one.
	DefaultServerPort = 25565
)

// Theme values persisted in Config.Theme.
const (
	ThemeSystem = "system"
	ThemeDark   = "dark"
	ThemeLight  = "light"
)

// Server is one playable server profile.
type Server struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	PanelURL string `json:"panel_url"`
}

// Config is the complete client configuration.
type Config struct {
	Version       int    `json:"version"`
	Language      string `json:"language,omitempty"`
	Theme         string `json:"theme,omitempty"`
	MorrowindPath string `json:"morrowind_path,omitempty"`
	WorkDir       string `json:"work_dir,omitempty"`
	ProtonPath    string `json:"proton_path,omitempty"`
	// FontSet selects one of the fixed font packs; empty means the default.
	FontSet string `json:"font_set,omitempty"`
	// Voiceover remembers whether the next localization install fetches the
	// optional Russian voice archive. Nil means the client default (enabled);
	// an explicit false is stored so the choice survives a restart.
	Voiceover *bool `json:"voiceover,omitempty"`

	SetupComplete bool     `json:"setup_complete,omitempty"`
	ActiveServer  string   `json:"active_server,omitempty"`
	Servers       []Server `json:"servers"`
}

// Default returns an empty configuration for a first launch. The launcher UI
// ships in English; a user can still keep an explicit Russian setting.
func Default() Config {
	return Config{
		Version:  CurrentVersion,
		Language: string(i18n.English),
		Theme:    ThemeDark,
		Servers:  []Server{},
	}
}

// Normalize trims user input, canonicalizes derived fields and fills defaults.
// Invalid values are left in place so Validate can report them.
func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	if c.Servers == nil {
		c.Servers = []Server{}
	}
	c.Language = strings.TrimSpace(c.Language)
	if c.Language == "" {
		c.Language = string(i18n.English)
	}
	if language, ok := i18n.Parse(c.Language); ok {
		c.Language = string(language)
	}
	c.Theme = strings.ToLower(strings.TrimSpace(c.Theme))
	if c.Theme == "" {
		c.Theme = ThemeDark
	}
	c.MorrowindPath = strings.TrimSpace(c.MorrowindPath)
	c.WorkDir = strings.TrimSpace(c.WorkDir)
	c.ProtonPath = strings.TrimSpace(c.ProtonPath)
	c.FontSet = strings.ToLower(strings.TrimSpace(c.FontSet))
	c.ActiveServer = strings.TrimSpace(c.ActiveServer)
	for index := range c.Servers {
		c.Servers[index].Normalize()
	}
	if c.ActiveServer != "" && !c.hasServer(c.ActiveServer) {
		c.ActiveServer = ""
	}
}

// Validate checks a normalized configuration. Error messages never contain
// passwords or full server records.
func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Language != "" {
		if _, ok := i18n.Parse(c.Language); !ok {
			return fmt.Errorf("unsupported language %q", c.Language)
		}
	}
	switch c.Theme {
	case ThemeSystem, ThemeDark, ThemeLight:
	default:
		return fmt.Errorf("unsupported theme %q", c.Theme)
	}

	seen := make(map[string]struct{}, len(c.Servers))
	for index, server := range c.Servers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("server %d: %w", index+1, err)
		}
		if _, exists := seen[server.ID]; exists {
			return fmt.Errorf("server %d: duplicate id %q", index+1, server.ID)
		}
		seen[server.ID] = struct{}{}
	}
	if c.ActiveServer != "" {
		if _, exists := seen[c.ActiveServer]; !exists {
			return fmt.Errorf("active server %q does not exist", c.ActiveServer)
		}
	}
	return nil
}

// Clone returns an independent copy safe to hand to the UI.
func (c Config) Clone() Config {
	clone := c
	clone.Servers = append([]Server(nil), c.Servers...)
	if c.Voiceover != nil {
		voiceover := *c.Voiceover
		clone.Voiceover = &voiceover
	}
	return clone
}

// VoiceoverEnabled reports whether a localization install should fetch the
// optional voice archive. A configuration without the field predates the
// option, when every install included the voices, so enabled is the default.
func (c Config) VoiceoverEnabled() bool {
	return c.Voiceover == nil || *c.Voiceover
}

// SetVoiceover stores an explicit voiceover preference.
func (c *Config) SetVoiceover(enabled bool) {
	value := enabled
	c.Voiceover = &value
}

func (c Config) hasServer(id string) bool {
	for _, server := range c.Servers {
		if server.ID == id {
			return true
		}
	}
	return false
}

// Normalize trims user input and rebuilds the identity derived from address and
// port. The ID is not user-editable. PanelURL is optional; when absent the
// public panel is assumed to live on the game host.
func (s *Server) Normalize() {
	s.Name = strings.TrimSpace(s.Name)
	s.Address = NormalizeAddress(s.Address)
	s.PanelURL = strings.TrimSpace(s.PanelURL)
	if s.Address != "" && s.Port != 0 {
		s.ID = ServerID(s.Address, s.Port)
	} else {
		s.ID = ""
	}
	if s.PanelURL != "" {
		if normalized, err := NormalizePanelURL(s.PanelURL); err == nil {
			s.PanelURL = normalized
		}
	}
}

// EffectivePanelURL returns the explicit panel URL or derives one from the game
// address. Loopback panels use HTTP because local development panels normally
// do not have a certificate.
func (s Server) EffectivePanelURL() string {
	if value := strings.TrimSpace(s.PanelURL); value != "" {
		return value
	}
	host := NormalizeAddress(s.Address)
	if host == "" {
		return ""
	}
	scheme := "https"
	if isLoopbackHost(host) {
		scheme = "http"
	}
	return scheme + "://" + panelHost(host)
}

// Validate checks one profile. It intentionally reports field names, not
// values, so a server password can never leak into an error dialog or log.
func (s Server) Validate() error {
	if s.Name == "" {
		return errors.New("name is empty")
	}
	if s.Address == "" {
		return errors.New("address is empty")
	}
	if err := ValidateAddress(s.Address); err != nil {
		return fmt.Errorf("address: %w", err)
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port %d is outside 1..65535", s.Port)
	}
	wantID := ServerID(s.Address, s.Port)
	if s.ID == "" || s.ID != wantID {
		return errors.New("id does not match address and port")
	}
	if s.PanelURL != "" {
		if _, err := NormalizePanelURL(s.PanelURL); err != nil {
			return fmt.Errorf("panel URL: %w", err)
		}
	}
	return nil
}

// ServerID derives the stable directory name used for one profile.
func ServerID(address string, port int) string {
	if address == "" || port < 1 || port > 65535 {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(address))
	var builder strings.Builder
	lastUnderscore := false
	for _, char := range host {
		allowed := char >= 'a' && char <= 'z' ||
			char >= '0' && char <= '9' ||
			char == '.' || char == '-'
		if allowed {
			builder.WriteRune(char)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	base := strings.Trim(builder.String(), "_")
	if base == "" {
		return ""
	}
	return base + "_" + strconv.Itoa(port)
}

// NormalizeAddress trims a host and removes one pair of IPv6 brackets. The
// persisted address is always the bare host; brackets are re-added where a URL
// needs them.
func NormalizeAddress(address string) string {
	address = strings.TrimSpace(address)
	if strings.HasPrefix(address, "[") && strings.HasSuffix(address, "]") {
		inner := address[1 : len(address)-1]
		if net.ParseIP(inner) != nil {
			return inner
		}
	}
	return address
}

// panelHost brackets IPv6 literals for use in a URL authority.
func panelHost(host string) string {
	if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// ServerEndpoint is the result of parsing the single add-server field.
type ServerEndpoint struct {
	Address  string
	Port     int
	PanelURL string
}

// ParseServerEndpoint accepts the forms users paste from chat or a browser:
// host, host:port, [IPv6]:port, tes3mp://host:port and https://host[:port].
// A web URL is treated as the panel endpoint and keeps the default game port.
func ParseServerEndpoint(raw string) (ServerEndpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ServerEndpoint{}, errors.New("server address is empty")
	}
	if strings.ContainsAny(raw, "\r\n\x00") {
		return ServerEndpoint{}, errors.New("server address contains a line break or NUL")
	}
	endpoint := ServerEndpoint{Port: DefaultServerPort}
	body := raw

	if index := strings.Index(raw, "://"); index >= 0 {
		scheme := strings.ToLower(strings.TrimSpace(raw[:index]))
		body = raw[index+3:]
		switch scheme {
		case "tes3mp":
			// Game endpoint with an explicit protocol prefix.
		case "http", "https":
			parsed, err := url.Parse(raw)
			if err != nil {
				return ServerEndpoint{}, fmt.Errorf("server address: %w", err)
			}
			if parsed.User != nil {
				return ServerEndpoint{}, errors.New("server address must not contain credentials")
			}
			if parsed.RawQuery != "" || parsed.Fragment != "" {
				return ServerEndpoint{}, errors.New("server address must not contain a query or fragment")
			}
			if parsed.Path != "" && parsed.Path != "/" {
				return ServerEndpoint{}, errors.New("server address must not contain a path")
			}
			if parsed.Hostname() == "" {
				return ServerEndpoint{}, errors.New("server address has no host")
			}
			panel, err := NormalizePanelURL(scheme + "://" + parsed.Host)
			if err != nil {
				return ServerEndpoint{}, fmt.Errorf("server address: %w", err)
			}
			endpoint.Address = NormalizeAddress(parsed.Hostname())
			endpoint.PanelURL = panel
			if err := ValidateAddress(endpoint.Address); err != nil {
				return ServerEndpoint{}, fmt.Errorf("server address: %w", err)
			}
			return endpoint, nil
		default:
			return ServerEndpoint{}, fmt.Errorf("unsupported server address scheme %q", scheme)
		}
	}

	host, port, err := splitEndpoint(body)
	if err != nil {
		return ServerEndpoint{}, err
	}
	endpoint.Address = NormalizeAddress(host)
	endpoint.Port = port
	if err := ValidateAddress(endpoint.Address); err != nil {
		return ServerEndpoint{}, fmt.Errorf("server address: %w", err)
	}
	server := Server{Address: endpoint.Address, Port: endpoint.Port}
	endpoint.PanelURL = server.EffectivePanelURL()
	return endpoint, nil
}

func splitEndpoint(body string) (string, int, error) {
	if body == "" {
		return "", 0, errors.New("server address has no host")
	}
	if strings.HasPrefix(body, "[") {
		end := strings.IndexByte(body, ']')
		if end < 0 {
			return "", 0, errors.New("IPv6 address is missing its closing bracket")
		}
		host := body[1:end]
		if net.ParseIP(host) == nil || !strings.Contains(host, ":") {
			return "", 0, errors.New("brackets must contain an IPv6 address")
		}
		rest := body[end+1:]
		switch {
		case rest == "":
			return host, DefaultServerPort, nil
		case strings.HasPrefix(rest, ":"):
			port, err := parseEndpointPort(rest[1:])
			return host, port, err
		default:
			return "", 0, errors.New("unexpected text after the IPv6 address")
		}
	}
	if strings.Count(body, ":") > 1 {
		if net.ParseIP(body) == nil {
			return "", 0, errors.New("invalid IPv6 address")
		}
		return body, DefaultServerPort, nil
	}
	if index := strings.LastIndexByte(body, ':'); index >= 0 {
		host := body[:index]
		if host == "" {
			return "", 0, errors.New("server address has no host")
		}
		port, err := parseEndpointPort(body[index+1:])
		return host, port, err
	}
	return body, DefaultServerPort, nil
}

func parseEndpointPort(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("server port must be a number between 1 and 65535")
	}
	return port, nil
}

// ValidateAddress accepts an IP address or a DNS host name. The port is a
// separate field, so "host:port" is rejected instead of silently parsed.
func ValidateAddress(address string) error {
	address = NormalizeAddress(address)
	if address == "" {
		return errors.New("is empty")
	}
	if strings.IndexFunc(address, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) >= 0 {
		return errors.New("must not contain whitespace or control characters")
	}
	if strings.Contains(address, "://") {
		return errors.New("must not contain a URL scheme")
	}
	if strings.ContainsAny(address, "/\\?#@") {
		return errors.New("must not contain a path, query or credentials")
	}
	if strings.HasPrefix(address, "[") || strings.HasSuffix(address, "]") {
		return errors.New("IPv6 brackets are not allowed in the address field")
	}
	if net.ParseIP(address) != nil {
		return nil
	}
	if strings.Contains(address, ":") {
		return errors.New("must not contain a port")
	}
	if len(address) > 253 {
		return errors.New("is too long")
	}
	for _, label := range strings.Split(address, ".") {
		switch {
		case label == "":
			return errors.New("contains an empty DNS label")
		case len(label) > 63:
			return errors.New("contains a DNS label longer than 63 characters")
		case strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-"):
			return errors.New("contains a DNS label starting or ending with '-'")
		}
	}
	return nil
}

// NormalizePanelURL validates and canonicalizes the panel base URL. HTTPS is
// required; plain HTTP is accepted only for loopback development panels.
func NormalizePanelURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("is invalid: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return "", errors.New("must use https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return "", errors.New("has no host")
	}
	if parsed.User != nil {
		return "", errors.New("must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("must not contain a query or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("must not contain a path")
	}
	if scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return "", errors.New("must use https outside localhost")
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = ""
	parsed.RawPath = ""
	return strings.TrimSuffix(parsed.String(), "/"), nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Load reads path. A missing file returns Default and is not an error.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is empty")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Config{}, fmt.Errorf("parse config %s: trailing data", path)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// Save atomically replaces path with cfg. Parent directories are created with
// mode 0700 and the file is always 0600.
func Save(path string, cfg Config) error {
	if path == "" {
		return errors.New("config path is empty")
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), DirMode); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := atomicfile.WriteFile(path, data, FileMode); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
