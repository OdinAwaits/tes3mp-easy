package panel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
)

const (
	defaultLogTail      = 200
	maxLogTail          = 2000
	maxAdminFormBytes   = 64 << 10
	maxConfigFormBytes  = 1 << 20
	maxModsUploadBytes  = 512 << 20
	maxUploadBodyBytes  = maxModsUploadBytes + (1 << 20)
	maxPlayerFileBytes  = 4 << 20
	maxStaffPlayers     = 1000
	adminBackupPageSize = 200
)

// opsDoctor and opsSnapshotter are optional capabilities. The public Ops
// interface stays small; OpsClient implements both, while fakes can opt in
// when a test needs diagnostics or read_only config keys.
type opsDoctor interface {
	Doctor(context.Context) (Doctor, error)
}

type opsSnapshotter interface {
	ConfigSnapshot(context.Context, string) (ConfigSnapshot, error)
}

// opsConfigBatch is optional: production ops can apply a whole form with one
// command, while an older client falls back to one ConfigSet per changed field.
type opsConfigBatch interface {
	ConfigSetMany(context.Context, string, map[string]any) error
}

// currentDoctor caches `ops doctor` for one minute so the dashboard poll does
// not spawn a subprocess every five seconds. Mutations share the same cache to
// refuse operations that doctor already marked as doomed.
func (h *panelHandler) currentDoctor(ctx context.Context) (Doctor, error) {
	doctor, ok := h.ops.(opsDoctor)
	if !ok {
		return Doctor{}, errOpsUnavailable
	}
	h.doctorMu.Lock()
	defer h.doctorMu.Unlock()
	if h.doctorValid && h.now().Sub(h.doctorAt) < time.Minute {
		return h.doctorReport, h.doctorErr
	}
	report, err := doctor.Doctor(ctx)
	if report.Checks == nil {
		report.Checks = []DoctorCheck{}
	}
	h.doctorReport = report
	h.doctorErr = err
	h.doctorAt = h.now()
	h.doctorValid = true
	return report, err
}

func (h *panelHandler) doctorMutationError(ctx context.Context) (bool, string) {
	report, _ := h.currentDoctor(ctx)
	if report.Status != "error" {
		return false, ""
	}
	var failed []string
	for _, check := range report.Checks {
		if check.Status == "error" {
			name := check.Name
			if check.Message != "" {
				name += ": " + check.Message
			}
			failed = append(failed, name)
		}
	}
	if len(failed) == 0 {
		return true, "doctor нашёл ошибку; мутация заблокирована."
	}
	return true, "doctor нашёл ошибку, мутация заблокирована: " +
		strings.Join(failed, "; ")
}

type adminPage struct {
	CSRF            string
	ActiveNav       string
	Notice          string
	Error           string
	ErrorField      string
	RestartRequired bool
}

type dashboardPageData struct {
	adminPage
	ServerName   string
	Address      string
	Port         int
	Status       string
	StatusLabel  string
	Health       string
	StartedAt    string
	StatusError  string
	Timer        string
	TimerError   string
	Events       []string
	EventsError  string
	Doctor       *Doctor
	DoctorError  string
	GuardTimer   string
	GuardError   string
	Mods         *modsCurrent
	ModsError    string
	Metrics      metricsView
	MetricsError string
	Window       string
}

type adminLogsPageData struct {
	adminPage
	Lines []string
	Tail  int
}

type adminModsPageData struct {
	adminPage
	Items    []adminBackupItem
	Current  string
	SHA256   string
	Previous string
	Empty    bool
	ModsURL  string
}

type adminStatePageData struct {
	adminPage
	Groups    []adminBackupGroup
	NextURL   string
	Empty     bool
	Limit     int
	StateURL  string
	ModsURL   string
	ServerURL string
	Warnings  []string
}

type adminBackupItem struct {
	Name          string
	Size          int64
	SizeLabel     string
	Mtime         string
	MtimeLabel    string
	Current       bool
	Class         string
	Reason        string
	Label         string
	Pinned        bool
	BaseName      string
	BaseAvailable bool
	UsedBy        int
	CanPin        bool
	CanDelete     bool
	NeedsForce    bool
}

type adminBackupGroup struct {
	Label string
	Items []adminBackupItem
}

type adminConfigPageData struct {
	adminPage
	Sections []adminConfigSection
}

type adminConfigSection struct {
	File      string
	Title     string
	Fields    []adminConfigField
	ReadOnly  []string
	Banlist   []adminConfigList
	Note      string
	Error     string
	SaveLabel string
}

type adminConfigField struct {
	Key     string
	Label   string
	Kind    string
	Value   string
	Checked bool
}

type adminConfigList struct {
	Key   string
	Label string
	Value string
}

type adminStaffPageData struct {
	adminPage
	Players []adminStaffPlayer
	Empty   bool
}

type adminStaffPlayer struct {
	Name     string
	Rank     int
	RankText string
	SetURL   string
	Error    string
}

func adminPageFor(r *http.Request, session sessionState) adminPage {
	page := adminPage{CSRF: session.CSRF, ActiveNav: adminActiveNav(r.URL.Path)}
	switch r.URL.Query().Get("ok") {
	case "server_started":
		page.Notice = "Сервер запускается."
	case "server_stopped":
		page.Notice = "Сервер остановлен."
	case "server_restarted":
		page.Notice = "Сервер перезапускается."
	case "backup_created":
		page.Notice = "Резервная копия создана."
	case "mods_uploaded":
		page.Notice = "Архив модов загружен и импортирован."
	case "mods_deployed":
		page.Notice = "Моды задеплоены."
	case "mods_rolled_back":
		page.Notice = "Предыдущий архив модов задеплоен."
	case "state_deployed":
		page.Notice = "Состояние восстановлено."
	case "checkpoint_pinned":
		page.Notice = "Чекпоинт закреплён: retention его не удалит."
	case "checkpoint_unpinned":
		page.Notice = "Чекпоинт откреплён."
	case "restore_point_deleted":
		page.Notice = "Архив удалён."
	case "staff_saved":
		page.Notice = "Ранг изменён. Перезапустите сервер, чтобы применить его."
	case "config_saved":
		page.Notice = "Настройка сохранена. Перезапустите сервер, чтобы применить её."
	}
	page.RestartRequired = r.URL.Query().Get("restart") == "1" ||
		r.URL.Query().Get("ok") == "staff_saved" ||
		r.URL.Query().Get("ok") == "config_saved"
	return page
}

func adminActiveNav(path string) string {
	switch {
	case path == "/admin" || path == "/admin/" || strings.HasPrefix(path, "/admin/server/"):
		return "dashboard"
	case path == "/admin/logs":
		return "logs"
	case path == "/admin/mods" || strings.HasPrefix(path, "/admin/mods/"):
		return "mods"
	case path == "/admin/restore-points" || strings.HasPrefix(path, "/admin/restore-points/"),
		path == "/admin/state" || strings.HasPrefix(path, "/admin/state/"):
		return "state"
	case path == "/admin/config" || strings.HasPrefix(path, "/admin/config/"):
		return "config"
	case path == "/admin/staff" || strings.HasPrefix(path, "/admin/staff/"):
		return "staff"
	case path == "/admin/password":
		return "password"
	default:
		return ""
	}
}

// adminFailure returns an HTTP status and an operator-facing message. Ops
// messages are specific and useful to an authenticated admin; unexpected Go
// errors stay generic and are logged separately.
func adminFailure(err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "", ""
	}
	if errors.Is(err, errOpsUnavailable) {
		return http.StatusServiceUnavailable,
			"ops не настроен: управление сервером недоступно.", ""
	}
	var opsErr *OpsError
	if errors.As(err, &opsErr) {
		field := ""
		if opsErr.Details != nil {
			if key, ok := opsErr.Details["key"].(string); ok {
				field = key
			}
		}
		message := opsErr.Message
		if message == "" {
			message = adminMessageForStatus(opsErr.HTTPStatus())
		}
		return opsErr.HTTPStatus(), message, field
	}
	return http.StatusInternalServerError, "Внутренняя ошибка. Подробности в журнале панели.", ""
}

func adminMessageForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Проверьте имя или значение."
	case http.StatusNotFound:
		return "Объект не найден."
	case http.StatusConflict:
		return "Другая операция уже выполняется. Попробуйте позже."
	case http.StatusUnprocessableEntity:
		return "Данные не прошли проверку."
	case http.StatusInsufficientStorage:
		return "Недостаточно свободного места."
	case http.StatusServiceUnavailable:
		return "systemd, journald или unit недоступен."
	default:
		return "Внутренняя ошибка."
	}
}

func adminErrorCode(err error) string {
	var opsErr *OpsError
	if errors.As(err, &opsErr) && opsErr.Code != "" {
		return opsErr.Code
	}
	switch OpsHTTPStatus(err) {
	case http.StatusBadRequest:
		return "invalid_argument"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "locked"
	case http.StatusUnprocessableEntity:
		return "config_invalid"
	case http.StatusInsufficientStorage:
		return "no_space"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal"
	}
}

func (h *panelHandler) ensureOps(w http.ResponseWriter, r *http.Request) bool {
	if h.ops != nil {
		return true
	}
	status, message, _ := adminFailure(errOpsUnavailable)
	writeAPIError(w, status, adminErrorCode(errOpsUnavailable), message)
	return false
}

func (h *panelHandler) ensureOpsPage(w http.ResponseWriter, r *http.Request,
	session sessionState,
) bool {
	if h.ops != nil {
		return true
	}
	status, message, _ := adminFailure(errOpsUnavailable)
	data := h.dashboardData(r, session)
	data.Error = message
	h.render(w, status, "dashboard.html", data)
	return false
}

func (h *panelHandler) handleDashboard(w http.ResponseWriter, r *http.Request, session sessionState) {
	if !validMetricsWindowQuery(r) {
		data := h.dashboardData(r, session)
		data.MetricsError = "Параметр window должен быть одним из: 1h, 6h, 24h."
		h.render(w, http.StatusBadRequest, "dashboard.html", data)
		return
	}
	data := h.dashboardData(r, session)
	h.render(w, http.StatusOK, "dashboard.html", data)
}

func validMetricsWindowQuery(r *http.Request) bool {
	values := r.URL.Query()["window"]
	if len(values) == 0 {
		return true
	}
	if len(values) != 1 {
		return false
	}
	_, ok := metricsWindowSpecFor(values[0])
	return ok
}

func (h *panelHandler) dashboardData(r *http.Request,
	session sessionState,
) dashboardPageData {
	data := dashboardPageData{
		adminPage:   adminPageFor(r, session),
		Status:      "unknown",
		StatusLabel: statusLabel("unknown"),
		Health:      "unknown",
		Timer:       "unknown",
	}
	data.ServerName = defaultServerName
	name, port := h.serverNamePort()
	if name != "" {
		data.ServerName = name
	}
	data.Address = publicAddress(h.cfg, r)
	data.Port = port

	if h.ops == nil {
		data.Status = "service_unavailable"
		data.StatusLabel = statusLabel(data.Status)
		data.StatusError = "ops не настроен: управление сервером недоступно."
	} else {
		status, err := h.ops.Status(r.Context(), "tes3mp.service")
		if err != nil {
			code, message, _ := adminFailure(err)
			data.Status = "service_unavailable"
			data.StatusLabel = statusLabel(data.Status)
			if code == http.StatusServiceUnavailable {
				data.StatusError = message
			} else {
				data.StatusError = adminMessageForStatus(code)
			}
		} else {
			if status.State != "" {
				data.Status = status.State
			}
			data.StatusLabel = statusLabel(data.Status)
			data.Health = status.Health
			if data.Health == "" {
				data.Health = "unknown"
			}
			if status.StartedAt != nil {
				data.StartedAt = *status.StartedAt
			}
		}

		timer, timerErr := h.ops.Status(r.Context(), "tes3mp-easy-backup.timer")
		if timerErr != nil {
			data.Timer = "unknown"
			data.TimerError = "Состояние таймера автобэкапов недоступно."
		} else {
			data.Timer = timer.State
			if data.Timer == "" {
				data.Timer = "unknown"
			}
			if data.Timer != "running" && data.Timer != "starting" {
				data.TimerError = "Автобэкапы не идут: таймер неактивен."
			}
		}

		guard, guardErr := h.ops.Status(r.Context(), "tes3mp-easy-guard.timer")
		if guardErr != nil {
			data.GuardTimer = "unknown"
			data.GuardError = "Состояние guard-таймера недоступно."
		} else {
			data.GuardTimer = guard.State
			if data.GuardTimer == "" {
				data.GuardTimer = "unknown"
			}
			if data.GuardTimer != "running" && data.GuardTimer != "starting" {
				data.GuardError = "Guard не работает: защита от переполнения диска выключена."
			}
		}

		if _, ok := h.ops.(opsDoctor); ok {
			report, doctorErr := h.currentDoctor(r.Context())
			if report.Status != "" || len(report.Checks) > 0 {
				copy := report
				if copy.Checks == nil {
					copy.Checks = []DoctorCheck{}
				}
				data.Doctor = &copy
			}
			if doctorErr != nil {
				_, message, _ := adminFailure(doctorErr)
				data.DoctorError = message
			}
		}
	}

	if current, err := h.readCurrentMods(); err != nil {
		data.ModsError = "Текущий архив модов не найден."
	} else {
		data.Mods = &modsCurrent{
			Name:      current.Name,
			SHA256:    current.SHA256,
			Size:      current.Size,
			SizeLabel: humanSize(current.Size),
		}
	}

	window := r.URL.Query().Get("window")
	if window == "" {
		window = "1h"
	}
	if h.metrics == nil {
		data.MetricsError = "Сбор метрик не запущен."
	} else if metrics, ok := h.metrics.snapshot(window); ok {
		data.Metrics = metrics.view
		data.Window = metrics.Window
	} else {
		data.MetricsError = "Неизвестное окно метрик."
	}

	if h.ops != nil {
		if lines, err := h.ops.Logs(r.Context(), 8); err != nil {
			data.EventsError = "Последние события недоступны."
		} else {
			data.Events = lines
		}
	}
	return data
}

func (h *panelHandler) handleAdminStatus(w http.ResponseWriter, r *http.Request, _ sessionState) {
	if !h.ensureOps(w, r) {
		return
	}
	status, err := h.ops.Status(r.Context(), "tes3mp.service")
	if err != nil {
		code, message, _ := adminFailure(err)
		writeAPIError(w, code, adminErrorCode(err), message)
		return
	}
	timer, timerErr := h.ops.Status(r.Context(), "tes3mp-easy-backup.timer")
	timerState := "unknown"
	if timerErr == nil && timer.State != "" {
		timerState = timer.State
	}
	guard, guardErr := h.ops.Status(r.Context(), "tes3mp-easy-guard.timer")
	guardState := "unknown"
	if guardErr == nil && guard.State != "" {
		guardState = guard.State
	}
	response := struct {
		Status      string  `json:"status"`
		Health      string  `json:"health"`
		StartedAt   *string `json:"started_at"`
		BackupTimer string  `json:"backup_timer"`
		GuardTimer  string  `json:"guard_timer"`
	}{
		Status:      status.State,
		Health:      status.Health,
		StartedAt:   status.StartedAt,
		BackupTimer: timerState,
		GuardTimer:  guardState,
	}
	if response.Status == "" {
		response.Status = "unknown"
	}
	if response.Health == "" {
		response.Health = "unknown"
	}
	writeJSON(w, http.StatusOK, "no-store", response)
}

func (h *panelHandler) handleAdminMetrics(w http.ResponseWriter, r *http.Request,
	_ sessionState,
) {
	if !validMetricsWindowQuery(r) {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument",
			"window must be one of 1h, 6h, 24h")
		return
	}
	window := r.URL.Query().Get("window")
	if h.metrics == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable",
			"metrics collector is not running")
		return
	}
	data, ok := h.metrics.snapshot(window)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument",
			"window must be one of 1h, 6h, 24h")
		return
	}
	writeJSON(w, http.StatusOK, "no-store", struct {
		OK   bool              `json:"ok"`
		Data metricsWindowData `json:"data"`
	}{OK: true, Data: data})
}

func (h *panelHandler) handleServerAction(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	action := r.PathValue("action")
	if action != "start" && action != "stop" && action != "restart" {
		http.NotFound(w, r)
		return
	}
	if h.ops == nil {
		status, message, _ := adminFailure(errOpsUnavailable)
		data := h.dashboardData(r, session)
		data.Error = message
		h.render(w, status, "dashboard.html", data)
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		data := h.dashboardData(r, session)
		data.Error = message
		h.render(w, http.StatusServiceUnavailable, "dashboard.html", data)
		return
	}

	var err error
	switch action {
	case "start":
		err = h.ops.Start(r.Context())
	case "stop":
		err = h.ops.Stop(r.Context())
	case "restart":
		err = h.ops.Restart(r.Context())
	}
	if err != nil {
		status, message, field := adminFailure(err)
		data := h.dashboardData(r, session)
		data.Error = message
		data.ErrorField = field
		h.logger.Printf("audit: action=server.%s result=error code=%s",
			action, adminErrorCode(err))
		h.render(w, status, "dashboard.html", data)
		return
	}
	h.logger.Printf("audit: action=server.%s result=ok", action)
	okCode := map[string]string{
		"start":   "server_started",
		"stop":    "server_stopped",
		"restart": "server_restarted",
	}[action]
	http.Redirect(w, r, "/admin?ok="+okCode, http.StatusSeeOther)
}

func parseLogTail(raw string) (int, error) {
	if raw == "" {
		return defaultLogTail, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maxLogTail {
		return 0, fmt.Errorf("tail must be in 1..%d", maxLogTail)
	}
	return value, nil
}

func (h *panelHandler) handleLogs(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	tail, err := parseLogTail(r.URL.Query().Get("tail"))
	if err != nil {
		data := adminLogsPageData{adminPage: adminPageFor(r, session), Tail: defaultLogTail}
		data.Error = "Параметр tail должен быть числом от 1 до 2000."
		h.render(w, http.StatusBadRequest, "admin_logs.html", data)
		return
	}
	data := adminLogsPageData{adminPage: adminPageFor(r, session), Tail: tail}
	if h.ops == nil {
		status, message, _ := adminFailure(errOpsUnavailable)
		data.Error = message
		h.render(w, status, "admin_logs.html", data)
		return
	}
	lines, err := h.ops.Logs(r.Context(), tail)
	if err != nil {
		status, message, _ := adminFailure(err)
		data.Error = message
		h.render(w, status, "admin_logs.html", data)
		return
	}
	if lines == nil {
		lines = []string{}
	}
	data.Lines = lines
	h.render(w, http.StatusOK, "admin_logs.html", data)
}

func (h *panelHandler) handleLogsAPI(w http.ResponseWriter, r *http.Request, _ sessionState) {
	tail, err := parseLogTail(r.URL.Query().Get("tail"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	if !h.ensureOps(w, r) {
		return
	}
	lines, err := h.ops.Logs(r.Context(), tail)
	if err != nil {
		status, message, _ := adminFailure(err)
		writeAPIError(w, status, adminErrorCode(err), message)
		return
	}
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, "no-store", struct {
		Lines []string `json:"lines"`
	}{Lines: lines})
}

func adminBackupItemFrom(item BackupItem) adminBackupItem {
	mtime, label := backupTime(item.Mtime)
	return adminBackupItem{
		Name:       item.Name,
		Size:       item.Size,
		SizeLabel:  humanSize(item.Size),
		Mtime:      mtime,
		MtimeLabel: label,
		Current:    item.Current,
		Class:      archiveClass(item.Name),
	}
}

func archiveClass(name string) string {
	switch {
	case strings.HasPrefix(name, "checkpoint-predeploy-"):
		return "checkpoint-predeploy"
	case strings.HasPrefix(name, "checkpoint-"):
		return "checkpoint"
	case strings.HasPrefix(name, "snapshot-"):
		return "snapshot"
	case strings.HasPrefix(name, "state-manual-"):
		return "state-manual"
	case strings.HasPrefix(name, "state-predeploy-"):
		return "state-predeploy"
	case strings.HasPrefix(name, "state-"):
		return "state"
	case strings.HasPrefix(name, "mods-init-"):
		return "mods-init"
	case strings.HasPrefix(name, "mods-"):
		return "mods"
	default:
		return ""
	}
}

func (h *panelHandler) modsPageData(r *http.Request,
	session sessionState,
) (adminModsPageData, error) {
	data := adminModsPageData{
		adminPage: adminPageFor(r, session),
		ModsURL:   "/admin/mods",
	}
	if h.ops == nil {
		return data, errOpsUnavailable
	}
	backups, err := h.ops.BackupsList(r.Context(), "mods", adminBackupPageSize, "")
	if err != nil {
		return data, err
	}
	for _, item := range backups.Items {
		data.Items = append(data.Items, adminBackupItemFrom(item))
	}
	data.Empty = len(data.Items) == 0
	if backups.Current != nil {
		data.Current = *backups.Current
	}
	if current, currentErr := h.readCurrentMods(); currentErr == nil {
		data.SHA256 = current.SHA256
		if data.Current == "" {
			data.Current = current.Name
		}
	}
	data.Previous = previousModsArchive(backups)
	return data, nil
}

func previousModsArchive(backups Backups) string {
	current := ""
	if backups.Current != nil {
		current = *backups.Current
	}
	for index, item := range backups.Items {
		isCurrent := item.Current || (current != "" && item.Name == current)
		if !isCurrent {
			continue
		}
		for next := index + 1; next < len(backups.Items); next++ {
			if backups.Items[next].Name != current {
				return backups.Items[next].Name
			}
		}
		return ""
	}
	for _, item := range backups.Items {
		if item.Name != current {
			return item.Name
		}
	}
	return ""
}

func (h *panelHandler) handleMods(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	data, err := h.modsPageData(r, session)
	if err != nil {
		status, message, _ := adminFailure(err)
		data.Error = message
		h.render(w, status, "admin_mods.html", data)
		return
	}
	h.render(w, http.StatusOK, "admin_mods.html", data)
}

func (h *panelHandler) handleModsDeploy(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		status, message, _ := adminFailure(errOpsUnavailable)
		h.renderModsFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderModsFailure(w, r, session, http.StatusBadRequest, "Некорректная форма.", "")
		return
	}
	name := r.PostForm.Get("archive")
	if !safeArchiveName(name) || archive.ValidateModsArchiveName(name) != nil {
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			"Некорректное имя архива модов.", "archive")
		return
	}
	result, err := h.ops.ModsDeploy(r.Context(), name)
	if err != nil {
		status, message, field := adminFailure(err)
		h.logger.Printf("audit: action=mods.deploy archive=%s result=error code=%s",
			name, adminErrorCode(err))
		h.renderModsFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=mods.deploy archive=%s result=ok restarted=%t",
		result.Archive, result.Restarted)
	http.Redirect(w, r, "/admin/mods?ok=mods_deployed", http.StatusSeeOther)
}

func (h *panelHandler) handleModsRollback(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		status, message, _ := adminFailure(errOpsUnavailable)
		h.renderModsFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	backups, err := h.ops.BackupsList(r.Context(), "mods", adminBackupPageSize, "")
	if err != nil {
		status, message, field := adminFailure(err)
		h.renderModsFailure(w, r, session, status, message, field)
		return
	}
	name := previousModsArchive(backups)
	if name == "" {
		h.renderModsFailure(w, r, session, http.StatusConflict,
			"Нет предыдущего архива для отката.", "archive")
		return
	}
	if archive.ValidateModsArchiveName(name) != nil {
		h.renderModsFailure(w, r, session, http.StatusUnprocessableEntity,
			"Предыдущий архив имеет неверное имя.", "archive")
		return
	}
	result, err := h.ops.ModsDeploy(r.Context(), name)
	if err != nil {
		status, message, field := adminFailure(err)
		h.logger.Printf("audit: action=mods.rollback archive=%s result=error code=%s",
			name, adminErrorCode(err))
		h.renderModsFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=mods.rollback archive=%s result=ok restarted=%t",
		result.Archive, result.Restarted)
	http.Redirect(w, r, "/admin/mods?ok=mods_rolled_back", http.StatusSeeOther)
}

func (h *panelHandler) renderModsFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	data, _ := h.modsPageData(r, session)
	data.Error = message
	data.ErrorField = field
	h.render(w, status, "admin_mods.html", data)
}

func (h *panelHandler) handleModsUpload(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не настроен: импорт модов недоступен.", "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			"Ожидалась multipart-форма с архивом.", "file")
		return
	}

	csrfOK := false
	if token := r.Header.Get(csrfHeaderName); token != "" {
		csrfOK = tokensEqual(token, session.CSRF)
	}
	uploadPath := ""
	var uploadSize int64
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			h.removeUpload(uploadPath)
			if isRequestTooLarge(partErr) {
				h.renderModsFailure(w, r, session, http.StatusRequestEntityTooLarge,
					"Архив больше 512 MiB.", "file")
				return
			}
			h.renderModsFailure(w, r, session, http.StatusBadRequest,
				"Не удалось прочитать загружаемый файл.", "file")
			return
		}

		name := part.FormName()
		if name == csrfFormField && part.FileName() == "" {
			token, readErr := readSmallPart(part, 1024)
			if readErr == nil && tokensEqual(string(token), session.CSRF) {
				csrfOK = true
			}
			_ = part.Close()
			continue
		}
		if part.FileName() == "" && name != "file" && name != "archive" {
			_, _ = io.Copy(io.Discard, io.LimitReader(part, maxAdminFormBytes))
			_ = part.Close()
			continue
		}
		if uploadPath != "" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}

		uploadPath, uploadSize, err = h.receiveUpload(part)
		_ = part.Close()
		if err != nil {
			h.removeUpload(uploadPath)
			if isRequestTooLarge(err) {
				h.renderModsFailure(w, r, session, http.StatusRequestEntityTooLarge,
					"Архив больше 512 MiB.", "file")
				return
			}
			h.logger.Printf("panel: mods upload: %v", err)
			h.renderModsFailure(w, r, session, http.StatusInternalServerError,
				"Не удалось сохранить загруженный файл.", "file")
			return
		}
	}

	if !csrfOK {
		h.removeUpload(uploadPath)
		http.Error(w, "CSRF token is missing or invalid", http.StatusForbidden)
		return
	}
	if uploadPath == "" {
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			"В форме нет файла архива.", "file")
		return
	}
	defer h.removeUpload(uploadPath)

	result, err := h.ops.ModsImport(r.Context(), uploadPath)
	if err != nil {
		status, message, field := adminFailure(err)
		h.logger.Printf("audit: action=mods.upload size=%d result=error code=%s",
			uploadSize, adminErrorCode(err))
		h.renderModsFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=mods.upload size=%d archive=%s result=ok",
		uploadSize, result.Archive)
	http.Redirect(w, r, "/admin/mods?ok=mods_uploaded", http.StatusSeeOther)
}

func (h *panelHandler) receiveUpload(part io.Reader) (string, int64, error) {
	dir := filepath.Join(h.cfg.DataDir, "uploads")
	if err := os.MkdirAll(dir, 0o2770); err != nil {
		return "", 0, fmt.Errorf("create uploads directory: %w", err)
	}
	name, err := newUploadName()
	if err != nil {
		return "", 0, err
	}
	target := filepath.Join(dir, name)
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("create upload: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(part, maxModsUploadBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return "", 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return "", 0, closeErr
	}
	if written > maxModsUploadBytes {
		_ = os.Remove(target)
		return "", 0, errUploadTooLarge
	}
	// ops runs as the tes3mp user and must be able to read the upload. The
	// setgid uploads directory supplies the shared group; chmod is explicit so
	// a restrictive umask cannot strip the group-read bit.
	if err := os.Chmod(target, 0o640); err != nil {
		_ = os.Remove(target)
		return "", 0, fmt.Errorf("make upload readable by ops: %w", err)
	}
	return target, written, nil
}

var errUploadTooLarge = errors.New("upload is larger than 512 MiB")

func readSmallPart(part io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errUploadTooLarge
	}
	return data, nil
}

func isRequestTooLarge(err error) bool {
	if errors.Is(err, errUploadTooLarge) {
		return true
	}
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func (h *panelHandler) removeUpload(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		h.logger.Printf("panel: remove upload %s: %v", path, err)
	}
}

func newUploadName() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate upload name: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32] + ".tar.gz", nil
}

func (h *panelHandler) statePageData(r *http.Request, session sessionState,
	before string, limit int,
) (adminStatePageData, error) {
	data := adminStatePageData{
		adminPage: adminPageFor(r, session),
		Limit:     limit,
		StateURL:  "/admin/restore-points",
		ModsURL:   "/admin/mods",
		ServerURL: "/admin",
	}
	if h.ops == nil {
		return data, errOpsUnavailable
	}
	if rp, ok := h.ops.(RestorePointOps); ok {
		return h.restorePointPageData(r, data, rp, before, limit)
	}
	backups, err := h.ops.BackupsList(r.Context(), "state", limit, before)
	if err != nil {
		return data, err
	}
	for _, group := range groupBackups(backups.Items, "state", h.now()) {
		adminGroup := adminBackupGroup{Label: group.Label}
		for _, item := range group.Items {
			adminGroup.Items = append(adminGroup.Items, adminBackupItem{
				Name:       item.Name,
				Size:       item.Size,
				SizeLabel:  item.SizeLabel,
				Mtime:      item.Mtime,
				MtimeLabel: item.MtimeLabel,
				Current:    item.Current,
				Class:      archiveClass(item.Name),
			})
		}
		data.Groups = append(data.Groups, adminGroup)
	}
	data.Empty = len(backups.Items) == 0
	if backups.NextBefore != nil && *backups.NextBefore != "" {
		values := url.Values{}
		values.Set("before", *backups.NextBefore)
		values.Set("limit", strconv.Itoa(limit))
		data.NextURL = "/admin/restore-points?" + values.Encode()
	}
	return data, nil
}

func (h *panelHandler) restorePointPageData(r *http.Request,
	data adminStatePageData, rp RestorePointOps, before string, limit int,
) (adminStatePageData, error) {
	snapshotBefore, checkpointBefore := "", ""
	switch {
	case strings.HasPrefix(before, "snapshot-"):
		snapshotBefore = before
	case strings.HasPrefix(before, "checkpoint-"):
		checkpointBefore = before
	}

	var snapshots SnapshotList
	var checkpoints CheckpointList
	var err error
	switch {
	case snapshotBefore != "":
		snapshots, err = rp.SnapshotsList(r.Context(), limit, snapshotBefore)
	case checkpointBefore != "":
		checkpoints, err = rp.CheckpointsList(r.Context(), limit, checkpointBefore)
	default:
		snapshots, err = rp.SnapshotsList(r.Context(), limit, "")
		if err == nil {
			checkpoints, err = rp.CheckpointsList(r.Context(), limit, "")
		}
	}
	if err != nil {
		return data, err
	}

	snapItem := adminBackupGroup{Label: "Снапшоты"}
	for i, item := range snapshots.Items {
		mtime, label := backupTime(item.Mtime)
		reason := item.Reason
		row := adminBackupItem{
			Name:       item.Name,
			Size:       item.Size,
			SizeLabel:  humanSize(item.Size),
			Mtime:      mtime,
			MtimeLabel: label,
			Class:      "snapshot",
			Reason:     reason,
			Label:      optionalText(item.Label),
			UsedBy:     item.UsedBy,
			CanDelete:  snapshotBefore != "" || i > 0,
			NeedsForce: item.UsedBy > 0,
		}
		snapItem.Items = append(snapItem.Items, row)
	}
	regular := adminBackupGroup{Label: "Чекпоинты"}
	predeploy := adminBackupGroup{Label: "Predeploy-чекпоинты"}
	for _, item := range checkpoints.Items {
		mtime, label := backupTime(item.Mtime)
		row := adminBackupItem{
			Name:          item.Name,
			Size:          item.Size,
			SizeLabel:     humanSize(item.Size),
			Mtime:         mtime,
			MtimeLabel:    label,
			Class:         "checkpoint",
			Reason:        item.Reason,
			Label:         optionalText(item.Label),
			Pinned:        item.Pinned,
			BaseName:      item.Base.Name,
			BaseAvailable: item.Base.Available,
			CanPin:        item.Reason != "predeploy",
			CanDelete:     true,
		}
		if item.Reason == "predeploy" {
			row.Class = "checkpoint-predeploy"
			predeploy.Items = append(predeploy.Items, row)
		} else {
			regular.Items = append(regular.Items, row)
		}
	}
	for _, group := range []adminBackupGroup{regular, predeploy, snapItem} {
		if len(group.Items) > 0 {
			data.Groups = append(data.Groups, group)
		}
	}
	data.Empty = len(data.Groups) == 0
	switch {
	case checkpointBefore != "" && checkpoints.NextBefore != nil:
		data.NextURL = restorePointNextURL(*checkpoints.NextBefore, limit)
	case snapshotBefore != "" && snapshots.NextBefore != nil:
		data.NextURL = restorePointNextURL(*snapshots.NextBefore, limit)
	case checkpointBefore == "" && snapshotBefore == "" &&
		checkpoints.NextBefore != nil && len(snapshots.Items) == 0:
		data.NextURL = restorePointNextURL(*checkpoints.NextBefore, limit)
	case checkpointBefore == "" && snapshotBefore == "" &&
		snapshots.NextBefore != nil && len(checkpoints.Items) == 0:
		data.NextURL = restorePointNextURL(*snapshots.NextBefore, limit)
	}
	return data, nil
}

func restorePointNextURL(before string, limit int) string {
	values := url.Values{}
	values.Set("before", before)
	values.Set("limit", strconv.Itoa(limit))
	return "/admin/restore-points?" + values.Encode()
}

func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (h *panelHandler) redirectToRestorePoints(w http.ResponseWriter, r *http.Request) {
	target := "/admin/restore-points"
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func (h *panelHandler) handleState(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	limit := adminBackupPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxBackupLimit {
			page := adminPageFor(r, session)
			page.Error = "Параметр limit должен быть числом от 1 до 500."
			data := adminStatePageData{
				adminPage: page, Limit: adminBackupPageSize,
				StateURL: "/admin/restore-points", ModsURL: "/admin/mods", ServerURL: "/admin",
			}
			h.render(w, http.StatusBadRequest, "restore_points.html", data)
			return
		}
		limit = parsed
	}
	before := r.URL.Query().Get("before")
	if before != "" && !safeArchiveName(before) {
		page := adminPageFor(r, session)
		page.Error = "Некорректное имя архива в параметре before."
		data := adminStatePageData{
			adminPage: page, Limit: limit,
			StateURL: "/admin/restore-points", ModsURL: "/admin/mods", ServerURL: "/admin",
		}
		h.render(w, http.StatusBadRequest, "restore_points.html", data)
		return
	}
	data, err := h.statePageData(r, session, before, limit)
	if err != nil {
		status, message, _ := adminFailure(err)
		data.Error = message
		h.render(w, status, "restore_points.html", data)
		return
	}
	h.render(w, http.StatusOK, "restore_points.html", data)
}

func (h *panelHandler) handleStateCreate(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не настроен: создание бэкапа недоступно.", "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	action := "state.create"
	switch {
	case strings.Contains(r.URL.Path, "/checkpoint/"):
		action = "checkpoint.create"
	case strings.Contains(r.URL.Path, "/snapshot/"):
		action = "snapshot.create"
	}
	var archive string
	rp, hasRestorePoints := h.ops.(RestorePointOps)
	switch {
	case hasRestorePoints && action == "checkpoint.create":
		result, err := rp.CheckpointCreate(r.Context())
		if err != nil {
			h.renderStateOpsFailure(w, r, session, action, err)
			return
		}
		archive = result.Archive
	case hasRestorePoints && action == "snapshot.create":
		result, err := rp.SnapshotCreate(r.Context())
		if err != nil {
			h.renderStateOpsFailure(w, r, session, action, err)
			return
		}
		archive = result.Archive
	case hasRestorePoints:
		result, err := rp.CheckpointCreate(r.Context())
		if err != nil {
			h.renderStateOpsFailure(w, r, session, action, err)
			return
		}
		archive = result.Archive
	default:
		backup, err := h.ops.BackupCreate(r.Context(), "state", true)
		if err != nil {
			h.renderStateOpsFailure(w, r, session, action, err)
			return
		}
		archive = backup.Archive
	}
	h.logger.Printf("audit: action=%s archive=%s result=ok", action, archive)
	http.Redirect(w, r, "/admin/restore-points?ok=backup_created", http.StatusSeeOther)
}

func (h *panelHandler) renderStateOpsFailure(w http.ResponseWriter,
	r *http.Request, session sessionState, action string, err error,
) {
	status, message, field := adminFailure(err)
	h.logger.Printf("audit: action=%s result=error code=%s",
		action, adminErrorCode(err))
	h.renderStateFailure(w, r, session, status, message, field)
}

func (h *panelHandler) handleStateDeploy(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не настроен: восстановление недоступно.", "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректная форма.", "")
		return
	}
	name := r.PostForm.Get("archive")
	if !safeArchiveName(name) {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректное имя архива.", "archive")
		return
	}
	rp, hasRestorePoints := h.ops.(RestorePointOps)
	var archiveName string
	var restarted bool
	action := "state.deploy"
	if hasRestorePoints {
		parsed, err := restorepoint.ParseName(name)
		if err != nil {
			h.renderStateFailure(w, r, session, http.StatusBadRequest,
				"Некорректное имя точки отката.", "archive")
			return
		}
		var result RestoreResult
		var restoreErr error
		if parsed.Kind == restorepoint.KindSnapshot {
			action = "snapshot.restore"
			result, restoreErr = rp.SnapshotRestore(r.Context(), name)
		} else {
			action = "checkpoint.restore"
			result, restoreErr = rp.CheckpointRestore(r.Context(), name,
				r.PostForm.Get("accept_mismatch") != "")
		}
		if restoreErr != nil {
			h.logger.Printf("audit: action=%s archive=%s result=error code=%s",
				action, name, adminErrorCode(restoreErr))
			status, message, field := adminFailure(restoreErr)
			h.renderStateFailure(w, r, session, status, message, field)
			return
		}
		archiveName = result.Archive
		restarted = result.Restarted
	} else {
		if archive.ValidateStateArchiveName(name) != nil {
			h.renderStateFailure(w, r, session, http.StatusBadRequest,
				"Некорректное имя state-архива.", "archive")
			return
		}
		result, err := h.ops.StateDeploy(r.Context(), name)
		if err != nil {
			h.logger.Printf("audit: action=state.deploy archive=%s result=error code=%s",
				name, adminErrorCode(err))
			status, message, field := adminFailure(err)
			h.renderStateFailure(w, r, session, status, message, field)
			return
		}
		archiveName = result.Archive
		restarted = result.Restarted
	}
	h.logger.Printf("audit: action=%s archive=%s result=ok restarted=%t",
		action, archiveName, restarted)
	http.Redirect(w, r, "/admin/restore-points?ok=state_deployed", http.StatusSeeOther)
}

func (h *panelHandler) renderStateFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	data, _ := h.statePageData(r, session, "", adminBackupPageSize)
	data.Error = message
	data.ErrorField = field
	h.render(w, status, "restore_points.html", data)
}

func (h *panelHandler) handleStatePin(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не настроен: закрепление недоступно.", "")
		return
	}
	rp, ok := h.ops.(RestorePointOps)
	if !ok {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не поддерживает закрепление чекпоинтов.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректная форма.", "")
		return
	}
	name := r.PostForm.Get("archive")
	if !safeArchiveName(name) {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректное имя архива.", "archive")
		return
	}
	parsed, err := restorepoint.ParseName(name)
	if err != nil || parsed.Kind != restorepoint.KindCheckpoint {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Закрепить можно только обычный чекпоинт.", "archive")
		return
	}
	pinned := false
	switch r.PostForm.Get("pin_action") {
	case "pin":
		pinned = true
	case "unpin":
	default:
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Неизвестное действие закрепления.", "pin_action")
		return
	}
	if err := rp.CheckpointPin(r.Context(), name, pinned); err != nil {
		h.renderStateOpsFailure(w, r, session,
			"checkpoint.pin", err)
		return
	}
	result := "checkpoint_unpinned"
	if pinned {
		result = "checkpoint_pinned"
	}
	h.logger.Printf("audit: action=checkpoint.pin archive=%s pinned=%t result=ok",
		name, pinned)
	http.Redirect(w, r, "/admin/restore-points?ok="+result, http.StatusSeeOther)
}

func (h *panelHandler) handleStateDelete(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не настроен: удаление недоступно.", "")
		return
	}
	rp, ok := h.ops.(RestorePointOps)
	if !ok {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			"ops не поддерживает удаление точек отката.", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректная форма.", "")
		return
	}
	name := r.PostForm.Get("archive")
	confirm := r.PostForm.Get("confirm")
	if !safeArchiveName(name) {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректное имя архива.", "archive")
		return
	}
	if confirm != name {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Введите имя архива целиком, чтобы подтвердить удаление.", "confirm")
		return
	}
	parsed, err := restorepoint.ParseName(name)
	if err != nil {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			"Некорректное имя точки отката.", "archive")
		return
	}
	force := r.PostForm.Get("force") != "" &&
		parsed.Kind == restorepoint.KindSnapshot
	action := "checkpoint.rm"
	var result RestorePointRemoveResult
	if parsed.Kind == restorepoint.KindSnapshot {
		action = "snapshot.rm"
		result, err = rp.SnapshotRemove(r.Context(), name, force)
	} else {
		result, err = rp.CheckpointRemove(r.Context(), name, force)
	}
	if err != nil {
		h.logger.Printf("audit: action=%s archive=%s force=%t result=error code=%s",
			action, name, force, adminErrorCode(err))
		status, message, field := adminFailure(err)
		if adminErrorCode(err) == ops.CodeArchiveInUse {
			message += " Снимите закрепление или повторите с «Удалить вместе с базой»."
		}
		h.renderStateFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=%s archive=%s force=%t result=ok freed=%d",
		action, name, force, result.Freed)

	data, pageErr := h.statePageData(r, session, "", adminBackupPageSize)
	if pageErr != nil {
		status, message, field := adminFailure(pageErr)
		data.Error = message
		data.ErrorField = field
		h.render(w, status, "restore_points.html", data)
		return
	}
	data.Notice = fmt.Sprintf("Удалено: %s. Освобождено %s.",
		strings.Join(result.Removed, ", "), humanSize(result.Freed))
	data.Warnings = append(data.Warnings, result.Warnings...)
	for _, skipped := range result.Skipped {
		data.Warnings = append(data.Warnings, fmt.Sprintf(
			"%s не удалён: %s", skipped.Name, skipped.Reason))
	}
	h.render(w, http.StatusOK, "restore_points.html", data)
}

type adminConfigFieldDef struct {
	Key   string
	Label string
	Kind  string
}

var adminConfigFields = map[string][]adminConfigFieldDef{
	"server": {
		{Key: "hostname", Label: "Имя сервера", Kind: "string"},
		{Key: "port", Label: "Порт (1024..65535)", Kind: "int"},
		{Key: "maximumPlayers", Label: "Максимум игроков (1..64)", Kind: "int"},
		{Key: "password", Label: "Пароль сервера (пусто = без пароля)", Kind: "password"},
		{Key: "logLevel", Label: "Уровень логов (0..4)", Kind: "int"},
		{Key: "masterServerEnabled", Label: "Master server", Kind: "bool"},
	},
	"config": {
		{Key: "gameMode", Label: "Game mode", Kind: "string"},
		{Key: "loginTime", Label: "loginTime (10..600)", Kind: "int"},
		{Key: "maxClientsPerIP", Label: "maxClientsPerIP (1..10)", Kind: "int"},
		{Key: "difficulty", Label: "difficulty (-100..100)", Kind: "int"},
		{Key: "passTimeWhenEmpty", Label: "passTimeWhenEmpty", Kind: "bool"},
		{Key: "nightStartHour", Label: "nightStartHour (0..23)", Kind: "int"},
		{Key: "nightEndHour", Label: "nightEndHour (0..23)", Kind: "int"},
		{Key: "allowConsole", Label: "allowConsole", Kind: "bool"},
		{Key: "allowBedRest", Label: "allowBedRest", Kind: "bool"},
		{Key: "allowWildernessRest", Label: "allowWildernessRest", Kind: "bool"},
		{Key: "allowWait", Label: "allowWait", Kind: "bool"},
		{Key: "allowSuicideCommand", Label: "allowSuicideCommand", Kind: "bool"},
		{Key: "allowFixmeCommand", Label: "allowFixmeCommand", Kind: "bool"},
		{Key: "shareJournal", Label: "shareJournal", Kind: "bool"},
		{Key: "shareFactionRanks", Label: "shareFactionRanks", Kind: "bool"},
		{Key: "shareTopics", Label: "shareTopics", Kind: "bool"},
		{Key: "shareBounty", Label: "shareBounty", Kind: "bool"},
		{Key: "shareReputation", Label: "shareReputation", Kind: "bool"},
		{Key: "enforceDataFiles", Label: "enforceDataFiles", Kind: "bool"},
	},
}

func (h *panelHandler) configSnapshot(ctx context.Context,
	file string,
) (ConfigSnapshot, error) {
	if h.ops == nil {
		return ConfigSnapshot{}, errOpsUnavailable
	}
	if snapshots, ok := h.ops.(opsSnapshotter); ok {
		snapshot, err := snapshots.ConfigSnapshot(ctx, file)
		if err != nil {
			return ConfigSnapshot{}, err
		}
		if snapshot.Values == nil {
			snapshot.Values = map[string]any{}
		}
		return snapshot, nil
	}
	values, err := h.ops.ConfigGet(ctx, file)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	if values == nil {
		values = map[string]any{}
	}
	return ConfigSnapshot{File: file, Values: values}, nil
}

func (h *panelHandler) configSections(r *http.Request) ([]adminConfigSection, error) {
	sections := make([]adminConfigSection, 0, 3)
	for _, item := range []struct {
		file  string
		title string
		note  string
	}{
		{file: "server", title: "Сервер"},
		{file: "config", title: "Игровые настройки", note: "Таблицы и списки показываются только для чтения."},
		{file: "banlist", title: "Банлист"},
	} {
		snapshot, err := h.configSnapshot(r.Context(), item.file)
		if err != nil {
			return nil, err
		}
		section := adminConfigSection{
			File:      item.file,
			Title:     item.title,
			Note:      item.note,
			ReadOnly:  snapshot.ReadOnly,
			SaveLabel: "Сохранить",
		}
		if item.file == "banlist" {
			for _, list := range []struct {
				key   string
				label string
			}{
				{key: "playerNames", label: "Имена игроков (по одному на строку)"},
				{key: "ipAddresses", label: "IP-адреса (по одному на строку)"},
			} {
				section.Banlist = append(section.Banlist, adminConfigList{
					Key:   list.key,
					Label: list.label,
					Value: strings.Join(stringList(snapshot.Values[list.key]), "\n"),
				})
			}
		} else {
			for _, def := range adminConfigFields[item.file] {
				value, exists := snapshot.Values[def.Key]
				if !exists {
					continue
				}
				field := adminConfigField{
					Key:   def.Key,
					Label: def.Label,
					Kind:  def.Kind,
					Value: fmt.Sprint(value),
				}
				if def.Kind == "bool" {
					field.Checked, _ = value.(bool)
				}
				section.Fields = append(section.Fields, field)
			}
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, fmt.Sprint(item))
		}
		return result
	default:
		return nil
	}
}

func adminConfigKind(file, key string) string {
	for _, def := range adminConfigFields[file] {
		if def.Key == key {
			return def.Kind
		}
	}
	return ""
}

func (h *panelHandler) handleConfig(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	data := adminConfigPageData{adminPage: adminPageFor(r, session)}
	sections, err := h.configSections(r)
	if err != nil {
		status, message, _ := adminFailure(err)
		data.Error = message
		h.render(w, status, "admin_config.html", data)
		return
	}
	data.Sections = sections
	h.render(w, http.StatusOK, "admin_config.html", data)
}

func (h *panelHandler) handleConfigPost(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	file := r.PathValue("file")
	if _, ok := adminConfigFields[file]; !ok && file != "banlist" {
		http.NotFound(w, r)
		return
	}
	if h.ops == nil {
		data := adminConfigPageData{adminPage: adminPageFor(r, session)}
		status, message, _ := adminFailure(errOpsUnavailable)
		data.Error = message
		h.render(w, status, "admin_config.html", data)
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderConfigFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxConfigFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderConfigFailure(w, r, session, http.StatusBadRequest,
			"Некорректная форма.", "")
		return
	}
	if _, legacy := r.PostForm["key"]; !legacy {
		h.handleConfigForm(w, r, session, file)
		return
	}
	key := r.PostForm.Get("key")
	if key == "" {
		h.renderConfigFailure(w, r, session, http.StatusBadRequest,
			"Не указан ключ настройки.", "")
		return
	}
	values, hasValue := r.PostForm["value"]
	raw := ""
	if len(values) > 0 {
		raw = values[0]
	}
	if !hasValue {
		if adminConfigKind(file, key) == "bool" {
			raw = "false"
		} else {
			h.renderConfigFailure(w, r, session, http.StatusBadRequest,
				"Не указано значение настройки.", key)
			return
		}
	}
	if err := h.ops.ConfigSet(r.Context(), file, key, raw); err != nil {
		status, message, field := adminFailure(err)
		if field == "" {
			field = key
		}
		h.logger.Printf("audit: action=config.set file=%s key=%s result=error code=%s",
			file, key, adminErrorCode(err))
		h.renderConfigFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=config.set file=%s key=%s result=ok", file, key)

	data := adminConfigPageData{adminPage: adminPageFor(r, session)}
	data.Notice = "Настройка сохранена."
	data.RestartRequired = true
	sections, err := h.configSections(r)
	if err != nil {
		status, message, _ := adminFailure(err)
		data.Error = message
		data.Sections = sections
		h.render(w, status, "admin_config.html", data)
		return
	}
	data.Sections = sections
	h.render(w, http.StatusOK, "admin_config.html", data)
}

func (h *panelHandler) handleConfigForm(w http.ResponseWriter, r *http.Request,
	session sessionState, file string,
) {
	snapshot, err := h.configSnapshot(r.Context(), file)
	if err != nil {
		status, message, _ := adminFailure(err)
		h.renderConfigFailure(w, r, session, status, message, "")
		return
	}

	changed := make(map[string]any)
	if file == "banlist" {
		for _, key := range []string{"playerNames", "ipAddresses"} {
			raw, ok := r.PostForm[key]
			if !ok {
				continue
			}
			value := normalizeListValue(raw[0])
			if strings.Join(stringList(snapshot.Values[key]), "\n") != value {
				changed[key] = value
			}
		}
	} else {
		for _, def := range adminConfigFields[file] {
			raw, ok := r.PostForm[def.Key]
			if !ok {
				if def.Kind == "bool" {
					raw = []string{"false"}
				} else {
					continue
				}
			}
			value, changedField, parseErr := configFormValue(def.Kind, raw[0],
				snapshot.Values[def.Key])
			if parseErr != nil {
				h.renderConfigFailure(w, r, session, http.StatusBadRequest,
					"Проверьте значение поля "+def.Label+".", def.Key)
				return
			}
			if changedField {
				changed[def.Key] = value
			}
		}
	}

	if len(changed) == 0 {
		data := adminConfigPageData{adminPage: adminPageFor(r, session)}
		data.Notice = "Изменений нет."
		data.Sections, _ = h.configSections(r)
		h.render(w, http.StatusOK, "admin_config.html", data)
		return
	}

	if batch, ok := h.ops.(opsConfigBatch); ok {
		if err := batch.ConfigSetMany(r.Context(), file, changed); err != nil {
			status, message, field := adminFailure(err)
			h.logger.Printf("audit: action=config.set file=%s keys=%d result=error code=%s",
				file, len(changed), adminErrorCode(err))
			h.renderConfigFailure(w, r, session, status, message, field)
			return
		}
	} else {
		for key, value := range changed {
			if err := h.ops.ConfigSet(r.Context(), file, key, value); err != nil {
				status, message, field := adminFailure(err)
				if field == "" {
					field = key
				}
				h.logger.Printf("audit: action=config.set file=%s key=%s result=error code=%s",
					file, key, adminErrorCode(err))
				h.renderConfigFailure(w, r, session, status, message, field)
				return
			}
			h.logger.Printf("audit: action=config.set file=%s key=%s result=ok", file, key)
		}
	}

	data := adminConfigPageData{adminPage: adminPageFor(r, session)}
	data.Notice = fmt.Sprintf("Сохранено полей: %d.", len(changed))
	data.RestartRequired = true
	data.Sections, err = h.configSections(r)
	if err != nil {
		status, message, _ := adminFailure(err)
		data.Error = message
		h.render(w, status, "admin_config.html", data)
		return
	}
	h.render(w, http.StatusOK, "admin_config.html", data)
}

func normalizeListValue(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimRight(value, "\n")
}

func configFormValue(kind, raw string, current any) (any, bool, error) {
	switch kind {
	case "bool":
		value := raw == "true" || raw == "1" || raw == "on"
		existing, _ := current.(bool)
		return value, value != existing, nil
	case "int":
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, false, err
		}
		existing := 0
		switch typed := current.(type) {
		case float64:
			existing = int(typed)
		case int:
			existing = typed
		}
		return value, value != existing, nil
	case "password":
		existing, _ := current.(string)
		return raw, raw != existing, nil
	default:
		existing := fmt.Sprint(current)
		return raw, raw != existing, nil
	}
}

func (h *panelHandler) renderConfigFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	data := adminConfigPageData{adminPage: adminPageFor(r, session)}
	sections, _ := h.configSections(r)
	data.Sections = sections
	data.Error = message
	data.ErrorField = field
	for index := range data.Sections {
		if field == "" {
			continue
		}
		for _, item := range data.Sections[index].Fields {
			if item.Key == field {
				data.Sections[index].Error = item.Label
			}
		}
		for _, item := range data.Sections[index].Banlist {
			if item.Key == field {
				data.Sections[index].Error = item.Label
			}
		}
	}
	h.render(w, status, "admin_config.html", data)
}

func (h *panelHandler) handleStaff(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	players, err := readStaffPlayers(h.cfg.Root)
	data := adminStaffPageData{adminPage: adminPageFor(r, session), Players: players}
	if err != nil {
		h.logger.Printf("panel: read staff players: %v", err)
		data.Error = "Не удалось прочитать список игроков."
		h.render(w, http.StatusInternalServerError, "admin_staff.html", data)
		return
	}
	data.Empty = len(players) == 0
	h.render(w, http.StatusOK, "admin_staff.html", data)
}

func (h *panelHandler) handleStaffSet(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	player := r.PathValue("player")
	if !validPlayerName(player) {
		h.renderStaffFailure(w, r, session, http.StatusBadRequest,
			"Некорректное имя игрока.", "player")
		return
	}
	if h.ops == nil {
		status, message, _ := adminFailure(errOpsUnavailable)
		h.renderStaffFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStaffFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderStaffFailure(w, r, session, http.StatusBadRequest,
			"Некорректная форма.", "")
		return
	}
	rank, err := strconv.Atoi(r.PostForm.Get("rank"))
	if err != nil || rank < 0 || rank > 3 {
		h.renderStaffFailure(w, r, session, http.StatusBadRequest,
			"Ранг должен быть числом от 0 до 3.", "rank")
		return
	}
	if err := h.ops.StaffSet(r.Context(), player, rank); err != nil {
		status, message, field := adminFailure(err)
		h.logger.Printf("audit: action=staff.set player=%s rank=%d result=error code=%s",
			player, rank, adminErrorCode(err))
		h.renderStaffFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=staff.set player=%s rank=%d result=ok", player, rank)
	http.Redirect(w, r, "/admin/staff?ok=staff_saved&restart=1", http.StatusSeeOther)
}

func (h *panelHandler) renderStaffFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	players, _ := readStaffPlayers(h.cfg.Root)
	data := adminStaffPageData{
		adminPage: adminPageFor(r, session),
		Players:   players,
		Empty:     len(players) == 0,
	}
	data.Error = message
	data.ErrorField = field
	h.render(w, status, "admin_staff.html", data)
}

func validPlayerName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 255 {
		return false
	}
	if strings.ContainsAny(name, "/\\\x00\r\n") || strings.Contains(name, "..") {
		return false
	}
	return true
}

func readStaffPlayers(root string) ([]adminStaffPlayer, error) {
	dir := filepath.Join(root, "players")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []adminStaffPlayer{}, nil
	}
	if err != nil {
		return nil, err
	}
	players := make([]adminStaffPlayer, 0, len(entries))
	for _, entry := range entries {
		if len(players) >= maxStaffPlayers {
			break
		}
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") ||
			strings.HasPrefix(name, ".") {
			continue
		}
		info, err := os.Lstat(filepath.Join(dir, name))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
			info.Size() > maxPlayerFileBytes {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var raw struct {
			Name     string `json:"name"`
			Settings struct {
				StaffRank int `json:"staffRank"`
			} `json:"settings"`
		}
		fileBase := strings.TrimSuffix(name, ".json")
		player := adminStaffPlayer{
			Name:   fileBase,
			Rank:   -1,
			SetURL: "/admin/staff/" + url.PathEscape(fileBase),
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			player.Error = "повреждённый JSON"
			player.RankText = "?"
		} else {
			if raw.Name != "" {
				player.Name = raw.Name
			}
			player.Rank = raw.Settings.StaffRank
			player.RankText = staffRankText(player.Rank)
			if player.Rank < 0 || player.Rank > 3 {
				player.Rank = -1
				player.RankText = "?"
				player.Error = "некорректный ранг"
			}
		}
		players = append(players, player)
	}
	sort.Slice(players, func(i, j int) bool {
		return strings.ToLower(players[i].Name) < strings.ToLower(players[j].Name)
	})
	return players, nil
}

func staffRankText(rank int) string {
	switch rank {
	case 0:
		return "0 — игрок"
	case 1:
		return "1 — модератор"
	case 2:
		return "2 — админ"
	case 3:
		return "3 — владелец"
	default:
		return "неизвестно"
	}
}
