package fixtures

import (
	"encoding/json"
	"testing"
)

// ServerResponse mirrors GET /api/v1/server from docs/v2/03-contracts.md §7.
type ServerResponse struct {
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	Port         int     `json:"port"`
	Status       string  `json:"status"`
	ModsSHA256   *string `json:"mods_sha256"`
	PanelVersion string  `json:"panel_version"`
}

// ModsCurrentResponse mirrors GET /api/v1/mods/current.
type ModsCurrentResponse struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	URL    string `json:"url"`
}

// APIError is the public API error body.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIErrorResponse is the public API error envelope.
type APIErrorResponse struct {
	Error APIError `json:"error"`
}

// DefaultServer returns a deterministic public server response.
func DefaultServer() ServerResponse {
	return ServerResponse{
		Name:         "Morrowind with friends",
		Address:      "203.0.113.10",
		Port:         25565,
		Status:       "running",
		PanelVersion: "0.1.0",
	}
}

// ServerWithMods returns DefaultServer with mods_sha256 set.
func ServerWithMods(manifest ModsManifest) ServerResponse {
	response := DefaultServer()
	sha := manifest.SHA256
	response.ModsSHA256 = &sha
	return response
}

// DefaultModsCurrent returns the public current-mods response for manifest.
func DefaultModsCurrent(manifest ModsManifest, size int64) ModsCurrentResponse {
	return ModsCurrentResponse{
		Name:   manifest.Name,
		SHA256: manifest.SHA256,
		Size:   size,
		URL:    "/api/v1/mods/current/archive",
	}
}

// DefaultAPIErrors returns every public error code from §7.
func DefaultAPIErrors() []APIErrorResponse {
	return []APIErrorResponse{
		{Error: APIError{Code: "not_found", Message: "Not found"}},
		{Error: APIError{Code: "no_mods", Message: "Current mods archive not found"}},
		{Error: APIError{Code: "rate_limited", Message: "Too many requests"}},
		{Error: APIError{Code: "service_unavailable", Message: "Service unavailable"}},
		{Error: APIError{Code: "internal", Message: "Internal error"}},
	}
}

// JSON marshals value into a deterministic JSON fixture.
func JSON(t testing.TB, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("fixtures: marshal %T: %v", value, err)
	}
	return data
}

// JSONFile writes a marshalled JSON fixture to path.
func JSONFile(t testing.TB, path string, value any) {
	t.Helper()
	WriteFile(t, path, string(JSON(t, value))+"\n")
}
