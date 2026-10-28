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

func archiveItems(items []BackupItem, typ string, opt archiveOptions) []archiveItem {
	views := make([]archiveItem, 0, len(items))
	for _, item := range items {
		mtime, mtimeLabel := backupTime(item.Mtime)
		view := archiveItem{
			Name:        item.Name,
			Size:        item.Size,
			SizeLabel:   humanSize(item.Size),
			Mtime:       mtime,
			MtimeLabel:  mtimeLabel,
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
		view.CanRestore = opt.Admin && typ == "state" && view.ModsAvailable && view.ModsCurrent
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

func groupArchiveItems(items []archiveItem, now time.Time) []archiveGroup {
	var groups []archiveGroup
	for _, item := range items {
		label := backupGroupLabel(item.Mtime, now)
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, archiveGroup{Label: label})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, item)
	}
	return groups
}

type backupsPageData struct {
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
			"Слишком много запросов. Попробуйте позже.")
		return false
	}
	http.Error(w, "too many requests", http.StatusTooManyRequests)
	return false
}

func (h *panelHandler) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	status, err := h.serverStatus(r.Context())
	if err != nil {
		code, message := publicOpsFailure(err)
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
			"Текущий архив модов не найден")
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
			"Текущий архив модов не найден")
		return
	}
	file, err := os.Open(filepath.Join(h.cfg.Root, "backups", "mods", current.Name))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "no_mods",
			"Текущий архив модов не найден")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeAPIError(w, http.StatusNotFound, "no_mods",
			"Текущий архив модов не найден")
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
		Name:        "TES3MP",
		ActiveNav:   "home",
		Nav:         "public",
		Address:     publicAddress(h.cfg, r),
		Port:        port,
		Status:      "unknown",
		StatusLabel: statusLabel("unknown"),
	}
	if name != "" {
		data.Name = name
	}

	status, err := h.serverStatus(r.Context())
	if err != nil {
		data.Status = "service_unavailable"
		data.StatusLabel = statusLabel(data.Status)
		data.StatusError = "Сервер временно недоступен: systemd или unit не отвечает."
	} else {
		data.Status = status
		data.StatusLabel = statusLabel(status)
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
				Error: "Параметр limit должен быть числом от 1 до 500.",
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
				Error: "Некорректное имя архива в параметре before.",
			})
			return
		}
	}
	if h.ops == nil {
		h.renderBackups(w, http.StatusServiceUnavailable, backupsPageData{
			Error: "Сервер временно недоступен: ops-клиент не настроен.",
		})
		return
	}

	stateBackups, err := h.ops.BackupsList(r.Context(), "state", limit, stateBefore)
	if err != nil {
		code, message := publicPageFailure(err)
		h.renderBackups(w, code, backupsPageData{Error: message})
		return
	}
	modsBackups, err := h.ops.BackupsList(r.Context(), "mods", limit, modsBefore)
	if err != nil {
		code, message := publicPageFailure(err)
		h.renderBackups(w, code, backupsPageData{Error: message})
		return
	}

	data := backupsPageData{
		StateGroups: groupArchiveItems(archiveItems(stateBackups.Items, "state", archiveOptions{}), h.now()),
		ModsGroups:  groupArchiveItems(archiveItems(modsBackups.Items, "mods", archiveOptions{}), h.now()),
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
	data.Nav = "public"
	if data.ActiveNav == "" {
		data.ActiveNav = "backups"
	}
	if data.Limit == 0 {
		data.Limit = defaultBackupLimit
	}
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

func publicOpsFailure(err error) (int, string) {
	if errors.Is(err, errOpsUnavailable) {
		return http.StatusServiceUnavailable, "Сервер временно недоступен."
	}
	switch OpsHTTPStatus(err) {
	case http.StatusServiceUnavailable:
		return http.StatusServiceUnavailable, "Сервер временно недоступен."
	default:
		return http.StatusInternalServerError, "Внутренняя ошибка."
	}
}

// publicPageFailure maps an ops error to a page status and a message that does
// not expose subprocess diagnostics or filesystem paths.
func publicPageFailure(err error) (int, string) {
	status := OpsHTTPStatus(err)
	switch status {
	case http.StatusBadRequest:
		return status, "Проверьте параметры запроса."
	case http.StatusNotFound:
		return status, "Запрошенный архив не найден."
	case http.StatusConflict:
		return status, "Другая операция уже выполняется. Попробуйте позже."
	case http.StatusUnprocessableEntity:
		return status, "Архив или конфигурация повреждены."
	case http.StatusInsufficientStorage:
		return status, "Недостаточно свободного места для операции."
	case http.StatusServiceUnavailable:
		return status, "Сервер временно недоступен: systemd или unit не отвечает."
	default:
		return http.StatusInternalServerError, "Внутренняя ошибка."
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

func statusLabel(status string) string {
	switch status {
	case "running":
		return "работает"
	case "stopped":
		return "остановлен"
	case "starting":
		return "запускается"
	case "unhealthy":
		return "нездоров"
	case "service_unavailable":
		return "недоступен"
	default:
		return "неизвестно"
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

func backupGroupLabel(raw string, now time.Time) string {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "Другие"
	}
	at = at.UTC()
	age := now.Sub(at)
	switch {
	case age <= 7*24*time.Hour:
		return at.Format("2006-01-02")
	case age <= 365*24*time.Hour:
		year, week := at.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	default:
		return at.Format("2006-01")
	}
}

func backupTime(raw string) (string, string) {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw, raw
	}
	at = at.UTC()
	return at.Format(time.RFC3339), at.Format("2006-01-02 15:04 UTC")
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
