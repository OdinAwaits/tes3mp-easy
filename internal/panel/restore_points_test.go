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
) (RestoreResult, error) {
	f.restored = append(f.restored, name)
	return RestoreResult{Archive: name, Restarted: true}, f.restoreErr
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
