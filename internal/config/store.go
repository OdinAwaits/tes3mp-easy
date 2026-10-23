package config

import (
	"errors"
	"fmt"
	"sync"
)

// Store serializes access to the on-disk configuration. Update persists a
// candidate first and only then makes it visible to Snapshot readers, so a
// failed write never changes the running client configuration.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// OpenStore loads path, creating an in-memory default when the file is absent.
func OpenStore(path string) (*Store, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Store{path: path, cfg: cfg}, nil
}

// Path returns the configuration file path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Snapshot returns an independent copy of the current configuration.
func (s *Store) Snapshot() Config {
	if s == nil {
		return Default()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

// Update applies change to a copy, validates it and atomically saves it. The
// current configuration is untouched when change or Save returns an error.
func (s *Store) Update(change func(*Config) error) error {
	if s == nil {
		return errors.New("config store is nil")
	}
	if change == nil {
		return errors.New("config update is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	candidate := s.cfg.Clone()
	if err := change(&candidate); err != nil {
		return err
	}
	candidate.Normalize()
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := Save(s.path, candidate); err != nil {
		return err
	}
	s.cfg = candidate
	return nil
}
