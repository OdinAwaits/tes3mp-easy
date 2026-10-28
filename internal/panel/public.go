package panel

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/gameconfig"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

const (
	defaultGamePort    = 25565
	defaultServerName  = "TES3MP"
	defaultBackupLimit = 200
	maxBackupLimit     = 500
	modsCurrentURL     = "/api/v1/mods/current/archive"
)

var (
	errNoMods         = errors.New("current mods archive is unavailable")
	errOpsUnavailable = errors.New("ops client is not configured")
)

// serverInfoResponse is the exact public GET /api/v1/server body from
// docs/v2/03-contracts.md §7.
type serverInfoResponse struct {
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	Port         int     `json:"port"`
	Status       string  `json:"status"`
	ModsSHA256   *string `json:"mods_sha256"`
	PanelVersion string  `json:"panel_version"`
}

// modsCurrentResponse is the exact public GET /api/v1/mods/current body.
type modsCurrentResponse struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	URL    string `json:"url"`
}

type publicAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type publicAPIErrorResponse struct {
	Error publicAPIError `json:"error"`
}

type currentMods struct {
	archive.Current
	Size int64
}

type homePageData struct {
	pageI18n
	Name        string
	ActiveNav   string
	Nav         string
	Address     string
	Port        int
	Status      string
	StatusLabel string
	StatusError string
}

// archiveItem is the shared view model for state and mods archives. Both the
// public archive page and the admin pages render it, so URL, availability and
// restore rules live in one place.
type archiveItem struct {
	Name            string
	Size            int64
	SizeLabel       string
	Mtime           string
	MtimeLabel      string
	MtimeAgo        string
	Current         bool
	ModsArchive     string
	ModsAvailable   bool
	ModsCurrent     bool
	CanRestore      bool
	DownloadURL     string
	ModsDownloadURL string
	BundleURL       string
}

type archiveGroup struct {
	Label string
	Items []archiveItem
}

// archiveOptions selects the link style and the current mods context.
type archiveOptions struct {
	Admin       bool
	CurrentMods string
}

func archiveItems(tr *i18n.Translator, items []BackupItem, typ string, opt archiveOptions,
	now time.Time,
) []archiveItem {
	views := make([]archiveItem, 0, len(items))
	for _, item := range items {
		mtime, mtimeLabel := backupTime(item.Mtime)
		view := archiveItem{
			Name:        item.Name,
			Size:        item.Size,
			SizeLabel:   humanSize(item.Size),
			Mtime:       mtime,
			MtimeLabel:  mtimeLabel,
			MtimeAgo:    timeLabel(tr, item.Mtime, now),
			Current:     item.Current,
			DownloadURL: archiveDownloadURL(typ, item.Name, opt.Admin),
		}
		if item.ModsArchive != nil {
			view.ModsArchive = *item.ModsArchive
		}
		if item.ModsAvailable != nil {
			view.ModsAvailable = *item.ModsAvailable
		}
		if typ == "state" && view.ModsArchive != "" &&
			archive.ValidateModsArchiveName(view.ModsArchive) == nil {
			view.ModsCurrent = opt.CurrentMods != "" && view.ModsArchive == opt.CurrentMods
			if view.ModsAvailable {
				view.ModsDownloadURL = archiveDownloadURL("mods", view.ModsArchive, opt.Admin)
				if !opt.Admin {
					view.BundleURL = view.DownloadURL + "/bundle"
				}
			}
		}
		// Restoring a state is always possible when its mod pack is still on
		// disk: the panel enables the pack first (see handleStateDeploy), so the
		// operator no longer has to deploy mods manually before restoring.
		view.CanRestore = opt.Admin && typ == "state" && view.ModsAvailable
		views = append(views, view)
	}
	return views
}

func archiveDownloadURL(typ, name string, admin bool) string {
	if admin {
		return "/admin/" + typ + "/download/" + url.PathEscape(name)
	}
	return "/backups/" + typ + "/" + url.PathEscape(name)
}

func groupArchiveItems(tr *i18n.Translator, items []archiveItem, now time.Time) []archiveGroup {
	var groups []archiveGroup
	for _, item := range items {
		label := backupGroupLabel(tr, item.Mtime, now)
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, archiveGroup{Label: label})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, item)
	}
	return groups
}

type backupsPageData struct {
	pageI18n
	ActiveNav    string
	Nav          string
	StateGroups  []archiveGroup
	ModsGroups   []archiveGroup
	StateEmpty   bool
	ModsEmpty    bool
	StateNextURL string
	ModsNextURL  string
	Limit        int
	Error        string
}

func (h *panelHandler) allowDownload(w http.ResponseWriter, r *http.Request) bool {
	allowed, retry := h.downloads.allow(clientIP(r), h.now())
	if allowed {
		return true
	}
	w.Header().Set("Retry-After", retryAfterSeconds(retry))
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeAPIError(w, http.StatusTooManyRequests, "rate_limited",
			h.tr().T(i18n.KeyPanelPublicRateLimited))
		return false
	}
	http.Error(w, "too many requests", http.StatusTooManyRequests)
	return false
}

func (h *panelHandler) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	status, err := h.serverStatus(r.Context())
	if err != nil {
		code, message := publicOpsFailure(h.tr(), err)
		writeAPIError(w, code, publicErrorCode(code), message)
		return
	}

	name, port := h.serverNamePort()
	var modsSHA *string
	if current, currentErr := h.readCurrentMods(); currentErr == nil {
		sha := current.SHA256
		modsSHA = &sha
	}

	writeJSON(w, http.StatusOK, "no-store", serverInfoResponse{
		Name:         name,
		Address:      publicAddress(h.cfg, r),
		Port:         port,
		Status:       status,
		ModsSHA256:   modsSHA,
		PanelVersion: Version,
	})
}

func (h *panelHandler) handleModsCurrent(w http.ResponseWriter, _ *http.Request) {
	current, err := h.readCurrentMods()
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "no_mods",
			h.tr().T(i18n.KeyPanelAdminModsMissing))
		return
	}
	writeJSON(w, http.StatusOK, "no-store", modsCurrentResponse{
		Name:   current.Name,
		SHA256: current.SHA256,
		Size:   current.Size,
		URL:    modsCurrentURL,
	})
}

func (h *panelHandler) handleModsArchive(w http.ResponseWriter, r *http.Request) {
	if !h.allowDownload(w, r) {
		return
	}

	current, err := h.readCurrentMods()
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "no_mods",
			h.tr().T(i18n.KeyPanelAdminModsMissing))
		return
	}
	file, err := os.Open(filepath.Join(h.cfg.Root, "backups", "mods", current.Name))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "no_mods",
			h.tr().T(i18n.KeyPanelAdminModsMissing))
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeAPIError(w, http.StatusNotFound, "no_mods",
			h.tr().T(i18n.KeyPanelAdminModsMissing))
		return
	}

	etag := `"` + current.SHA256 + `"`
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", contentDisposition(current.Name))
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	http.ServeContent(w, r, current.Name, info.ModTime(), file)
}

func (h *panelHandler) handleHome(w http.ResponseWriter, r *http.Request) {
	name, port := h.serverNamePort()
	data := homePageData{
		pageI18n:    h.pageI18n(),
		Name:        "TES3MP",
		ActiveNav:   "home",
		Nav:         "public",
		Address:     publicAddress(h.cfg, r),
		Port:        port,
		Status:      "unknown",
		StatusLabel: statusLabel(h.tr(), "unknown"),
	}
	if name != "" {
		data.Name = name
	}

	status, err := h.serverStatus(r.Context())
	if err != nil {
		data.Status = "service_unavailable"
		data.StatusLabel = statusLabel(h.tr(), data.Status)
		data.StatusError = h.tr().T(i18n.KeyPanelPublicUnavailableLong)
	} else {
		data.Status = status
		data.StatusLabel = statusLabel(h.tr(), status)
	}

	h.render(w, http.StatusOK, "home.html", data)
}

func (h *panelHandler) serverStatus(ctx context.Context) (string, error) {
	if h.ops == nil {
		return "", errOpsUnavailable
	}
	status, err := h.ops.Status(ctx, "tes3mp.service")
	if err != nil {
		return "", err
	}
	if status.State == "" {
		return "unknown", nil
	}
	return status.State, nil
}

func (h *panelHandler) serverNamePort() (string, int) {
	name := defaultServerName
	port := defaultGamePort
	snapshot, err := gameconfig.Read(h.cfg.Root, gameconfig.FileServer)
	if err != nil {
		return name, port
	}
	if value, ok := snapshot.Values["hostname"].(string); ok {
		if value = strings.TrimSpace(value); value != "" {
			name = value
		}
	}
	if value, ok := snapshot.Values["port"].(int); ok {
		port = value
	}
	return name, port
}

func (h *panelHandler) readCurrentMods() (currentMods, error) {
	dir := filepath.Join(h.cfg.Root, "backups", "mods")
	current, err := archive.ReadCurrentFile(filepath.Join(dir, "current.txt"))
	if err != nil {
		return currentMods{}, errNoMods
	}
	info, err := os.Lstat(filepath.Join(dir, current.Name))
	if err != nil || !info.Mode().IsRegular() ||
		info.Size() > archive.MaxCompressedSize {
		return currentMods{}, errNoMods
	}
	return currentMods{Current: current, Size: info.Size()}, nil
}

func (h *panelHandler) handleBackups(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := defaultBackupLimit
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxBackupLimit {
			h.renderBackups(w, http.StatusBadRequest, backupsPageData{
				Error: h.tr().T(i18n.KeyPanelAdminLimitInvalid),
			})
			return
		}
		limit = parsed
	}

	stateBefore := query.Get("state_before")
	modsBefore := query.Get("mods_before")
	for _, before := range []string{stateBefore, modsBefore} {
		if before != "" && !safeArchiveName(before) {
			h.renderBackups(w, http.StatusBadRequest, backupsPageData{
				Error: h.tr().T(i18n.KeyPanelAdminBeforeInvalid),
			})
			return
		}
	}
	if h.ops == nil {
		h.renderBackups(w, http.StatusServiceUnavailable, backupsPageData{
			Error: h.tr().T(i18n.KeyPanelAdminOpsUnavailable),
		})
		return
	}

	stateBackups, err := h.ops.BackupsList(r.Context(), "state", limit, stateBefore)
	if err != nil {
		code, message := publicPageFailure(h.tr(), err)
		h.renderBackups(w, code, backupsPageData{Error: message})
		return
	}
	modsBackups, err := h.ops.BackupsList(r.Context(), "mods", limit, modsBefore)
	if err != nil {
		code, message := publicPageFailure(h.tr(), err)
		h.renderBackups(w, code, backupsPageData{Error: message})
		return
	}

	now := h.now()
	data := backupsPageData{
		StateGroups: groupArchiveItems(h.tr(), archiveItems(h.tr(), stateBackups.Items, "state", archiveOptions{}, now), now),
		ModsGroups:  groupArchiveItems(h.tr(), archiveItems(h.tr(), modsBackups.Items, "mods", archiveOptions{}, now), now),
		StateEmpty:  len(stateBackups.Items) == 0,
		ModsEmpty:   len(modsBackups.Items) == 0,
		Limit:       limit,
	}
	data.StateNextURL = backupNextURL("state", stateBackups.NextBefore,
		limit, stateBefore, modsBefore)
	data.ModsNextURL = backupNextURL("mods", modsBackups.NextBefore,
		limit, stateBefore, modsBefore)
	h.renderBackups(w, http.StatusOK, data)
}

func backupNextURL(typ string, nextBefore *string, limit int,
	stateBefore, modsBefore string,
) string {
	if nextBefore == nil || *nextBefore == "" {
		return ""
	}
	values := url.Values{}
	if stateBefore != "" {
		values.Set("state_before", stateBefore)
	}
	if modsBefore != "" {
		values.Set("mods_before", modsBefore)
	}
	if typ == "state" {
		values.Set("state_before", *nextBefore)
	} else {
		values.Set("mods_before", *nextBefore)
	}
	if limit != defaultBackupLimit {
		values.Set("limit", strconv.Itoa(limit))
	}
	return "/backups?" + values.Encode() + "#" + typ
}

func (h *panelHandler) renderBackups(w http.ResponseWriter, status int, data backupsPageData) {
	data.pageI18n = h.pageI18n()
	data.Nav = "public"
	if data.ActiveNav == "" {
		data.ActiveNav = "backups"
	}
	if data.Limit == 0 {
		data.Limit = defaultBackupLimit
	}
	// Public archives are meant to be downloadable, not indexed: a search
	// engine must not keep copies of a world that changes every day.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	h.render(w, status, "backups.html", data)
}

func (h *panelHandler) handleBackupDownload(typ string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.allowDownload(w, r) {
			return
		}
		name := r.PathValue("name")
		if !safeArchiveName(name) {
			http.NotFound(w, r)
			return
		}
		switch typ {
		case "mods":
			if archive.ValidateModsArchiveName(name) != nil {
				http.NotFound(w, r)
				return
			}
		case "state":
			if archive.ValidateStateArchiveName(name) != nil {
				http.NotFound(w, r)
				return
			}
		default:
			http.NotFound(w, r)
			return
		}
		target := filepath.Join(h.cfg.Root, "backups", typ, name)
		info, err := os.Lstat(target)
		if err != nil || info.Mode()&os.ModeSymlink != 0 ||
			!info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		if typ == "state" {
			// Old archives may contain the server config with its password
			// or banlist.json. Serve only archives that match the current
			// public state layout.
			if _, err := archive.ValidateStateLayoutFile(target); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		file, err := os.Open(target)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", contentDisposition(name))
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		http.ServeContent(w, r, name, info.ModTime(), file)
	}
}

func (h *panelHandler) handleStateBundle(w http.ResponseWriter, r *http.Request) {
	if !h.allowDownload(w, r) {
		return
	}
	name := r.PathValue("name")
	if !safeArchiveName(name) || archive.ValidateStateArchiveName(name) != nil {
		http.NotFound(w, r)
		return
	}

	statePath := filepath.Join(h.cfg.Root, "backups", "state", name)
	stateInfo, err := os.Lstat(statePath)
	if err != nil || stateInfo.Mode()&os.ModeSymlink != 0 ||
		!stateInfo.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	contents, err := archive.ValidateStateLayoutFile(statePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	modsName := contents.Current.Name
	if archive.ValidateModsArchiveName(modsName) != nil {
		http.NotFound(w, r)
		return
	}
	modsPath := filepath.Join(h.cfg.Root, "backups", "mods", modsName)
	modsInfo, err := os.Lstat(modsPath)
	if err != nil || modsInfo.Mode()&os.ModeSymlink != 0 ||
		!modsInfo.Mode().IsRegular() ||
		modsInfo.Size() > archive.MaxCompressedSize {
		http.NotFound(w, r)
		return
	}

	stateFile, err := os.Open(statePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer stateFile.Close()
	modsFile, err := os.Open(modsPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer modsFile.Close()

	bundleName := strings.TrimSuffix(name, ".tar.gz") + "-with-mods.zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition(bundleName))
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.WriteHeader(http.StatusOK)

	writer := zip.NewWriter(w)
	for _, entry := range []struct {
		name string
		info os.FileInfo
		file *os.File
	}{
		{name: name, info: stateInfo, file: stateFile},
		{name: modsName, info: modsInfo, file: modsFile},
	} {
		header := &zip.FileHeader{
			Name:     entry.name,
			Method:   zip.Store,
			Modified: entry.info.ModTime(),
		}
		target, err := writer.CreateHeader(header)
		if err != nil {
			return
		}
		if _, err := io.Copy(target, entry.file); err != nil {
			return
		}
	}
	_ = writer.Close()
}

func writeJSON(w http.ResponseWriter, status int, cacheControl string, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, "no-store", publicAPIErrorResponse{
		Error: publicAPIError{Code: code, Message: message},
	})
}

func publicOpsFailure(tr *i18n.Translator, err error) (int, string) {
	if errors.Is(err, errOpsUnavailable) {
		return http.StatusServiceUnavailable, tr.T(i18n.KeyPanelPublicUnavailable)
	}
	switch OpsHTTPStatus(err) {
	case http.StatusServiceUnavailable:
		return http.StatusServiceUnavailable, tr.T(i18n.KeyPanelPublicUnavailable)
	default:
		return http.StatusInternalServerError, tr.T(i18n.KeyPanelErrorInternalShort)
	}
}

// publicPageFailure maps an ops error to a page status and a message that does
// not expose subprocess diagnostics or filesystem paths.
func publicPageFailure(tr *i18n.Translator, err error) (int, string) {
	status := OpsHTTPStatus(err)
	switch status {
	case http.StatusBadRequest:
		return status, tr.T(i18n.KeyPanelPublicBadRequest)
	case http.StatusNotFound:
		return status, tr.T(i18n.KeyPanelPublicNotFound)
	case http.StatusConflict:
		return status, tr.T(i18n.KeyPanelErrorLocked)
	case http.StatusUnprocessableEntity:
		return status, tr.T(i18n.KeyPanelPublicCorrupted)
	case http.StatusInsufficientStorage:
		return status, tr.T(i18n.KeyPanelPublicNoSpace)
	case http.StatusServiceUnavailable:
		return status, tr.T(i18n.KeyPanelPublicUnavailableLong)
	default:
		return http.StatusInternalServerError, tr.T(i18n.KeyPanelErrorInternalShort)
	}
}

func publicErrorCode(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal"
	}
}

func publicAddress(cfg Config, r *http.Request) string {
	if cfg.PublicAddress != "" {
		if host, _, err := net.SplitHostPort(cfg.PublicAddress); err == nil {
			return host
		}
		return strings.Trim(cfg.PublicAddress, "[]")
	}
	if host, _, err := net.SplitHostPort(r.Host); err == nil {
		return host
	}
	host := strings.Trim(r.Host, "[]")
	if host == "" {
		return "localhost"
	}
	return host
}

func statusLabel(tr *i18n.Translator, status string) string {
	switch status {
	case "running":
		return tr.T(i18n.KeyPanelStatusRunning)
	case "stopped":
		return tr.T(i18n.KeyPanelStatusStopped)
	case "starting":
		return tr.T(i18n.KeyPanelStatusStarting)
	case "unhealthy":
		return tr.T(i18n.KeyPanelStatusUnhealthy)
	case "service_unavailable":
		return tr.T(i18n.KeyPanelStatusUnavailable)
	default:
		return tr.T(i18n.KeyPanelStatusUnknown)
	}
}

func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

// backupGroupLabel groups archives by day for the last week, by ISO week for
// the last year and by month after that. The labels are meant for humans; the
// exact timestamp stays in <time datetime>.
func backupGroupLabel(tr *i18n.Translator, raw string, now time.Time) string {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return tr.T(i18n.KeyPanelGroupOther)
	}
	at = at.UTC()
	now = now.UTC()
	age := now.Sub(at)
	switch {
	case sameUTCDay(at, now):
		return tr.T(i18n.KeyPanelGroupToday)
	case sameUTCDay(at, now.AddDate(0, 0, -1)):
		return tr.T(i18n.KeyPanelGroupYesterday)
	case age <= 7*24*time.Hour:
		return at.Format("02.01.2006")
	case age <= 365*24*time.Hour:
		year, week := at.ISOWeek()
		return tr.Tf(i18n.KeyPanelGroupWeek, week, year)
	default:
		return tr.T(monthNameKey(at.Month())) + " " + at.Format("2006")
	}
}

// monthNameKey maps a month to its catalog key, keeping the group labels of the
// archive pages in the operator's language.
func monthNameKey(month time.Month) string {
	keys := [...]string{
		i18n.KeyPanelMonthJanuary, i18n.KeyPanelMonthFebruary,
		i18n.KeyPanelMonthMarch, i18n.KeyPanelMonthApril,
		i18n.KeyPanelMonthMay, i18n.KeyPanelMonthJune,
		i18n.KeyPanelMonthJuly, i18n.KeyPanelMonthAugust,
		i18n.KeyPanelMonthSeptember, i18n.KeyPanelMonthOctober,
		i18n.KeyPanelMonthNovember, i18n.KeyPanelMonthDecember,
	}
	if month < time.January || month > time.December {
		return i18n.KeyPanelGroupOther
	}
	return keys[month-1]
}

func sameUTCDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// backupTime returns the machine-readable timestamp and the short absolute
// label used in tables ("23.10.2026 11:00 UTC").
func backupTime(raw string) (string, string) {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw, raw
	}
	at = at.UTC()
	return at.Format(time.RFC3339), at.Format("02.01.2006 15:04 UTC")
}

// timeLabel renders a timestamp relative to now ("5 minutes ago"). It is used
// where the exact value matters less than the age; invalid input is returned
// unchanged so the page never hides data.
func timeLabel(tr *i18n.Translator, raw string, now time.Time) string {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return humanAgo(tr, at, now)
}

// humanAgo renders how long ago at happened in the language of the panel.
func humanAgo(tr *i18n.Translator, at, now time.Time) string {
	if !at.Before(now) {
		return tr.T(i18n.KeyPanelTimeJustNow)
	}
	age := now.Sub(at)
	var value int
	var key string
	switch {
	case age < time.Minute:
		return tr.T(i18n.KeyPanelTimeJustNow)
	case age < time.Hour:
		value, key = int(age/time.Minute), i18n.KeyPanelTimeMinuteAgo
	case age < 24*time.Hour:
		value, key = int(age/time.Hour), i18n.KeyPanelTimeHourAgo
	case age < 30*24*time.Hour:
		value, key = int(age/(24*time.Hour)), i18n.KeyPanelTimeDayAgo
	case age < 365*24*time.Hour:
		value, key = int(age/(30*24*time.Hour)), i18n.KeyPanelTimeMonthAgo
	default:
		value, key = int(age/(365*24*time.Hour)), i18n.KeyPanelTimeYearAgo
	}
	if value < 1 {
		value = 1
	}
	return tr.Pluralf(key, value)
}

// healthLabel turns the ops health verdict into an operator-facing word.
func healthLabel(tr *i18n.Translator, health string) string {
	switch health {
	case "healthy":
		return tr.T(i18n.KeyPanelHealthOK)
	case "unhealthy":
		return tr.T(i18n.KeyPanelHealthProblems)
	default:
		return tr.T(i18n.KeyPanelHealthUnknown)
	}
}

func safeArchiveName(name string) bool {
	if name == "" || len(name) > 255 || path.Base(name) != name ||
		!strings.HasSuffix(name, ".tar.gz") {
		return false
	}
	for index := 0; index < len(name); index++ {
		ch := name[index]
		if ch < 0x20 || ch == 0x7f || ch == '/' || ch == '\\' || ch == '"' {
			return false
		}
	}
	return true
}

func contentDisposition(name string) string {
	return `attachment; filename="` + name + `"`
}

func matchesETag(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if strings.HasPrefix(candidate, "W/") {
			candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "W/"))
		}
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
