package panel

import "embed"

// staticFS contains the small same-origin scripts used by the admin UI. Inline
// scripts stay forbidden by the CSP.
//
//go:embed static/*
var staticFS embed.FS
