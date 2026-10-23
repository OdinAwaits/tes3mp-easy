package panel

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestAuthStore(t *testing.T) (*authStore, string, Config) {
	t.Helper()
	cfg := testConfig(t)
	logs := new(bytes.Buffer)
	store, err := openAuthStore(cfg, log.New(logs, "", 0))
	if err != nil {
		t.Fatalf("openAuthStore: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.DataDir, InitialPasswordFileName))
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	password := strings.TrimSpace(string(data))
	if password == "" {
		t.Fatal("initial password is empty")
	}
	return store, password, cfg
}

func TestOpenAuthStoreCreatesInitialPassword(t *testing.T) {
	store, password, cfg := newTestAuthStore(t)

	if !store.mustChangePassword() {
		t.Fatal("mustChangePassword = false, want true")
	}
	if !store.verify(password) {
		t.Fatal("initial password does not verify")
	}
	if store.verify(password + "x") {
		t.Fatal("wrong password verified")
	}

	paths := pathsForAuth(cfg)
	for _, path := range []string{paths.record, paths.initial} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 600", path, got)
		}
	}

	authData, err := os.ReadFile(paths.record)
	if err != nil {
		t.Fatalf("read auth.json: %v", err)
	}
	if bytes.Contains(authData, []byte(password)) {
		t.Fatal("auth.json contains the plaintext password")
	}
	record, _, _, err := parseAuthRecord(authData)
	if err != nil {
		t.Fatalf("parse generated auth.json: %v", err)
	}
	if record.Version != authFileVersion || record.Algorithm != authAlgorithm {
		t.Fatalf("record = %+v", record)
	}
	if record.Params != defaultArgonParams() {
		t.Fatalf("params = %+v, want %+v", record.Params, defaultArgonParams())
	}

	reopened, err := openAuthStore(cfg, log.New(new(bytes.Buffer), "", 0))
	if err != nil {
		t.Fatalf("reopen auth store: %v", err)
	}
	if !reopened.mustChangePassword() {
		t.Fatal("reopened mustChangePassword = false, want true")
	}
	if !reopened.verify(password) {
		t.Fatal("reopened store does not verify the initial password")
	}
}

func TestOpenAuthStoreRejectsCorruptFile(t *testing.T) {
	cfg := testConfig(t)
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(cfg.DataDir, AuthFileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	if _, err := openAuthStore(cfg, log.New(new(bytes.Buffer), "", 0)); err == nil {
		t.Fatal("openAuthStore accepted a corrupt auth.json")
	}
}

func TestAuthStoreChangePassword(t *testing.T) {
	store, current, cfg := newTestAuthStore(t)
	next := "a brand new panel password"

	if err := store.changePassword("wrong current", next); !errors.Is(err, errCurrentPassword) {
		t.Fatalf("changePassword with wrong current = %v, want errCurrentPassword", err)
	}
	if !store.verify(current) {
		t.Fatal("wrong current password changed the stored hash")
	}
	if err := store.changePassword(current, "too short"); err == nil {
		t.Fatal("short password was accepted")
	}
	if err := store.changePassword(current, current); err == nil {
		t.Fatal("unchanged password was accepted")
	}
	if err := store.changePassword(current, next); err != nil {
		t.Fatalf("changePassword: %v", err)
	}

	if store.mustChangePassword() {
		t.Fatal("mustChangePassword = true after change")
	}
	if !store.verify(next) {
		t.Fatal("new password does not verify")
	}
	if store.verify(current) {
		t.Fatal("old password still verifies")
	}
	if exists, err := pathExists(filepath.Join(cfg.DataDir, InitialPasswordFileName)); err != nil {
		t.Fatalf("inspect initial password: %v", err)
	} else if exists {
		t.Fatal("initial-password.txt still exists after change")
	}

	reopened, err := openAuthStore(cfg, log.New(new(bytes.Buffer), "", 0))
	if err != nil {
		t.Fatalf("reopen auth store: %v", err)
	}
	if reopened.mustChangePassword() {
		t.Fatal("reopened mustChangePassword = true")
	}
	if !reopened.verify(next) {
		t.Fatal("reopened store does not verify the new password")
	}
	if reopened.verify(current) {
		t.Fatal("reopened store still verifies the old password")
	}
}

func TestResetPasswordOverwritesCorruptStore(t *testing.T) {
	_, oldPassword, cfg := newTestAuthStore(t)

	if err := os.WriteFile(filepath.Join(cfg.DataDir, AuthFileName),
		[]byte("corrupt on purpose"), 0o600); err != nil {
		t.Fatalf("corrupt auth.json: %v", err)
	}
	newPassword, err := ResetPassword(cfg)
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if newPassword == oldPassword {
		t.Fatal("ResetPassword reused the old password")
	}

	initial, err := os.ReadFile(filepath.Join(cfg.DataDir, InitialPasswordFileName))
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	if strings.TrimSpace(string(initial)) != newPassword {
		t.Fatalf("initial password = %q, want %q",
			strings.TrimSpace(string(initial)), newPassword)
	}
	reopened, err := openAuthStore(cfg, log.New(new(bytes.Buffer), "", 0))
	if err != nil {
		t.Fatalf("open reset store: %v", err)
	}
	if !reopened.mustChangePassword() {
		t.Fatal("mustChangePassword = false after reset")
	}
	if !reopened.verify(newPassword) {
		t.Fatal("new temporary password does not verify")
	}
	if reopened.verify(oldPassword) {
		t.Fatal("old password still verifies after reset")
	}
}

func TestValidateNewPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "short", password: "12345678901", wantErr: true},
		{name: "minimum ascii", password: "123456789012"},
		{name: "minimum unicode runes", password: strings.Repeat("ы", minPasswordRunes)},
		{name: "too many bytes", password: strings.Repeat("x", maxPasswordBytes+1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNewPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateNewPassword(%d bytes) = %v, wantErr %v",
					len(tt.password), err, tt.wantErr)
			}
		})
	}
}

func TestParseAuthRecordRejectsMalformedFiles(t *testing.T) {
	record, salt, hash, err := newAuthRecord("a valid initial password")
	if err != nil {
		t.Fatalf("newAuthRecord: %v", err)
	}
	valid, err := marshalAuthRecord(record)
	if err != nil {
		t.Fatalf("marshalAuthRecord: %v", err)
	}
	parsed, parsedSalt, parsedHash, err := parseAuthRecord(valid)
	if err != nil {
		t.Fatalf("parse valid record: %v", err)
	}
	if parsed.Algorithm != authAlgorithm || !bytes.Equal(parsedSalt, salt) ||
		!bytes.Equal(parsedHash, hash) {
		t.Fatal("parsed record does not match generated record")
	}

	base := recordMap(t, valid)
	if len(base) == 0 {
		t.Fatal("base record is empty")
	}
	mutate := func(change func(map[string]any)) []byte {
		copied := recordMap(t, valid)
		change(copied)
		return marshalMap(t, copied)
	}
	params := func(values map[string]any) map[string]any {
		return values["params"].(map[string]any)
	}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "empty", data: nil, want: "decode auth file"},
		{name: "invalid json", data: []byte("{"), want: "decode auth file"},
		{name: "unknown field", data: mutate(func(values map[string]any) {
			values["extra"] = true
		}), want: "unknown field"},
		{name: "bad version", data: mutate(func(values map[string]any) {
			values["version"] = 2
		}), want: "unsupported auth version"},
		{name: "bad algorithm", data: mutate(func(values map[string]any) {
			values["algorithm"] = "scrypt"
		}), want: "unsupported auth algorithm"},
		{name: "bad salt base64", data: mutate(func(values map[string]any) {
			values["salt"] = "***"
		}), want: "decode password salt"},
		{name: "bad salt length", data: mutate(func(values map[string]any) {
			values["salt"] = base64.StdEncoding.EncodeToString([]byte("short"))
		}), want: "password salt is"},
		{name: "bad hash length", data: mutate(func(values map[string]any) {
			values["hash"] = base64.StdEncoding.EncodeToString(make([]byte, 16))
		}), want: "password hash is"},
		{name: "bad time", data: mutate(func(values map[string]any) {
			params(values)["time"] = 0
		}), want: "invalid argon2 params"},
		{name: "bad memory", data: mutate(func(values map[string]any) {
			params(values)["memory"] = 64 * 1024
		}), want: "invalid argon2 params"},
		{name: "trailing value", data: append(append([]byte(nil), valid...), []byte("{}")...),
			want: "decode auth file"},
		{name: "too large", data: bytes.Repeat([]byte(" "), maxAuthFileBytes+1),
			want: "limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, _, err := parseAuthRecord(tt.data); err == nil ||
				!strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseAuthRecord error = %v, want %q", err, tt.want)
			}
		})
	}
}

func recordMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	return values
}

func marshalMap(t *testing.T, values map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	return data
}
