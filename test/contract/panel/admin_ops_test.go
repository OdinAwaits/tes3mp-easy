package panel

import (
	"net/http"
	"net/url"
	"testing"
)

func TestPanelAdminServerActionsContract(t *testing.T) {
	p := startPanel(t, nil)
	session := loginPanel(t, p, "panel contract password")

	resp := p.request(t, session.Client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin")
	for _, action := range []string{"start", "stop", "restart"} {
		requireContains(t, resp.Body, `action="/admin/server/`+action+`"`)
	}

	resp = p.request(t, session.Client, http.MethodGet, "/api/v1/admin/status", nil, nil)
	expectJSON(t, resp, http.StatusOK)
	requireContains(t, resp.Body, `"status":"running"`)
	requireContains(t, resp.Body, `"health":"healthy"`)
	requireContains(t, resp.Body, `"backup_timer":"running"`)

	resp = p.request(t, session.Client, http.MethodGet, "/api/v1/admin/logs?tail=2", nil, nil)
	expectJSON(t, resp, http.StatusOK)
	requireContains(t, resp.Body, `"lines":["first line","second line"]`)

	resp = p.request(t, session.Client, http.MethodGet, "/api/v1/admin/logs?tail=0", nil, nil)
	expectJSON(t, resp, http.StatusBadRequest)
	requireContains(t, resp.Body, `"code":"invalid_argument"`)

	actions := []struct {
		action string
		ok     string
	}{
		{action: "start", ok: "server_started"},
		{action: "stop", ok: "server_stopped"},
		{action: "restart", ok: "server_restarted"},
	}
	for _, action := range actions {
		resp = p.request(t, session.Client, http.MethodPost,
			"/admin/server/"+action.action, url.Values{
				"_csrf": {session.CSRF},
			}, nil)
		requireStatus(t, resp, http.StatusSeeOther,
			"POST /admin/server/"+action.action)
		requireLocation(t, resp, "/admin?ok="+action.ok)
		requireOpsCall(t, readOpsCalls(t, p.opsLogPath()),
			"-n", "-u", "tes3mp", "/opt/tes3mp-easy/bin/tes3mp-easy-ops",
			"--root", p.root, "--json", action.action)
	}

	logText := p.logContents()
	for _, action := range actions {
		requireContains(t, logText, "audit: action=server."+action.action+" result=ok")
	}

	// Unsafe admin actions require the session CSRF token and only known
	// start/stop/restart routes exist.
	resp = p.request(t, session.Client, http.MethodPost, "/admin/server/start",
		url.Values{"_csrf": {"wrong"}}, nil)
	requireStatus(t, resp, http.StatusForbidden, "POST server start with bad CSRF")

	resp = p.request(t, session.Client, http.MethodPost, "/admin/server/reload",
		url.Values{"_csrf": {session.CSRF}}, nil)
	requireStatus(t, resp, http.StatusNotFound, "POST server reload")

	// Ops failures keep the operator on the dashboard with the ops message.
	writeFakeMode(t, p.modePath, "fail")
	resp = p.request(t, session.Client, http.MethodPost, "/admin/server/start",
		url.Values{"_csrf": {session.CSRF}}, nil)
	requireStatus(t, resp, http.StatusServiceUnavailable, "POST server start while failing")
	requireContains(t, resp.Body, "systemd is not running")
	requireContains(t, resp.Body, `action="/admin/server/start"`)
	requireContains(t, p.logContents(),
		"audit: action=server.start result=error code=service_unavailable")

	writeFakeMode(t, p.modePath, "locked")
	resp = p.request(t, session.Client, http.MethodPost, "/admin/server/restart",
		url.Values{"_csrf": {session.CSRF}}, nil)
	requireStatus(t, resp, http.StatusConflict, "POST server restart while locked")
	requireContains(t, resp.Body, "another operation is running")
	requireContains(t, p.logContents(),
		"audit: action=server.restart result=error code=locked")

	writeFakeMode(t, p.modePath, "ok")
}
