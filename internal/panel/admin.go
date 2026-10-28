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
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/serverruntime"
)

const (
	defaultLogTail     = 200
	maxLogTail         = 2000
	maxAdminFormBytes  = 64 << 10
	maxConfigFormBytes = 1 << 20
	maxModsUploadBytes = 512 << 20
	maxUploadBodyBytes = maxModsUploadBytes + (1 << 20)
	// A restore bundle carries one state and one mods tar.gz. Each member is
	// limited independently by archive.MaxCompressedSize.
	maxRestoreUploadBytes = 2*archive.MaxCompressedSize + (16 << 20)
	maxRestoreBodyBytes   = maxRestoreUploadBytes + (1 << 20)
	maxPlayerFileBytes    = 4 << 20
	maxStaffPlayers       = 1000
	adminBackupPageSize   = 200
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

// currentDoctor caches `ops doctor` for one minute so mutations do not spawn a
// subprocess for every request and can refuse operations that doctor already
// marked as doomed.
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
		return true, h.tr().T(i18n.KeyPanelErrorDoctorBlocked)
	}
	return true, h.tr().Tf(i18n.KeyPanelErrorDoctorBlockedDetails,
		strings.Join(failed, "; "))
}

type adminPage struct {
	pageI18n
	CSRF            string
	ActiveNav       string
	Path            string
	Nav             string
	Notice          string
	Error           string
	ErrorField      string
	RestartRequired bool

	// PanelVersion, ServerVersion and ClientVersion are the three versions the
	// admin header shows on every page. ServerVersion is "—" when the install
	// marker of the runtime is absent, so the header keeps its shape.
	PanelVersion  string
	ServerVersion string
	ClientVersion string
}

type dashboardPageData struct {
	adminPage
	ServerName     string
	Address        string
	Port           int
	Status         string
	StatusLabel    string
	Health         string
	HealthLabel    string
	StartedAt      string
	StartedAtLabel string
	CanStart       bool
	CanStop        bool
	StatusError    string
	Events         []string
	EventsError    string
	Mods           *modsCurrent
	ModsError      string
	Metrics        metricsView
	MetricsError   string
	Window         string
}

type adminLogsPageData struct {
	adminPage
	Lines []string
	Tail  int
}

type modsCurrent struct {
	Name      string
	SizeLabel string
}

type adminModsPageData struct {
	adminPage
	Items    []archiveItem
	Current  string
	Previous string
	Empty    bool
}

type adminStatePageData struct {
	adminPage
	Groups      []archiveGroup
	NextURL     string
	Empty       bool
	Limit       int
	CurrentMods string
	MissingMods bool
}

// adminBackupsPageData renders snapshots and mod packs on one page: the two
// used to live behind separate nav items and unclear ordering rules.
type adminBackupsPageData struct {
	adminPage
	State adminStatePageData
	Mods  adminModsPageData
}

type adminConfigPageData struct {
	adminPage
	Sections []adminConfigSection
}

type adminConfigSection struct {
	File   string
	Title  string
	Fields []adminConfigField
	Note   string
	Error  string
}

type adminConfigField struct {
	Key     string
	Label   string
	Kind    string
	Value   string
	Checked bool
}

// adminPlayersPageData merges two former pages: staff ranks and the banlist.
// Both answer the same question, "who may play on this server", so they now
// live next to each other.
type adminPlayersPageData struct {
	adminPage
	Players   []adminPlayer
	Empty     bool
	BanNames  string
	BanIPs    string
	BanError  string
	BanListOK bool
}

type adminPlayer struct {
	Name    string
	File    string
	Rank    int
	Banned  bool
	RankURL string
	BanURL  string
	Error   string
}

var adminNoticeMessages = map[string]string{
	"server_started":   i18n.KeyPanelNoticeServerStarted,
	"server_stopped":   i18n.KeyPanelNoticeServerStopped,
	"server_restarted": i18n.KeyPanelNoticeServerRestart,
	"backup_created":   i18n.KeyPanelNoticeBackupCreated,
	"mods_uploaded":    i18n.KeyPanelNoticeModsUploaded,
	"mods_deployed":    i18n.KeyPanelNoticeModsDeployed,
	"mods_rolled_back": i18n.KeyPanelNoticeModsRolledBack,
	"state_deployed":   i18n.KeyPanelNoticeStateDeployed,
	"state_restored":   i18n.KeyPanelNoticeStateRestored,
	"staff_saved":      i18n.KeyPanelNoticeStaffSaved,
	"ban_saved":        i18n.KeyPanelNoticeBanSaved,
}

var adminRestartNotices = map[string]bool{
	"staff_saved": true,
	"ban_saved":   true,
}

func (h *panelHandler) adminPageFor(r *http.Request, session sessionState) adminPage {
	query := r.URL.Query()
	status := query.Get("ok")
	localized := h.pageI18n()
	page := adminPage{
		pageI18n:  localized,
		CSRF:      session.CSRF,
		ActiveNav: adminActiveNav(r.URL.Path),
		Path:      r.URL.Path,
		Nav:       "admin",
		Notice:    localized.T(adminNoticeMessages[status]),
	}
	page.RestartRequired = query.Get("restart") == "1" || adminRestartNotices[status]
	page.PanelVersion = Version
	page.ClientVersion = ClientVersion
	page.ServerVersion = h.serverVersion()
	return page
}

// serverVersion reads the TES3MP release from the runtime install marker. A
// missing or unreadable marker is not an error for the page: the header shows a
// dash, because an older runtime without the marker is a valid install.
func (h *panelHandler) serverVersion() string {
	info, err := serverruntime.ReadInfo(h.cfg.RuntimeRoot)
	if err != nil || info.Version == "" {
		return "—"
	}
	return info.Version
}

// adminFailure returns an HTTP status and an operator-facing message. Ops
// messages are specific and useful to an authenticated admin; unexpected Go
// errors stay generic and are logged separately.
func (h *panelHandler) adminFailure(err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "", ""
	}
	if errors.Is(err, errOpsUnavailable) {
		return http.StatusServiceUnavailable,
			h.tr().T(i18n.KeyPanelErrorOpsUnavailable), ""
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
			message = h.adminMessageForStatus(opsErr.HTTPStatus())
		}
		return opsErr.HTTPStatus(), message, field
	}
	return http.StatusInternalServerError, h.tr().T(i18n.KeyPanelErrorInternal), ""
}

func (h *panelHandler) adminMessageForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return h.tr().T(i18n.KeyPanelErrorInvalidArgument)
	case http.StatusNotFound:
		return h.tr().T(i18n.KeyPanelErrorNotFound)
	case http.StatusConflict:
		return h.tr().T(i18n.KeyPanelErrorLocked)
	case http.StatusUnprocessableEntity:
		return h.tr().T(i18n.KeyPanelErrorValidation)
	case http.StatusInsufficientStorage:
		return h.tr().T(i18n.KeyPanelErrorNoSpace)
	case http.StatusServiceUnavailable:
		return h.tr().T(i18n.KeyPanelErrorServiceUnavailable)
	default:
		return h.tr().T(i18n.KeyPanelErrorInternalShort)
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
	status, message, _ := h.adminFailure(errOpsUnavailable)
	writeAPIError(w, status, adminErrorCode(errOpsUnavailable), message)
	return false
}

func (h *panelHandler) handleDashboard(w http.ResponseWriter, r *http.Request, session sessionState) {
	if !validMetricsWindowQuery(r) {
		data := h.dashboardData(r, session)
		data.MetricsError = h.tr().T(i18n.KeyPanelAdminWindowInvalid)
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
		adminPage:   h.adminPageFor(r, session),
		Status:      "unknown",
		StatusLabel: statusLabel(h.tr(), "unknown"),
		Health:      "unknown",
		HealthLabel: healthLabel(h.tr(), "unknown"),
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
		data.StatusLabel = statusLabel(h.tr(), data.Status)
		data.StatusError = h.tr().T(i18n.KeyPanelErrorOpsUnavailable)
	} else {
		status, err := h.ops.Status(r.Context(), "tes3mp.service")
		if err != nil {
			code, message, _ := h.adminFailure(err)
			data.Status = "service_unavailable"
			data.StatusLabel = statusLabel(h.tr(), data.Status)
			if code == http.StatusServiceUnavailable {
				data.StatusError = message
			} else {
				data.StatusError = h.adminMessageForStatus(code)
			}
		} else {
			if status.State != "" {
				data.Status = status.State
			}
			data.StatusLabel = statusLabel(h.tr(), data.Status)
			data.Health = status.Health
			if data.Health == "" {
				data.Health = "unknown"
			}
			if status.StartedAt != nil {
				data.StartedAt = *status.StartedAt
			}
		}

	}

	data.HealthLabel = healthLabel(h.tr(), data.Health)
	if data.StartedAt != "" {
		data.StartedAtLabel = timeLabel(h.tr(), data.StartedAt, h.now())
	}
	data.CanStart, data.CanStop = serverActions(data.Status)

	if current, err := h.readCurrentMods(); err != nil {
		data.ModsError = h.tr().T(i18n.KeyPanelAdminModsMissing)
	} else {
		data.Mods = &modsCurrent{
			Name:      current.Name,
			SizeLabel: humanSize(current.Size),
		}
	}

	window := r.URL.Query().Get("window")
	if window == "" {
		window = "1h"
	}
	if h.metrics == nil {
		data.MetricsError = h.tr().T(i18n.KeyPanelAdminMetricsStopped)
	} else if metrics, ok := h.metrics.snapshot(window); ok {
		data.Metrics = metrics.view
		data.Window = metrics.Window
	} else {
		data.MetricsError = h.tr().T(i18n.KeyPanelAdminMetricsWindow)
	}

	if h.ops != nil {
		if lines, err := h.ops.Logs(r.Context(), 8); err != nil {
			data.EventsError = h.tr().T(i18n.KeyPanelAdminEventsUnavailable)
		} else {
			data.Events = lines
		}
	}
	return data
}

// serverActions maps the ops state to the buttons that make sense. "starting"
// counts as running: systemd already accepted the start job, so a second start
// would only fail and stop is still meaningful.
func serverActions(status string) (canStart, canStop bool) {
	switch status {
	case "running", "starting":
		return false, true
	default:
		return true, false
	}
}

func (h *panelHandler) handleAdminStatus(w http.ResponseWriter, r *http.Request, _ sessionState) {
	if !h.ensureOps(w, r) {
		return
	}
	status, err := h.ops.Status(r.Context(), "tes3mp.service")
	if err != nil {
		code, message, _ := h.adminFailure(err)
		writeAPIError(w, code, adminErrorCode(err), message)
		return
	}
	timer, timerErr := h.ops.Status(r.Context(), "tes3mp-easy-backup.timer")
	timerState := "unknown"
	if timerErr == nil && timer.State != "" {
		timerState = timer.State
	}
	response := struct {
		Status         string  `json:"status"`
		StatusLabel    string  `json:"status_label"`
		Health         string  `json:"health"`
		HealthLabel    string  `json:"health_label"`
		StartedAt      *string `json:"started_at"`
		StartedAtLabel *string `json:"started_at_label"`
		CanStart       bool    `json:"can_start"`
		CanStop        bool    `json:"can_stop"`
		BackupTimer    string  `json:"backup_timer"`
	}{
		Status:      status.State,
		Health:      status.Health,
		StartedAt:   status.StartedAt,
		BackupTimer: timerState,
	}
	if response.Status == "" {
		response.Status = "unknown"
	}
	if response.Health == "" {
		response.Health = "unknown"
	}
	response.StatusLabel = statusLabel(h.tr(), response.Status)
	response.HealthLabel = healthLabel(h.tr(), response.Health)
	if response.StartedAt != nil {
		label := timeLabel(h.tr(), *response.StartedAt, h.now())
		response.StartedAtLabel = &label
	}
	response.CanStart, response.CanStop = serverActions(response.Status)
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
		status, message, _ := h.adminFailure(errOpsUnavailable)
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
		status, message, field := h.adminFailure(err)
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
		data := adminLogsPageData{adminPage: h.adminPageFor(r, session), Tail: defaultLogTail}
		data.Error = h.tr().T(i18n.KeyPanelAdminTailInvalid)
		h.render(w, http.StatusBadRequest, "admin_logs.html", data)
		return
	}
	data := adminLogsPageData{adminPage: h.adminPageFor(r, session), Tail: tail}
	if h.ops == nil {
		status, message, _ := h.adminFailure(errOpsUnavailable)
		data.Error = message
		h.render(w, status, "admin_logs.html", data)
		return
	}
	lines, err := h.ops.Logs(r.Context(), tail)
	if err != nil {
		status, message, _ := h.adminFailure(err)
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
		status, message, _ := h.adminFailure(err)
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

func (h *panelHandler) modsPageData(r *http.Request,
	session sessionState,
) (adminModsPageData, error) {
	data := adminModsPageData{adminPage: h.adminPageFor(r, session)}
	if h.ops == nil {
		return data, errOpsUnavailable
	}
	backups, err := h.ops.BackupsList(r.Context(), "mods", adminBackupPageSize, "")
	if err != nil {
		return data, err
	}
	if backups.Current != nil {
		data.Current = *backups.Current
	}
	if current, currentErr := h.readCurrentMods(); currentErr == nil {
		if data.Current == "" {
			data.Current = current.Name
		}
	}
	data.Items = archiveItems(h.tr(), backups.Items, "mods", archiveOptions{
		Admin:       true,
		CurrentMods: data.Current,
	}, h.now())
	data.Empty = len(data.Items) == 0
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
	_ sessionState,
) {
	redirectPreservingQuery(w, r, "/admin/backups")
}

// redirectPreservingQuery keeps notices (ok=, restart=) and pagination on the
// way to a merged page, so an old bookmark or a POST redirect still lands on
// the right state of the new page.
func redirectPreservingQuery(w http.ResponseWriter, r *http.Request, target string) {
	if query := r.URL.RawQuery; query != "" {
		target += "?" + query
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (h *panelHandler) handleModsDeploy(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		status, message, _ := h.adminFailure(errOpsUnavailable)
		h.renderModsFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderModsFailure(w, r, session, http.StatusBadRequest, h.tr().T(i18n.KeyPanelErrorBadForm), "")
		return
	}
	name := r.PostForm.Get("archive")
	if !safeArchiveName(name) || archive.ValidateModsArchiveName(name) != nil {
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminModsNameInvalid), "archive")
		return
	}
	result, err := h.ops.ModsDeploy(r.Context(), name)
	if err != nil {
		status, message, field := h.adminFailure(err)
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
		status, message, _ := h.adminFailure(errOpsUnavailable)
		h.renderModsFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	backups, err := h.ops.BackupsList(r.Context(), "mods", adminBackupPageSize, "")
	if err != nil {
		status, message, field := h.adminFailure(err)
		h.renderModsFailure(w, r, session, status, message, field)
		return
	}
	name := previousModsArchive(backups)
	if name == "" {
		h.renderModsFailure(w, r, session, http.StatusConflict,
			h.tr().T(i18n.KeyPanelAdminNoPrevious), "archive")
		return
	}
	if archive.ValidateModsArchiveName(name) != nil {
		h.renderModsFailure(w, r, session, http.StatusUnprocessableEntity,
			h.tr().T(i18n.KeyPanelAdminPreviousInvalid), "archive")
		return
	}
	result, err := h.ops.ModsDeploy(r.Context(), name)
	if err != nil {
		status, message, field := h.adminFailure(err)
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
	h.renderBackupsFailure(w, r, session, status, message, field)
}

func (h *panelHandler) handleModsUpload(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable,
			h.tr().T(i18n.KeyPanelAdminModsImportUnavail), "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderModsFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	upload, err := h.receiveAdminUpload(r, session.CSRF, maxModsUploadBytes)
	if err != nil {
		h.renderModsUploadFailure(w, r, session, err)
		return
	}
	if !upload.CSRF {
		h.removeUpload(upload.Path)
		http.Error(w, "CSRF token is missing or invalid", http.StatusForbidden)
		return
	}
	if upload.Path == "" {
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminFileMissing), "file")
		return
	}
	defer h.removeUpload(upload.Path)

	result, err := h.ops.ModsImport(r.Context(), upload.Path)
	if err != nil {
		status, message, field := h.adminFailure(err)
		h.logger.Printf("audit: action=mods.upload size=%d result=error code=%s",
			upload.Size, adminErrorCode(err))
		h.renderModsFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=mods.upload size=%d archive=%s result=ok",
		upload.Size, result.Archive)
	http.Redirect(w, r, "/admin/mods?ok=mods_uploaded", http.StatusSeeOther)
}

// adminUpload is one file streamed out of a multipart request. Path is a
// private file in panel-data/uploads, not the browser-supplied name.
type adminUpload struct {
	Path     string
	FileName string
	Size     int64
	CSRF     bool
}

var (
	errMultipartUpload = errors.New("multipart upload is required")
	errNoUploadFile    = errors.New("multipart request has no file")
)

// receiveAdminUpload streams the first file part to uploads and validates the
// CSRF field while reading. It is deliberately independent of the destination
// handler so mods and state restore share the same upload rules.
func (h *panelHandler) receiveAdminUpload(r *http.Request, csrf string,
	maxBytes int64,
) (adminUpload, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return adminUpload{}, errMultipartUpload
	}

	upload := adminUpload{}
	if token := r.Header.Get(csrfHeaderName); token != "" {
		upload.CSRF = tokensEqual(token, csrf)
	}
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			h.removeUpload(upload.Path)
			return adminUpload{}, partErr
		}

		name := part.FormName()
		if name == csrfFormField && part.FileName() == "" {
			token, readErr := readSmallPart(part, 1024)
			if readErr == nil {
				upload.CSRF = tokensEqual(string(token), csrf)
			}
			_ = part.Close()
			continue
		}
		if part.FileName() == "" {
			_, _ = io.Copy(io.Discard, io.LimitReader(part, maxAdminFormBytes))
			_ = part.Close()
			continue
		}
		if upload.Path != "" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}

		upload.FileName = safeUploadFileName(part.FileName())
		upload.Path, upload.Size, err = h.receiveUploadWithLimit(part, maxBytes)
		_ = part.Close()
		if err != nil {
			h.removeUpload(upload.Path)
			return adminUpload{}, err
		}
	}
	if upload.Path == "" && upload.CSRF {
		return upload, errNoUploadFile
	}
	return upload, nil
}

func safeUploadFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	return filepath.Base(name)
}

func (h *panelHandler) renderModsUploadFailure(w http.ResponseWriter,
	r *http.Request, session sessionState, err error,
) {
	switch {
	case errors.Is(err, errMultipartUpload):
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminMultipartExpected), "file")
	case errors.Is(err, errNoUploadFile):
		h.renderModsFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminFileMissing), "file")
	case isRequestTooLarge(err):
		h.renderModsFailure(w, r, session, http.StatusRequestEntityTooLarge,
			h.tr().T(i18n.KeyPanelAdminModsTooLarge), "file")
	default:
		h.logger.Printf("panel: mods upload: %v", err)
		h.renderModsFailure(w, r, session, http.StatusInternalServerError,
			h.tr().T(i18n.KeyPanelAdminUploadFailed), "file")
	}
}

func (h *panelHandler) receiveUploadWithLimit(part io.Reader, maxBytes int64) (string, int64, error) {
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
	written, copyErr := io.Copy(file, io.LimitReader(part, maxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return "", 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return "", 0, closeErr
	}
	if written > maxBytes {
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

var errUploadTooLarge = errors.New("upload is larger than the configured limit")

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
		adminPage: h.adminPageFor(r, session),
		Limit:     limit,
	}
	if h.ops == nil {
		return data, errOpsUnavailable
	}

	mods, modsErr := h.ops.BackupsList(r.Context(), "mods", 1, "")
	backups, err := h.ops.BackupsList(r.Context(), "state", limit, before)
	if err != nil {
		return data, err
	}
	currentMods := ""
	if modsErr == nil && mods.Current != nil {
		currentMods = *mods.Current
	}
	data.CurrentMods = currentMods
	data.MissingMods = currentMods == ""

	now := h.now()
	data.Groups = groupArchiveItems(h.tr(), archiveItems(h.tr(), backups.Items, "state", archiveOptions{
		Admin:       true,
		CurrentMods: currentMods,
	}, now), now)
	data.Empty = len(backups.Items) == 0
	if backups.NextBefore != nil && *backups.NextBefore != "" {
		values := url.Values{}
		values.Set("before", *backups.NextBefore)
		values.Set("limit", strconv.Itoa(limit))
		data.NextURL = "/admin/backups?" + values.Encode()
	}
	return data, nil
}

// backupsPageData loads both halves of the merged page. A failure in one half
// still renders the other, so a broken mods list never hides snapshots.
func (h *panelHandler) backupsPageData(r *http.Request, session sessionState,
	before string, limit int,
) (adminBackupsPageData, error) {
	state, stateErr := h.statePageData(r, session, before, limit)
	mods, modsErr := h.modsPageData(r, session)
	data := adminBackupsPageData{adminPage: state.adminPage, State: state, Mods: mods}
	if stateErr != nil {
		return data, stateErr
	}
	return data, modsErr
}

func (h *panelHandler) handleAdminBackups(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	limit := adminBackupPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxBackupLimit {
			data, _ := h.backupsPageData(r, session, "", adminBackupPageSize)
			data.Error = h.tr().T(i18n.KeyPanelAdminLimitInvalid)
			h.render(w, http.StatusBadRequest, "admin_backups.html", data)
			return
		}
		limit = parsed
	}
	before := r.URL.Query().Get("before")
	if before != "" && !safeArchiveName(before) {
		data, _ := h.backupsPageData(r, session, "", limit)
		data.Error = h.tr().T(i18n.KeyPanelAdminBeforeInvalid)
		h.render(w, http.StatusBadRequest, "admin_backups.html", data)
		return
	}
	data, err := h.backupsPageData(r, session, before, limit)
	if err != nil {
		status, message, _ := h.adminFailure(err)
		data.Error = message
		h.render(w, status, "admin_backups.html", data)
		return
	}
	h.render(w, http.StatusOK, "admin_backups.html", data)
}

func (h *panelHandler) handleState(w http.ResponseWriter, r *http.Request,
	_ sessionState,
) {
	redirectPreservingQuery(w, r, "/admin/backups")
}

func (h *panelHandler) renderBackupsFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	data, _ := h.backupsPageData(r, session, "", adminBackupPageSize)
	data.Error = message
	data.ErrorField = field
	h.render(w, status, "admin_backups.html", data)
}

func (h *panelHandler) handleStateCreate(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			h.tr().T(i18n.KeyPanelAdminBackupUnavailable), "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	backup, err := h.ops.BackupCreate(r.Context(), "state", true)
	if err != nil {
		h.logger.Printf("audit: action=state.create result=error code=%s",
			adminErrorCode(err))
		status, message, field := h.adminFailure(err)
		h.renderStateFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=state.create archive=%s result=ok", backup.Archive)
	http.Redirect(w, r, "/admin/state?ok=backup_created", http.StatusSeeOther)
}

func (h *panelHandler) handleStateDeploy(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable,
			h.tr().T(i18n.KeyPanelAdminRestoreUnavail), "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStateFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelErrorBadForm), "")
		return
	}
	name := r.PostForm.Get("archive")
	if !safeArchiveName(name) || archive.ValidateStateArchiveName(name) != nil {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminStateNameInvalid), "archive")
		return
	}
	// The state row sends the mod pack the snapshot was created with. If that
	// pack is not active yet the panel deploys it first, so restoring stays a
	// single click instead of "deploy mods, then restore".
	mods := r.PostForm.Get("mods")
	if mods != "" && (!safeArchiveName(mods) || archive.ValidateModsArchiveName(mods) != nil) {
		h.renderStateFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminModsNameInvalid), "mods")
		return
	}
	if mods != "" {
		current, listErr := h.ops.BackupsList(r.Context(), "mods", 1, "")
		if listErr != nil {
			status, message, field := h.adminFailure(listErr)
			h.renderStateFailure(w, r, session, status, message, field)
			return
		}
		active := ""
		if current.Current != nil {
			active = *current.Current
		}
		if active != mods {
			deployed, deployErr := h.ops.ModsDeploy(r.Context(), mods)
			if deployErr != nil {
				h.logger.Printf("audit: action=mods.deploy archive=%s reason=restore result=error code=%s",
					mods, adminErrorCode(deployErr))
				status, message, field := h.adminFailure(deployErr)
				h.renderStateFailure(w, r, session, status, message, field)
				return
			}
			h.logger.Printf("audit: action=mods.deploy archive=%s reason=restore result=ok restarted=%t",
				deployed.Archive, deployed.Restarted)
		}
	}
	result, err := h.ops.StateDeploy(r.Context(), name)
	if err != nil {
		h.logger.Printf("audit: action=state.deploy archive=%s result=error code=%s",
			name, adminErrorCode(err))
		status, message, field := h.adminFailure(err)
		h.renderStateFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=state.deploy archive=%s mods=%s result=ok restarted=%t",
		result.Archive, mods, result.Restarted)
	http.Redirect(w, r, "/admin/state?ok=state_deployed", http.StatusSeeOther)
}

func (h *panelHandler) renderStateFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	h.renderBackupsFailure(w, r, session, status, message, field)
}

func (h *panelHandler) handleStateDownload(w http.ResponseWriter, r *http.Request,
	_ sessionState,
) {
	h.serveAdminArchive(w, r, "state")
}

func (h *panelHandler) handleModsDownload(w http.ResponseWriter, r *http.Request,
	_ sessionState,
) {
	h.serveAdminArchive(w, r, "mods")
}

func (h *panelHandler) serveAdminArchive(w http.ResponseWriter, r *http.Request,
	typ string,
) {
	name := r.PathValue("name")
	if !safeArchiveName(name) {
		http.NotFound(w, r)
		return
	}
	valid := archive.ValidateModsArchiveName(name) == nil
	if typ == "state" {
		valid = archive.ValidateStateArchiveName(name) == nil
	}
	if !valid {
		http.NotFound(w, r)
		return
	}

	target := filepath.Join(h.cfg.Root, "backups", typ, name)
	linkInfo, err := os.Lstat(target)
	if err != nil || linkInfo.Mode()&os.ModeSymlink != 0 ||
		!linkInfo.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	h.private(w)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", contentDisposition(name))
	http.ServeContent(w, r, name, info.ModTime(), file)
}

type adminConfigFieldDef struct {
	Key   string
	Label string
	// LabelKey wins over Label: technical keys such as loginTime stay as they
	// are, operator-facing labels are translated.
	LabelKey string
	Kind     string
}

var adminConfigFields = map[string][]adminConfigFieldDef{
	"server": {
		{Key: "hostname", LabelKey: i18n.KeyPanelConfigHostname, Kind: "string"},
		{Key: "port", LabelKey: i18n.KeyPanelConfigPort, Kind: "int"},
		{Key: "maximumPlayers", LabelKey: i18n.KeyPanelConfigMaxPlayers, Kind: "int"},
		{Key: "password", LabelKey: i18n.KeyPanelConfigPassword, Kind: "password"},
		{Key: "logLevel", LabelKey: i18n.KeyPanelConfigLogLevel, Kind: "int"},
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

func configFieldLabel(tr *i18n.Translator, def adminConfigFieldDef) string {
	if def.LabelKey != "" {
		return tr.T(def.LabelKey)
	}
	return def.Label
}

func (h *panelHandler) configSections(r *http.Request) ([]adminConfigSection, error) {
	sections := make([]adminConfigSection, 0, 2)
	for _, item := range []struct {
		file     string
		titleKey string
		noteKey  string
	}{
		{file: "server", titleKey: i18n.KeyPanelConfigSectionServer},
		{file: "config", titleKey: i18n.KeyPanelConfigSectionGame,
			noteKey: i18n.KeyPanelConfigGameNote},
	} {
		snapshot, err := h.configSnapshot(r.Context(), item.file)
		if err != nil {
			return nil, err
		}
		section := adminConfigSection{
			File:  item.file,
			Title: h.tr().T(item.titleKey),
			Note:  h.tr().T(item.noteKey),
		}
		for _, def := range adminConfigFields[item.file] {
			value, exists := snapshot.Values[def.Key]
			if !exists {
				continue
			}
			field := adminConfigField{
				Key:   def.Key,
				Label: configFieldLabel(h.tr(), def),
				Kind:  def.Kind,
				Value: fmt.Sprint(value),
			}
			if def.Kind == "bool" {
				field.Checked, _ = value.(bool)
			}
			section.Fields = append(section.Fields, field)
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

func (h *panelHandler) handleConfig(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	data := adminConfigPageData{adminPage: h.adminPageFor(r, session)}
	sections, err := h.configSections(r)
	if err != nil {
		status, message, _ := h.adminFailure(err)
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
	if _, ok := adminConfigFields[file]; !ok {
		http.NotFound(w, r)
		return
	}
	if h.ops == nil {
		data := adminConfigPageData{adminPage: h.adminPageFor(r, session)}
		status, message, _ := h.adminFailure(errOpsUnavailable)
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
			h.tr().T(i18n.KeyPanelErrorBadForm), "")
		return
	}
	h.handleConfigForm(w, r, session, file)
}

func (h *panelHandler) handleConfigForm(w http.ResponseWriter, r *http.Request,
	session sessionState, file string,
) {
	snapshot, err := h.configSnapshot(r.Context(), file)
	if err != nil {
		status, message, _ := h.adminFailure(err)
		h.renderConfigFailure(w, r, session, status, message, "")
		return
	}

	changed := make(map[string]any)
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
				h.tr().Tf(i18n.KeyPanelAdminFieldInvalid, def.Label), def.Key)
			return
		}
		if changedField {
			changed[def.Key] = value
		}
	}

	if len(changed) == 0 {
		data := adminConfigPageData{adminPage: h.adminPageFor(r, session)}
		data.Notice = h.tr().T(i18n.KeyPanelCommonNoChanges)
		data.Sections, _ = h.configSections(r)
		h.render(w, http.StatusOK, "admin_config.html", data)
		return
	}

	keys := make([]string, 0, len(changed))
	for key := range changed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := h.ops.ConfigSet(r.Context(), file, key, changed[key]); err != nil {
			status, message, field := h.adminFailure(err)
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

	data := adminConfigPageData{adminPage: h.adminPageFor(r, session)}
	data.Notice = h.tr().Tf(i18n.KeyPanelAdminSavedFields, len(changed))
	data.RestartRequired = true
	data.Sections, err = h.configSections(r)
	if err != nil {
		status, message, _ := h.adminFailure(err)
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
	data := adminConfigPageData{adminPage: h.adminPageFor(r, session)}
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
	}
	h.render(w, status, "admin_config.html", data)
}

func (h *panelHandler) handleStaff(w http.ResponseWriter, r *http.Request,
	_ sessionState,
) {
	// Kept as a redirect: staff ranks are part of the players page now.
	redirectPreservingQuery(w, r, "/admin/players")
}

func (h *panelHandler) handlePlayers(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	data := h.playersPageData(r, session)
	status := http.StatusOK
	if data.Error != "" {
		status = http.StatusInternalServerError
	}
	h.render(w, status, "admin_players.html", data)
}

func (h *panelHandler) playersPageData(r *http.Request,
	session sessionState,
) adminPlayersPageData {
	data := adminPlayersPageData{adminPage: h.adminPageFor(r, session)}
	players, err := readPlayers(h.tr(), h.cfg.Root)
	if err != nil {
		h.logger.Printf("panel: read players: %v", err)
		data.Error = h.tr().T(i18n.KeyPanelAdminPlayersUnreadable)
		return data
	}
	data.Players = players
	data.Empty = len(players) == 0

	if h.ops == nil {
		data.BanError = h.tr().T(i18n.KeyPanelErrorOpsUnavailable)
		return data
	}
	snapshot, err := h.configSnapshot(r.Context(), "banlist")
	if err != nil {
		_, message, _ := h.adminFailure(err)
		h.logger.Printf("panel: read banlist: %v", err)
		data.BanError = message
		return data
	}
	names := stringList(snapshot.Values["playerNames"])
	data.BanNames = strings.Join(names, "\n")
	data.BanIPs = strings.Join(stringList(snapshot.Values["ipAddresses"]), "\n")
	data.BanListOK = true
	for index := range data.Players {
		player := &data.Players[index]
		player.Banned = containsFold(names, player.Name) || containsFold(names, player.File)
	}
	return data
}

func (h *panelHandler) renderPlayersFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	data := h.playersPageData(r, session)
	data.Error = message
	data.ErrorField = field
	h.render(w, status, "admin_players.html", data)
}

// handlePlayersBans saves the banlist form as a whole, exactly like the
// settings page: only changed keys are written.
func (h *panelHandler) handlePlayersBans(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	if h.ops == nil {
		status, message, _ := h.adminFailure(errOpsUnavailable)
		h.renderPlayersFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderPlayersFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxConfigFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderPlayersFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelErrorBadForm), "")
		return
	}
	snapshot, err := h.configSnapshot(r.Context(), "banlist")
	if err != nil {
		status, message, _ := h.adminFailure(err)
		h.renderPlayersFailure(w, r, session, status, message, "")
		return
	}
	keys, err := h.saveBanlist(w, r, session, snapshot,
		[]string{"playerNames", "ipAddresses"})
	if err != nil {
		return
	}
	if len(keys) == 0 {
		data := h.playersPageData(r, session)
		data.Notice = h.tr().T(i18n.KeyPanelCommonNoChanges)
		h.render(w, http.StatusOK, "admin_players.html", data)
		return
	}
	http.Redirect(w, r, "/admin/players?ok=ban_saved&restart=1", http.StatusSeeOther)
}

// handlePlayerBan toggles one name in the banlist. It writes the whole
// playerNames list because `config set` replaces the value of a key.
func (h *panelHandler) handlePlayerBan(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	player := r.PathValue("player")
	if !validPlayerName(player) {
		h.renderPlayersFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminPlayerInvalid), "player")
		return
	}
	if h.ops == nil {
		status, message, _ := h.adminFailure(errOpsUnavailable)
		h.renderPlayersFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderPlayersFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderPlayersFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelErrorBadForm), "")
		return
	}
	banned := r.PostForm.Get("banned") == "1"
	snapshot, err := h.configSnapshot(r.Context(), "banlist")
	if err != nil {
		status, message, _ := h.adminFailure(err)
		h.renderPlayersFailure(w, r, session, status, message, "")
		return
	}
	names := stringList(snapshot.Values["playerNames"])
	next := make([]string, 0, len(names)+1)
	for _, name := range names {
		if !strings.EqualFold(name, player) {
			next = append(next, name)
		}
	}
	if banned {
		next = append(next, player)
	}
	if strings.Join(next, "\n") == strings.Join(names, "\n") {
		redirectPreservingQuery(w, r, "/admin/players")
		return
	}
	if err := h.ops.ConfigSet(r.Context(), "banlist", "playerNames",
		strings.Join(next, "\n")); err != nil {
		status, message, field := h.adminFailure(err)
		if field == "" {
			field = "playerNames"
		}
		h.logger.Printf("audit: action=player.ban player=%s banned=%t result=error code=%s",
			player, banned, adminErrorCode(err))
		h.renderPlayersFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=player.ban player=%s banned=%t result=ok", player, banned)
	http.Redirect(w, r, "/admin/players?ok=ban_saved&restart=1", http.StatusSeeOther)
}

// saveBanlist writes the posted banlist keys that actually changed. It renders
// the failure page itself, so callers only have to stop on a non-nil error.
func (h *panelHandler) saveBanlist(w http.ResponseWriter, r *http.Request,
	session sessionState, snapshot ConfigSnapshot, keys []string,
) ([]string, error) {
	changed := make(map[string]string)
	for _, key := range keys {
		raw, ok := r.PostForm[key]
		if !ok {
			continue
		}
		value := normalizeListValue(raw[0])
		if strings.Join(stringList(snapshot.Values[key]), "\n") != value {
			changed[key] = value
		}
	}
	names := make([]string, 0, len(changed))
	for key := range changed {
		names = append(names, key)
	}
	sort.Strings(names)
	for _, key := range names {
		if err := h.ops.ConfigSet(r.Context(), "banlist", key, changed[key]); err != nil {
			status, message, field := h.adminFailure(err)
			if field == "" {
				field = key
			}
			h.logger.Printf("audit: action=config.set file=banlist key=%s result=error code=%s",
				key, adminErrorCode(err))
			h.renderPlayersFailure(w, r, session, status, message, field)
			return nil, err
		}
		h.logger.Printf("audit: action=config.set file=banlist key=%s result=ok", key)
	}
	return names, nil
}

func containsFold(list []string, value string) bool {
	if value == "" {
		return false
	}
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func (h *panelHandler) handleStaffSet(w http.ResponseWriter, r *http.Request,
	session sessionState,
) {
	player := r.PathValue("player")
	if !validPlayerName(player) {
		h.renderStaffFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminPlayerInvalid), "player")
		return
	}
	if h.ops == nil {
		status, message, _ := h.adminFailure(errOpsUnavailable)
		h.renderStaffFailure(w, r, session, status, message, "")
		return
	}
	if blocked, message := h.doctorMutationError(r.Context()); blocked {
		h.renderStaffFailure(w, r, session, http.StatusServiceUnavailable, message, "")
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderStaffFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelErrorBadForm), "")
		return
	}
	rank, err := strconv.Atoi(r.PostForm.Get("rank"))
	if err != nil || rank < 0 || rank > 3 {
		h.renderStaffFailure(w, r, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAdminRankInvalid), "rank")
		return
	}
	if err := h.ops.StaffSet(r.Context(), player, rank); err != nil {
		status, message, field := h.adminFailure(err)
		h.logger.Printf("audit: action=staff.set player=%s rank=%d result=error code=%s",
			player, rank, adminErrorCode(err))
		h.renderStaffFailure(w, r, session, status, message, field)
		return
	}
	h.logger.Printf("audit: action=staff.set player=%s rank=%d result=ok", player, rank)
	http.Redirect(w, r, "/admin/players?ok=staff_saved&restart=1", http.StatusSeeOther)
}

func (h *panelHandler) renderStaffFailure(w http.ResponseWriter, r *http.Request,
	session sessionState,
	status int, message, field string,
) {
	h.renderPlayersFailure(w, r, session, status, message, field)
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

func readPlayers(tr *i18n.Translator, root string) ([]adminPlayer, error) {
	dir := filepath.Join(root, "players")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []adminPlayer{}, nil
	}
	if err != nil {
		return nil, err
	}
	players := make([]adminPlayer, 0, len(entries))
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
		player := adminPlayer{
			Name:    fileBase,
			File:    fileBase,
			Rank:    -1,
			RankURL: "/admin/staff/" + url.PathEscape(fileBase),
			BanURL:  "/admin/players/" + url.PathEscape(fileBase) + "/ban",
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			player.Error = tr.T(i18n.KeyPanelStaffBadJSON)
		} else {
			if raw.Name != "" {
				player.Name = raw.Name
			}
			player.Rank = raw.Settings.StaffRank
			if player.Rank < 0 || player.Rank > 3 {
				player.Rank = -1
				player.Error = tr.T(i18n.KeyPanelStaffBadRank)
			}
		}
		players = append(players, player)
	}
	sort.Slice(players, func(i, j int) bool {
		return strings.ToLower(players[i].Name) < strings.ToLower(players[j].Name)
	})
	return players, nil
}
