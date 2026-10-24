package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// ClientConfigName is the TES3MP client settings file read on every start.
const ClientConfigName = "tes3mp-client-default.cfg"

// ClientConfigOptions describes the game endpoint written into the TES3MP
// client settings. Password is never included in an error or log message.
type ClientConfigOptions struct {
	TES3MPDir string
	Address   string
	Port      int
	Password  string
	Now       func() time.Time
}

// ClientConfigResult describes the TES3MP client settings replacement.
type ClientConfigResult struct {
	Path    string
	Backup  string
	Created bool
	Address string
	Port    int
}

// PrepareClientConfig writes destinationAddress, port and password into
// tes3mp-client-default.cfg. Comments and all unrelated keys are preserved,
// line endings are kept, and an existing file is backed up before the atomic
// replacement.
func PrepareClientConfig(options ClientConfigOptions) (ClientConfigResult, error) {
	if strings.TrimSpace(options.TES3MPDir) == "" {
		return ClientConfigResult{}, errors.New("TES3MP directory is empty")
	}
	dir, err := filepath.Abs(options.TES3MPDir)
	if err != nil {
		return ClientConfigResult{}, fmt.Errorf("TES3MP directory %q: %w",
			options.TES3MPDir, err)
	}
	address := strings.TrimSpace(options.Address)
	if address == "" {
		return ClientConfigResult{}, errors.New("server address is empty")
	}
	if strings.ContainsAny(address, "\r\n\x00") {
		return ClientConfigResult{}, errors.New("server address contains a line break or NUL")
	}
	if options.Port < 1 || options.Port > 65535 {
		return ClientConfigResult{}, fmt.Errorf("server port %d is outside 1..65535",
			options.Port)
	}
	if strings.ContainsAny(options.Password, "\r\n\x00") {
		return ClientConfigResult{}, errors.New("server password contains a line break or NUL")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ClientConfigResult{}, fmt.Errorf("create TES3MP directory: %w", err)
	}

	path := filepath.Join(dir, ClientConfigName)
	result := ClientConfigResult{Path: path, Address: address, Port: options.Port}
	existing, err := readClientConfig(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		existing = nil
	case err != nil:
		return ClientConfigResult{}, err
	}
	updated, err := updateClientConfig(existing, address, options.Port, options.Password)
	if err != nil {
		return ClientConfigResult{}, err
	}
	if existing != nil && bytes.Equal(existing, updated) {
		return result, nil
	}
	if existing != nil {
		now := time.Now
		if options.Now != nil {
			now = options.Now
		}
		backup, err := writeClientConfigBackup(path, existing, now())
		if err != nil {
			return ClientConfigResult{}, err
		}
		result.Backup = backup
	}
	if err := atomicfile.WriteFile(path, updated, 0o600); err != nil {
		return ClientConfigResult{}, fmt.Errorf("write %s: %w", ClientConfigName, err)
	}
	result.Created = true
	return result, nil
}

func readClientConfig(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	const maxSize = 4 << 20
	if info.Size() > maxSize {
		return nil, fmt.Errorf("%s is too large", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ClientConfigName, err)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("%s contains NUL", ClientConfigName)
	}
	return data, nil
}

func updateClientConfig(data []byte, address string, port int, password string) ([]byte, error) {
	lineEnding := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		lineEnding = "\r\n"
	}
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	trailingNewline := normalized == "" || strings.HasSuffix(normalized, "\n")
	lines := strings.Split(normalized, "\n")
	if trailingNewline && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{"[General]"}
		trailingNewline = true
	}

	generalStart := -1
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "general" {
				generalStart = index
				break
			}
		}
	}

	found := map[string]bool{
		"destinationaddress": false,
		"port":               false,
		"password":           false,
	}
	if generalStart >= 0 {
		for index := generalStart + 1; index < len(lines); index++ {
			line := strings.TrimSpace(lines[index])
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				break
			}
			key, _, ok := splitConfigLine(line)
			if !ok {
				continue
			}
			switch key {
			case "destinationaddress":
				lines[index] = "destinationAddress = " + address
				found[key] = true
			case "port":
				lines[index] = "port = " + strconv.Itoa(port)
				found[key] = true
			case "password":
				lines[index] = "password = " + password
				found[key] = true
			}
		}
		insertAt := generalStart + 1
		for _, key := range []string{"destinationAddress", "port", "password"} {
			if found[strings.ToLower(key)] {
				continue
			}
			value := address
			switch key {
			case "port":
				value = strconv.Itoa(port)
			case "password":
				value = password
			}
			lines = append(lines, "")
			copy(lines[insertAt+1:], lines[insertAt:])
			lines[insertAt] = key + " = " + value
			insertAt++
		}
	} else {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines,
			"[General]",
			"destinationAddress = "+address,
			"port = "+strconv.Itoa(port),
			"password = "+password,
		)
	}

	updated := strings.Join(lines, "\n")
	if trailingNewline {
		updated += "\n"
	}
	if lineEnding == "\r\n" {
		updated = strings.ReplaceAll(updated, "\n", "\r\n")
	}
	return []byte(updated), nil
}

func splitConfigLine(line string) (string, string, bool) {
	index := strings.IndexByte(line, '=')
	if index < 0 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(line[:index]))
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(line[index+1:]), true
}

func writeClientConfigBackup(path string, data []byte, now time.Time) (string, error) {
	base := path + ".bak-" + now.UTC().Format("20060102-150405")
	for suffix := 0; suffix < 100; suffix++ {
		candidate := base
		if suffix > 0 {
			candidate = fmt.Sprintf("%s-%d", base, suffix+1)
		}
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			if err := atomicfile.WriteFile(candidate, data, 0o600); err != nil {
				return "", fmt.Errorf("write TES3MP client config backup: %w", err)
			}
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect TES3MP client config backup: %w", err)
		}
	}
	return "", errors.New("cannot choose a unique TES3MP client config backup name")
}
