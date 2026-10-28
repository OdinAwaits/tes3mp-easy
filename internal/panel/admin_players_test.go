package panel

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

func TestAdminPlayersPageShowsBansAndToggles(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		configSnapshots: adminConfigSnapshots(),
	})
	writePlayerFile(t, env.cfg.Root, "Trouble Maker",
		`{"name":"Trouble Maker","settings":{"staffRank":0}}`)
	writePlayerFile(t, env.cfg.Root, "Jane",
		`{"name":"Jane","settings":{"staffRank":2}}`)

	rec := env.request(t, http.MethodGet, "/admin/players", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/players = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Trouble Maker", "Jane", `data-status="failed"`, `data-status="ok"`,
		"/admin/players/Jane/ban", "203.0.113.44",
		`action="/admin/players/bans"`, "Save banlist",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("players page does not contain %q:\n%s", want, body)
		}
	}

	rec = env.request(t, http.MethodPost, "/admin/players/Jane/ban",
		url.Values{"banned": {"1"}})
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/players?ok=ban_saved&restart=1" {
		t.Fatalf("ban = %d %q: %s", rec.Code, rec.Header().Get("Location"),
			rec.Body.String())
	}
	if len(env.ops.configSets) != 1 ||
		env.ops.configSets[0].file != "banlist" ||
		env.ops.configSets[0].key != "playerNames" ||
		env.ops.configSets[0].value != "Trouble Maker\nJane" {
		t.Fatalf("ban calls = %+v", env.ops.configSets)
	}

	env.ops.configSnapshots["banlist"].Values["playerNames"] =
		[]string{"Trouble Maker", "Jane"}
	env.ops.configSets = nil
	rec = env.request(t, http.MethodPost, "/admin/players/Trouble%20Maker/ban",
		url.Values{"banned": {"0"}})
	if rec.Code != http.StatusSeeOther || len(env.ops.configSets) != 1 ||
		env.ops.configSets[0].value != "Jane" {
		t.Fatalf("unban = %d calls=%+v", rec.Code, env.ops.configSets)
	}

	// Unbanning a name that is not listed is a no-op, not a config write.
	env.ops.configSets = nil
	rec = env.request(t, http.MethodPost, "/admin/players/Ghost/ban",
		url.Values{"banned": {"0"}})
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/players" ||
		len(env.ops.configSets) != 0 {
		t.Fatalf("no-op unban = %d %q calls=%+v", rec.Code,
			rec.Header().Get("Location"), env.ops.configSets)
	}
}

func TestAdminPlayersBanlistFormSavesDiff(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		configSnapshots: adminConfigSnapshots(),
	})
	form := url.Values{
		"playerNames": {"Trouble Maker\nJane"},
		"ipAddresses": {"203.0.113.44"},
	}
	rec := env.request(t, http.MethodPost, "/admin/players/bans", form)
	if rec.Code != http.StatusSeeOther || len(env.ops.configSets) != 1 ||
		env.ops.configSets[0].key != "playerNames" ||
		env.ops.configSets[0].value != "Trouble Maker\nJane" {
		t.Fatalf("banlist form = %d calls=%+v", rec.Code, env.ops.configSets)
	}

	env.ops.configSnapshots["banlist"].Values["playerNames"] =
		[]string{"Trouble Maker", "Jane"}
	env.ops.configSets = nil
	rec = env.request(t, http.MethodPost, "/admin/players/bans", form)
	if rec.Code != http.StatusOK || len(env.ops.configSets) != 0 ||
		!strings.Contains(rec.Body.String(), "No changes") {
		t.Fatalf("unchanged banlist = %d calls=%+v", rec.Code, env.ops.configSets)
	}
}

func TestAdminPlayerBanFailureKeepsReason(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		configSnapshots: adminConfigSnapshots(),
	})
	env.ops.configSetErr = adminError(ops.CodeConfigInvalid, ops.ExitConfigInvalid,
		"banlist.json: playerNames must be a list", map[string]any{"key": "playerNames"})

	rec := env.request(t, http.MethodPost, "/admin/players/Jane/ban",
		url.Values{"banned": {"1"}})
	if rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "playerNames must be a list") {
		t.Fatalf("ban failure = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminPlayersPageWithoutOpsStillListsPlayers(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	writePlayerFile(t, env.cfg.Root, "Jane", `{"name":"Jane","settings":{"staffRank":2}}`)
	env.h.ops = nil

	rec := env.request(t, http.MethodGet, "/admin/players", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/players = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Jane") {
		t.Fatalf("players page lost the player list:\n%s", body)
	}
	if !strings.Contains(body, "ops is not configured") {
		t.Fatalf("players page hides the unavailable banlist:\n%s", body)
	}
	if strings.Contains(body, `action="/admin/players/bans"`) {
		t.Fatalf("players page renders a banlist form without ops:\n%s", body)
	}
}
