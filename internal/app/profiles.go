package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/config"
)

// SaveServer creates a profile or replaces originalID with updated values.
// Changing address or port changes the derived ID and therefore the profile
// directory; the caller must treat the returned ID as authoritative.
func (s *Service) SaveServer(originalID string, profile config.Server) (string, error) {
	if s == nil {
		return "", errors.New("app service is nil")
	}
	profile.Normalize()
	if err := profile.Validate(); err != nil {
		return "", fmt.Errorf("profile: %w", err)
	}
	originalID = strings.TrimSpace(originalID)
	desiredID := profile.ID

	err := s.store.Update(func(cfg *config.Config) error {
		index := -1
		for current, existing := range cfg.Servers {
			if originalID != "" && existing.ID == originalID {
				index = current
			}
		}
		if originalID != "" && index < 0 {
			return fmt.Errorf("profile %q not found", originalID)
		}
		for _, existing := range cfg.Servers {
			if existing.ID == desiredID && existing.ID != originalID {
				return fmt.Errorf("profile %q already exists", desiredID)
			}
		}
		if index >= 0 {
			cfg.Servers[index] = profile
		} else {
			cfg.Servers = append(cfg.Servers, profile)
		}
		if cfg.ActiveServer == "" {
			cfg.ActiveServer = desiredID
		} else if originalID != "" && cfg.ActiveServer == originalID {
			cfg.ActiveServer = desiredID
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return desiredID, nil
}

// DeleteServer removes a profile and clears the active selection if needed.
func (s *Service) DeleteServer(id string) error {
	if s == nil {
		return errors.New("app service is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("server id is empty")
	}
	return s.store.Update(func(cfg *config.Config) error {
		index := -1
		for current, existing := range cfg.Servers {
			if existing.ID == id {
				index = current
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("profile %q not found", id)
		}
		cfg.Servers = append(cfg.Servers[:index], cfg.Servers[index+1:]...)
		if cfg.ActiveServer == id {
			cfg.ActiveServer = ""
		}
		return nil
	})
}

// SetActiveServer marks an existing profile as the active one.
func (s *Service) SetActiveServer(id string) error {
	if s == nil {
		return errors.New("app service is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("server id is empty")
	}
	return s.store.Update(func(cfg *config.Config) error {
		for _, existing := range cfg.Servers {
			if existing.ID == id {
				cfg.ActiveServer = id
				return nil
			}
		}
		return fmt.Errorf("profile %q not found", id)
	})
}

// UpdateSettings persists paths and the interface language.
func (s *Service) UpdateSettings(settings Settings) error {
	if s == nil {
		return errors.New("app service is nil")
	}
	return s.store.Update(func(cfg *config.Config) error {
		cfg.Language = strings.TrimSpace(settings.Language)
		if value := strings.TrimSpace(settings.Theme); value != "" {
			cfg.Theme = value
		}
		cfg.MorrowindPath = strings.TrimSpace(settings.MorrowindPath)
		cfg.WorkDir = strings.TrimSpace(settings.WorkDir)
		cfg.ProtonPath = strings.TrimSpace(settings.ProtonPath)
		cfg.SkipFonts = settings.SkipFonts

		return nil
	})
}
