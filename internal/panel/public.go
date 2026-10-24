package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Size    int64
	ModTime time.Time
}

type homePageData struct {
	Name        string
	ActiveNav   string
	Address     string
	Port        int
	Status      string
	StatusLabel string
	StatusError string
	Mods        *modsCurrent
	ModsError   string
}

type modsCurrent struct {
	Name        string
	SHA256      string
	Size        int64
	SizeLabel   string
	DownloadURL string
}

type backupItemView struct {
	Name        string
	Size        int64
	SizeLabel   string
	Mtime       string
	MtimeLabel  string
	Current     bool
	DownloadURL string
}

type backupGroup struct {
	Label string
	Items []backupItemView
}

type backupsPageData struct {
	Type      string
	ActiveNav string
	TypeLabel string
	Current   string
	Groups    []backupGroup
	ItemCount int
	NextURL   string
	Error     string
	Empty     bool
	ModsURL   string
	StateURL  string
	Limit     int
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

	current, err := h.readCurrentMods()
	if err != nil {
		data.ModsError = "Текущий архив модов не найден."
	} else {
		data.Mods = &modsCurrent{
			Name:        current.Name,
			SHA256:      current.SHA256,
			Size:        current.Size,
			SizeLabel:   humanSize(current.Size),
			DownloadURL: modsCurrentURL,
		}
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
	name := ""
	port := defaultGamePort
	snapshot, err := gameconfig.Read(h.cfg.Root, gameconfig.FileServer)
	if err != nil {
		return name, port
	}
	if value, ok := snapshot.Values["hostname"].(string); ok {
		name = value
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
	return currentMods{Current: current, Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (h *panelHandler) handleBackups(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	typ := query.Get("type")
	if typ == "" {
		typ = "state"
	}
	if typ != "mods" && typ != "state" {
		h.renderBackups(w, http.StatusBadRequest, backupsPageData{
			Type:  "state",
			Error: "Неизвестный тип бэкапа. Допустимы mods и state.",
		})
		return
	}

	limit := defaultBackupLimit
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxBackupLimit {
			h.renderBackups(w, http.StatusBadRequest, backupsPageData{
				Type:  typ,
				Error: "Параметр limit должен быть числом от 1 до 500.",
			})
			return
		}
		limit = parsed
	}
	before := query.Get("before")
	if before != "" && !safeArchiveName(before) {
		h.renderBackups(w, http.StatusBadRequest, backupsPageData{
			Type:  typ,
			Error: "Некорректное имя архива в параметре before.",
		})
		return
	}
	if h.ops == nil {
		h.renderBackups(w, http.StatusServiceUnavailable, backupsPageData{
			Type:  typ,
			Error: "Сервер временно недоступен: ops-клиент не настроен.",
		})
		return
	}

	backups, err := h.ops.BackupsList(r.Context(), typ, limit, before)
	if err != nil {
		code, message := publicPageFailure(err)
		h.renderBackups(w, code, backupsPageData{Type: typ, Error: message})
		return
	}

	data := backupsPageData{
		Type:      typ,
		TypeLabel: backupTypeLabel(typ),
		Groups:    groupBackups(backups.Items, typ, h.now()),
		ItemCount: len(backups.Items),
		Empty:     len(backups.Items) == 0,
		ModsURL:   "/backups?type=mods",
		StateURL:  "/backups?type=state",
		Limit:     limit,
	}
	if backups.Current != nil {
		data.Current = *backups.Current
	}
	if backups.NextBefore != nil && *backups.NextBefore != "" {
		values := url.Values{"type": {typ}, "before": {*backups.NextBefore}}
		if limit != defaultBackupLimit {
			values.Set("limit", strconv.Itoa(limit))
		}
		data.NextURL = "/backups?" + values.Encode()
	}
	h.renderBackups(w, http.StatusOK, data)
}

func (h *panelHandler) renderBackups(w http.ResponseWriter, status int, data backupsPageData) {
	if data.ActiveNav == "" {
		data.ActiveNav = "backups"
	}
	if data.ModsURL == "" {
		data.ModsURL = "/backups?type=mods"
	}
	if data.StateURL == "" {
		data.StateURL = "/backups?type=state"
	}
	if data.TypeLabel == "" {
		data.TypeLabel = backupTypeLabel(data.Type)
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
		target := filepath.Join(h.cfg.Root, "backups", typ, name)
		info, err := os.Lstat(target)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
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

func backupTypeLabel(typ string) string {
	if typ == "mods" {
		return "Моды"
	}
	return "Состояние"
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

func groupBackups(items []BackupItem, typ string, now time.Time) []backupGroup {
	var groups []backupGroup
	for _, item := range items {
		label := backupGroupLabel(item.Mtime, now)
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, backupGroup{Label: label})
		}
		mtime, mtimeLabel := backupTime(item.Mtime)
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items,
			backupItemView{
				Name:        item.Name,
				Size:        item.Size,
				SizeLabel:   humanSize(item.Size),
				Mtime:       mtime,
				MtimeLabel:  mtimeLabel,
				Current:     item.Current,
				DownloadURL: "/backups/" + typ + "/" + url.PathEscape(item.Name),
			})
	}
	return groups
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
