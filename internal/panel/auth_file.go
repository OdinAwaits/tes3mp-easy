package panel

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	// AuthFileName is the password hash inside the panel data directory.
	AuthFileName = "auth.json"

	// InitialPasswordFileName holds the temporary password until it is changed.
	InitialPasswordFileName = "initial-password.txt"

	authFileVersion = 1
	authAlgorithm   = "argon2id"

	argonTime    uint32 = 3
	argonMemory  uint32 = 32 * 1024 // KiB, not bytes
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32

	authSaltLength     = 16
	initialPasswordLen = 18 // 24 base64url characters
	minPasswordRunes   = 12
	maxPasswordBytes   = 1024
	maxAuthFileBytes   = 64 << 10
)

var errCurrentPassword = errors.New("current password is incorrect")

// argonParams mirrors the params object in auth.json. The documented values
// are deliberately small enough to fit one password hash into the panel's
// 64 MiB cgroup together with the Go runtime.
type argonParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"key_len"`
}

func defaultArgonParams() argonParams {
	return argonParams{
		Time:    argonTime,
		Memory:  argonMemory,
		Threads: argonThreads,
		KeyLen:  argonKeyLen,
	}
}

func (p argonParams) validate() error {
	switch {
	case p.Time < 1 || p.Time > 10:
		return fmt.Errorf("time %d is outside 1..10", p.Time)
	case p.Memory < 8*1024 || p.Memory > argonMemory:
		return fmt.Errorf("memory %d is outside 8192..%d KiB", p.Memory, argonMemory)
	case p.Threads < 1 || p.Threads > 4:
		return fmt.Errorf("threads %d is outside 1..4", p.Threads)
	case p.KeyLen < 16 || p.KeyLen > 64:
		return fmt.Errorf("key_len %d is outside 16..64", p.KeyLen)
	case p.Memory < 8*uint32(p.Threads):
		return errors.New("memory is too small for the thread count")
	}
	return nil
}

type authRecord struct {
	Version   int         `json:"version"`
	Algorithm string      `json:"algorithm"`
	Salt      string      `json:"salt"`
	Hash      string      `json:"hash"`
	Params    argonParams `json:"params"`
}

func newAuthRecord(password string) (authRecord, []byte, []byte, error) {
	salt := make([]byte, authSaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return authRecord{}, nil, nil, fmt.Errorf("generate password salt: %w", err)
	}
	params := defaultArgonParams()
	hash := hashPassword(password, salt, params)

	record := authRecord{
		Version:   authFileVersion,
		Algorithm: authAlgorithm,
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Hash:      base64.StdEncoding.EncodeToString(hash),
		Params:    params,
	}
	return record, salt, hash, nil
}

func hashPassword(password string, salt []byte, params argonParams) []byte {
	return argon2.IDKey([]byte(password), salt, params.Time, params.Memory,
		params.Threads, params.KeyLen)
}

func parseAuthRecord(data []byte) (authRecord, []byte, []byte, error) {
	if len(data) > maxAuthFileBytes {
		return authRecord{}, nil, nil, fmt.Errorf(
			"auth file is %d bytes, limit is %d", len(data), maxAuthFileBytes)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record authRecord
	if err := decoder.Decode(&record); err != nil {
		return authRecord{}, nil, nil, fmt.Errorf("decode auth file: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return authRecord{}, nil, nil, fmt.Errorf("decode auth file: %w", err)
	}

	switch {
	case record.Version != authFileVersion:
		return authRecord{}, nil, nil, fmt.Errorf(
			"unsupported auth version %d", record.Version)
	case record.Algorithm != authAlgorithm:
		return authRecord{}, nil, nil, fmt.Errorf(
			"unsupported auth algorithm %q", record.Algorithm)
	}
	if err := record.Params.validate(); err != nil {
		return authRecord{}, nil, nil, fmt.Errorf("invalid argon2 params: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(record.Salt)
	if err != nil {
		return authRecord{}, nil, nil, fmt.Errorf("decode password salt: %w", err)
	}
	if len(salt) != authSaltLength {
		return authRecord{}, nil, nil, fmt.Errorf(
			"password salt is %d bytes, want %d", len(salt), authSaltLength)
	}
	hash, err := base64.StdEncoding.DecodeString(record.Hash)
	if err != nil {
		return authRecord{}, nil, nil, fmt.Errorf("decode password hash: %w", err)
	}
	if len(hash) != int(record.Params.KeyLen) {
		return authRecord{}, nil, nil, fmt.Errorf(
			"password hash is %d bytes, key_len is %d", len(hash), record.Params.KeyLen)
	}
	return record, salt, hash, nil
}

func marshalAuthRecord(record authRecord) ([]byte, error) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode auth file: %w", err)
	}
	return append(data, '\n'), nil
}

func readAuthFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxAuthFileBytes {
		return nil, fmt.Errorf("auth file is %d bytes, limit is %d",
			info.Size(), maxAuthFileBytes)
	}
	return os.ReadFile(path)
}

func generateInitialPassword() (string, error) {
	raw := make([]byte, initialPasswordLen)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", fmt.Errorf("generate initial password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func validateNewPassword(password string) error {
	switch {
	case utf8.RuneCountInString(password) < minPasswordRunes:
		return fmt.Errorf("password must be at least %d characters", minPasswordRunes)
	case len(password) > maxPasswordBytes:
		return fmt.Errorf("password must be at most %d bytes", maxPasswordBytes)
	}
	return nil
}
