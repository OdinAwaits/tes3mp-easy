package panel

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// releaseHashMemory hands the 32 MiB Argon2 working buffer back to the kernel.
// Go keeps freed spans in its own heap, so without this call the panel's RSS
// stays at ~37 MiB after the first password check even though only ~5 MiB is
// live. Measured cost is 0.2–0.4 ms (full GC of a tiny live set plus madvise),
// i.e. noise next to the ~170 ms Argon2 work, and it is what keeps the panel
// inside its 64 MiB cgroup. It is a variable so tests can observe the call.
var releaseHashMemory = debug.FreeOSMemory

// authStore owns the password hash and the temporary-password marker. Argon2
// work is serialized through hashSem and the 32 MiB buffer is released to the
// OS after every hash: one hash at a time plus an immediate release keeps the
// panel at a few MiB when idle and around 40 MiB only while a login runs.
type authStore struct {
	mu         sync.RWMutex
	params     argonParams
	salt       []byte
	hash       []byte
	mustChange bool

	authPath    string
	initialPath string
	logger      *log.Logger

	hashSem  chan struct{}
	changeMu sync.Mutex
}

type authPaths struct {
	record  string
	initial string
}

func pathsForAuth(cfg Config) authPaths {
	return authPaths{
		record:  filepath.Join(cfg.DataDir, AuthFileName),
		initial: filepath.Join(cfg.DataDir, InitialPasswordFileName),
	}
}

// openAuthStore loads auth.json or creates a fresh initial password. A corrupt
// file is an error: replacing it silently would hide a broken deployment.
func openAuthStore(cfg Config, logger *log.Logger) (*authStore, error) {
	cfg = cfg.Normalize()
	if cfg.DataDir == "" {
		return nil, errors.New("panel data directory is empty")
	}
	if logger == nil {
		logger = log.New(os.Stderr, "", 0)
	}
	paths := pathsForAuth(cfg)

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create panel data directory %s: %w", cfg.DataDir, err)
	}

	store := &authStore{
		authPath:    paths.record,
		initialPath: paths.initial,
		logger:      logger,
		hashSem:     make(chan struct{}, 1),
	}

	data, err := readAuthFile(paths.record)
	switch {
	case err == nil:
		record, salt, hash, parseErr := parseAuthRecord(data)
		if parseErr != nil {
			return nil, fmt.Errorf("load %s: %w", paths.record, parseErr)
		}
		mustChange, statErr := pathExists(paths.initial)
		if statErr != nil {
			return nil, fmt.Errorf("inspect %s: %w", paths.initial, statErr)
		}
		store.params = record.Params
		store.salt = salt
		store.hash = hash
		store.mustChange = mustChange
		logger.Printf("auth: loaded password hash from %s", paths.record)
		return store, nil

	case errors.Is(err, fs.ErrNotExist):
		password, genErr := generateInitialPassword()
		if genErr != nil {
			return nil, genErr
		}
		record, salt, hash, genErr := newAuthRecord(password)
		if genErr != nil {
			return nil, genErr
		}
		releaseHashMemory()
		authData, genErr := marshalAuthRecord(record)
		if genErr != nil {
			return nil, genErr
		}
		if genErr := atomicfile.WriteFile(paths.record, authData, 0o600); genErr != nil {
			return nil, fmt.Errorf("write %s: %w", paths.record, genErr)
		}
		if genErr := writeInitialPassword(paths.initial, password); genErr != nil {
			removeErr := os.Remove(paths.record)
			return nil, errors.Join(genErr, removeErr)
		}

		store.params = record.Params
		store.salt = salt
		store.hash = hash
		store.mustChange = true
		logger.Printf("auth: initial password %s written to %s; change it after first login",
			password, paths.initial)
		return store, nil

	default:
		return nil, fmt.Errorf("read %s: %w", paths.record, err)
	}
}

// ResetPassword replaces auth.json and creates a new initial password. It
// ignores the old hash on purpose so a corrupt auth.json can be recovered.
func ResetPassword(cfg Config) (string, error) {
	cfg = cfg.Normalize()
	if cfg.DataDir == "" {
		return "", errors.New("panel data directory is empty")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return "", fmt.Errorf("create panel data directory %s: %w", cfg.DataDir, err)
	}
	paths := pathsForAuth(cfg)

	oldAuth, err := readOptionalFile(paths.record)
	if err != nil {
		return "", err
	}
	oldInitial, err := readOptionalFile(paths.initial)
	if err != nil {
		return "", err
	}

	password, err := generateInitialPassword()
	if err != nil {
		return "", err
	}
	record, _, _, err := newAuthRecord(password)
	if err != nil {
		return "", err
	}
	releaseHashMemory()
	authData, err := marshalAuthRecord(record)
	if err != nil {
		return "", err
	}

	if err := atomicfile.WriteFile(paths.record, authData, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", paths.record, err)
	}
	if err := writeInitialPassword(paths.initial, password); err != nil {
		rollbackAuth := restoreOptionalFile(paths.record, oldAuth)
		rollbackInitial := restoreOptionalFile(paths.initial, oldInitial)
		return "", errors.Join(err, rollbackAuth, rollbackInitial)
	}
	return password, nil
}

func readOptionalFile(path string) ([]byte, error) {
	data, err := readAuthFile(path)
	switch {
	case err == nil:
		return data, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	default:
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
}

func restoreOptionalFile(path string, data []byte) error {
	if data == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}
	if err := atomicfile.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("restore %s: %w", path, err)
	}
	return nil
}

func writeInitialPassword(path, password string) error {
	if err := atomicfile.WriteFile(path, []byte(password+"\n"), 0o600); err != nil {
		return fmt.Errorf("write initial password %s: %w", path, err)
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func (a *authStore) verify(password string) bool {
	a.mu.RLock()
	params := a.params
	salt := a.salt
	hash := a.hash
	a.mu.RUnlock()

	var ok bool
	a.hashLocked(func() {
		derived := hashPassword(password, salt, params)
		defer func() {
			for i := range derived {
				derived[i] = 0
			}
		}()
		ok = subtle.ConstantTimeCompare(derived, hash) == 1
	})
	return ok
}

// hashLocked runs one Argon2 operation under hashSem and releases the working
// buffer after the semaphore is dropped, so goroutines waiting for their turn
// never wait for the GC as well.
func (a *authStore) hashLocked(fn func()) {
	func() {
		a.hashSem <- struct{}{}
		defer func() { <-a.hashSem }()
		fn()
	}()
	releaseHashMemory()
}

func (a *authStore) mustChangePassword() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mustChange
}

// hashWithSemaphore creates a new record while holding the same semaphore as
// verify, so a login and a password change cannot allocate two 32 MiB Argon2
// buffers at once.
func (a *authStore) hashWithSemaphore(password string) (authRecord, []byte, []byte, error) {
	var (
		record     authRecord
		salt, hash []byte
		err        error
	)
	a.hashLocked(func() { record, salt, hash, err = newAuthRecord(password) })
	return record, salt, hash, err
}

// changePassword verifies the current password, writes a new hash and removes
// the initial-password marker. A failure to remove the marker is logged but
// does not fail the change: the new hash is already active.
func (a *authStore) changePassword(current, next string) error {
	a.changeMu.Lock()
	defer a.changeMu.Unlock()

	if err := validateNewPassword(next); err != nil {
		return err
	}
	if current == next {
		return errors.New("new password must differ from the current password")
	}
	if !a.verify(current) {
		return errCurrentPassword
	}

	record, salt, hash, err := a.hashWithSemaphore(next)
	if err != nil {
		return err
	}
	data, err := marshalAuthRecord(record)
	if err != nil {
		return err
	}
	if err := atomicfile.WriteFile(a.authPath, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", a.authPath, err)
	}

	a.mu.Lock()
	a.params = record.Params
	a.salt = salt
	a.hash = hash
	a.mustChange = false
	a.mu.Unlock()

	if err := os.Remove(a.initialPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		a.logger.Printf("auth: remove %s: %v", a.initialPath, err)
	}
	return nil
}
