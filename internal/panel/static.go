package panel

import "embed"

// staticFS contains the same-origin CSS and small scripts used by the panel.
// Inline scripts and styles stay forbidden by the CSP.
//
//go:embed static/*
var staticFS embed.FS
