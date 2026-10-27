package panel

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

type fakeRestorePointAdminOps struct {
	*fakeAdminOps
	snapshots       SnapshotList
	checkpoints     CheckpointList
	snapshotsErr    error
	checkpointsErr  error
	createdSnapshot bool
	createdCheck    bool
	restored        []string
	restoreErr      error
	acceptMismatch  []bool
	pins            []string
	removals        []string
	removalForce    []bool
	removeResult    RestorePointRemoveResult
	removeErr       error
}

func (f *fakeRestorePointAdminOps) SnapshotsList(context.Context, int, string,
) (SnapshotList, error) {
	return f.snapshots, f.snapshotsErr
}

func (f *fakeRestorePointAdminOps) SnapshotCreate(context.Context) (Snapshot, error) {
	f.createdSnapshot = true
	return Snapshot{Archive: "snapshot-2026-10-27_16-40-00.tar.gz"}, nil
}

func (f *fakeRestorePointAdminOps) CheckpointsList(context.Context, int, string,
) (CheckpointList, error) {
	return f.checkpoints, f.checkpointsErr
}

func (f *fakeRestorePointAdminOps) CheckpointCreate(context.Context) (Checkpoint, error) {
	f.createdCheck = true
	return Checkpoint{Archive: "checkpoint-2026-10-27_16-45-00.tar.gz"}, nil
}

func (f *fakeRestorePointAdminOps) SnapshotRestore(_ context.Context, name string,
) (RestoreResult, error) {
	f.restored = append(f.restored, name)
	return RestoreResult{Archive: name, Restarted: true}, f.restoreErr
}

func (f *fakeRestorePointAdminOps) CheckpointRestore(_ context.Context, name string,
	acceptMismatch bool,
) (RestoreResult, error) {
	f.restored = append(f.restored, name)
	f.acceptMismatch = append(f.acceptMismatch, acceptMismatch)
	return RestoreResult{Archive: name, Restarted: true}, f.restoreErr
}

func (f *fakeRestorePointAdminOps) CheckpointPin(_ context.Context, name string,
	pinned bool,
) error {
	action := "unpin"
	if pinned {
		action = "pin"
	}
	f.pins = append(f.pins, action+":"+name)
	return nil
}

func (f *fakeRestorePointAdminOps) SnapshotRemove(_ context.Context, name string,
	force bool,
) (RestorePointRemoveResult, error) {
	return f.remove("snapshot:"+name, force)
}

func (f *fakeRestorePointAdminOps) CheckpointRemove(_ context.Context, name string,
	force bool,
) (RestorePointRemoveResult, error) {
	return f.remove("checkpoint:"+name, force)
}

func (f *fakeRestorePointAdminOps) remove(name string, force bool,
) (RestorePointRemoveResult, error) {
	f.removals = append(f.removals, name)
	f.removalForce = append(f.removalForce, force)
	if f.removeErr != nil {
		return RestorePointRemoveResult{}, f.removeErr
	}
	if len(f.removeResult.Removed) == 0 && len(f.removeResult.Skipped) == 0 {
		archive := strings.TrimPrefix(name, "snapshot:")
		archive = strings.TrimPrefix(archive, "checkpoint:")
		return RestorePointRemoveResult{
			Removed: []string{archive},
			Freed:   1024,
		}, nil
	}
	return f.removeResult, nil
}

func newRestorePointAdminEnv(t *testing.T, rp *fakeRestorePointAdminOps) *adminTestEnv {
	t.Helper()
	auth, _, cfg := newTestAuthStore(t)
	h, err := newPanelHandler(cfg, auth, rp, log.New(io.Discard, "", 0),
		func() time.Time { return sessionTestNow })
	if err != nil {
		t.Fatalf("newPanelHandler: %v", err)
	}
	session, err := h.sessions.create(sessionTestNow, false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &adminTestEnv{
		handler: withSecurityHeaders(h.routes()),
		h:       h,
		cfg:     cfg,
		ops:     rp.fakeAdminOps,
		session: session,
		cookie:  &http.Cookie{Name: sessionCookieName, Value: session.ID},
	}
}

func TestRestorePointsPinAndDelete(t *testing.T) {
	pinned := "checkpoint-2026-10-27_16-45-00.tar.gz"
	missingBase := "checkpoint-2026-10-27_16-46-00.tar.gz"
	base := "snapshot-2026-10-27_16-40-00.tar.gz"
	old := "snapshot-2026-10-26_16-40-00.tar.gz"
	rp := &fakeRestorePointAdminOps{
		fakeAdminOps: &fakeAdminOps{},
		snapshots: SnapshotList{Items: []SnapshotItem{
			{Name: "snapshot-2026-10-27_17-00-00.tar.gz", Size: 2048,
				Mtime: "2026-10-27T17:00:00Z", Reason: "manual"},
			{Name: base, Size: 2048, Mtime: "2026-10-27T16:40:00Z",
				Reason: "base", UsedBy: 1},
			{Name: old, Size: 1024, Mtime: "2026-10-26T16:40:00Z",
				Reason: "manual"},
		}},
		checkpoints: CheckpointList{Items: []CheckpointItem{
			{Name: pinned, Size: 1024, Mtime: "2026-10-27T16:45:00Z",
				Reason: "manual", Label: stringPtr("перед квестом"), Pinned: true,
				Base: CheckpointBaseItem{Name: base, Available: true}},
			{Name: missingBase, Size: 1024,
				Mtime: "2026-10-27T16:46:00Z", Reason: "timer",
				Base: CheckpointBaseItem{Name: base, Available: false}},
		}},
	}
	env := newRestorePointAdminEnv(t, rp)

	rec := env.request(t, http.MethodGet, "/admin/restore-points", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET restore-points = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"закреплён", "База удалена", "Ярлык: перед квестом",
		"Удалить вместе с базой", "Защищён: последний снапшот",
		"Только мир, оставить текущие моды",
		`data-confirm-name="` + old + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page does not contain %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, `action="/admin/restore-points/delete"`); got != 4 {
		t.Fatalf("delete forms = %d, want 4 (latest snapshot is protected)", got)
	}

	rec = env.request(t, http.MethodPost, "/admin/restore-points/pin",
		url.Values{"archive": {pinned}, "pin_action": {"unpin"}})
	if rec.Code != http.StatusSeeOther ||
		len(rp.pins) != 1 || rp.pins[0] != "unpin:"+pinned {
		t.Fatalf("unpin = %d calls=%v", rec.Code, rp.pins)
	}

	rec = env.request(t, http.MethodPost, "/admin/restore-points/restore",
		url.Values{"archive": {missingBase}, "accept_mismatch": {"1"}})
	if rec.Code != http.StatusSeeOther || len(rp.restored) != 1 ||
		rp.restored[0] != missingBase || !rp.acceptMismatch[0] {
		t.Fatalf("restore with mismatch = %d restored=%v accept=%v",
			rec.Code, rp.restored, rp.acceptMismatch)
	}

	rec = env.request(t, http.MethodPost, "/admin/restore-points/delete",
		url.Values{"archive": {pinned}, "confirm": {"wrong"}})
	if rec.Code != http.StatusBadRequest || len(rp.removals) != 0 {
		t.Fatalf("wrong confirm = %d removals=%v", rec.Code, rp.removals)
	}

	rec = env.request(t, http.MethodPost, "/admin/restore-points/delete",
		url.Values{"archive": {pinned}, "confirm": {pinned}, "force": {"1"}})
	if rec.Code != http.StatusOK || len(rp.removals) != 1 ||
		rp.removals[0] != "checkpoint:"+pinned || rp.removalForce[0] {
		t.Fatalf("checkpoint delete = %d removals=%v force=%v",
			rec.Code, rp.removals, rp.removalForce)
	}
	if !strings.Contains(rec.Body.String(), "Удалено: "+pinned) {
		t.Fatalf("delete page does not report success:\n%s", rec.Body.String())
	}

	rec = env.request(t, http.MethodPost, "/admin/restore-points/delete",
		url.Values{"archive": {base}, "confirm": {base}, "force": {"1"}})
	if rec.Code != http.StatusOK || len(rp.removals) != 2 ||
		rp.removals[1] != "snapshot:"+base || !rp.removalForce[1] {
		t.Fatalf("snapshot delete = %d removals=%v force=%v",
			rec.Code, rp.removals, rp.removalForce)
	}
}

func TestRestorePointsDeleteExplainsArchiveInUse(t *testing.T) {
	tests := []struct {
		name    string
		archive string
		want    string
		notWant string
	}{
		{
			name:    "pinned checkpoint",
			archive: "checkpoint-2026-10-27_16-45-00.tar.gz",
			want:    "Снимите закрепление",
			notWant: "Удалить вместе с базой",
		},
		{
			name:    "snapshot base",
			archive: "snapshot-2026-10-27_16-40-00.tar.gz",
			want:    "Удалить вместе с базой",
			notWant: "Снимите закрепление",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := &fakeRestorePointAdminOps{
				fakeAdminOps: &fakeAdminOps{},
				removeErr: &OpsError{
					Exit:    ops.ExitArchiveInUse,
					Code:    ops.CodeArchiveInUse,
					Message: "restore point is in use",
				},
			}
			env := newRestorePointAdminEnv(t, rp)

			rec := env.request(t, http.MethodPost, "/admin/restore-points/delete",
				url.Values{"archive": {tt.archive}, "confirm": {tt.archive}})
			if rec.Code != http.StatusConflict {
				t.Fatalf("delete in use = %d: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, tt.want) {
				t.Fatalf("in-use page does not contain %q:\n%s", tt.want, body)
			}
			if strings.Contains(body, tt.notWant) {
				t.Fatalf("in-use page contains %q:\n%s", tt.notWant, body)
			}
		})
	}
}

func TestRestorePointsPageUsesSnapshotAndCheckpointOps(t *testing.T) {
	rp := &fakeRestorePointAdminOps{
		fakeAdminOps: &fakeAdminOps{},
		snapshots: SnapshotList{Items: []SnapshotItem{
			{Name: "snapshot-2026-10-27_16-40-00.tar.gz", Size: 2048,
				Mtime: "2026-10-27T16:40:00Z", Reason: "manual"},
		}},
		checkpoints: CheckpointList{Items: []CheckpointItem{
			{Name: "checkpoint-2026-10-27_16-45-00.tar.gz", Size: 1024,
				Mtime: "2026-10-27T16:45:00Z", Reason: "timer",
				Base: CheckpointBaseItem{
					Name: "snapshot-2026-10-27_16-40-00.tar.gz", Available: true,
				}},
			{Name: "checkpoint-predeploy-2026-10-27_16-50-00.tar.gz",
				Size: 512, Mtime: "2026-10-27T16:50:00Z", Reason: "predeploy"},
		}},
	}
	env := newRestorePointAdminEnv(t, rp)

	rec := env.request(t, http.MethodGet, "/admin/restore-points", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET restore-points = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"snapshot-2026-10-27_16-40-00.tar.gz",
		"checkpoint-2026-10-27_16-45-00.tar.gz",
		"checkpoint-predeploy-2026-10-27_16-50-00.tar.gz",
		"Снапшоты", "Чекпоинты", "Predeploy-чекпоинты",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page does not contain %q:\n%s", want, body)
		}
	}

	rec = env.request(t, http.MethodPost,
		"/admin/restore-points/checkpoint/create", url.Values{})
	if rec.Code != http.StatusSeeOther || !rp.createdCheck {
		t.Fatalf("checkpoint create = %d created=%v", rec.Code, rp.createdCheck)
	}
	rec = env.request(t, http.MethodPost,
		"/admin/restore-points/snapshot/create", url.Values{})
	if rec.Code != http.StatusSeeOther || !rp.createdSnapshot {
		t.Fatalf("snapshot create = %d created=%v", rec.Code, rp.createdSnapshot)
	}

	name := "snapshot-2026-10-27_16-40-00.tar.gz"
	rec = env.request(t, http.MethodPost, "/admin/restore-points/restore",
		url.Values{"archive": {name}})
	if rec.Code != http.StatusSeeOther || len(rp.restored) != 1 ||
		rp.restored[0] != name {
		t.Fatalf("snapshot restore = %d calls=%v", rec.Code, rp.restored)
	}
	name = "checkpoint-2026-10-27_16-45-00.tar.gz"
	rec = env.request(t, http.MethodPost, "/admin/restore-points/restore",
		url.Values{"archive": {name}})
	if rec.Code != http.StatusSeeOther || len(rp.restored) != 2 ||
		rp.restored[1] != name {
		t.Fatalf("checkpoint restore = %d calls=%v", rec.Code, rp.restored)
	}
}

func stringPtr(value string) *string {
	return &value
}
