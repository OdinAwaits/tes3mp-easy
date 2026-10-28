package panel

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

// pageTemplates holds one parsed set per page: the shared layout and partials
// plus the page-specific title, scripts, body attributes and content.
type pageTemplates map[string]*template.Template

var templatePageNames = []string{
	"admin_config.html",
	"admin_logs.html",
	"admin_mods.html",
	"admin_staff.html",
	"backups.html",
	"dashboard.html",
	"home.html",
	"login.html",
	"password.html",
	"state.html",
}

var templateSharedFiles = []string{
	"templates/layout.html",
	"templates/head.html",
	"templates/admin_nav.html",
	"templates/public_nav.html",
}

func loadTemplates() (pageTemplates, error) {
	base, err := template.ParseFS(templateFS, templateSharedFiles...)
	if err != nil {
		return nil, err
	}
	pages := make(pageTemplates, len(templatePageNames))
	for _, name := range templatePageNames {
		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		parsed, err := clone.ParseFS(templateFS, "templates/"+name)
		if err != nil {
			return nil, err
		}
		pages[name] = parsed
	}
	return pages, nil
}
