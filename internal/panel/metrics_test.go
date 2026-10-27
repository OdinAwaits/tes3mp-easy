package panel

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type sequenceMetricsReader struct {
	values []metricsRaw
	index  int
}

func (r *sequenceMetricsReader) read(time.Time) metricsRaw {
	if len(r.values) == 0 {
		return metricsRaw{}
	}
	value := r.values[r.index%len(r.values)]
	r.index++
	return value
}

func ptrInt64(value int64) *int64       { return &value }
func ptrFloat64(value float64) *float64 { return &value }
func ptrUint64(value uint64) *uint64    { return &value }

func TestMetricsAggregatesWindowsAndNullableGaps(t *testing.T) {
	now := time.Date(2026, 10, 27, 16, 0, 0, 0, time.UTC)
	reader := &sequenceMetricsReader{values: []metricsRaw{
		{GameRAM: ptrInt64(100), GameCPU: ptrFloat64(10), SysCPU: ptrFloat64(5),
			SysRAMUsed: 1000, SwapUsed: 10, DiskFree: ptrUint64(5000)},
		{GameRAM: nil, GameCPU: nil, SysCPU: ptrFloat64(7),
			SysRAMUsed: 1100, SwapUsed: 10, DiskFree: ptrUint64(4900)},
		{GameRAM: ptrInt64(300), GameCPU: ptrFloat64(30), SysCPU: ptrFloat64(9),
			SysRAMUsed: 1200, SwapUsed: 20, DiskFree: ptrUint64(4800)},
	}}
	store := newMetricsStoreWithReader(reader, func() time.Time { return now })
	for i := 0; i < 3; i++ {
		store.sample()
		now = now.Add(metricsInterval)
	}

	data, ok := store.snapshot("1h")
	if !ok || data.Window != "1h" || data.Step != 15 || !data.Available {
		t.Fatalf("snapshot = %+v ok=%v", data, ok)
	}
	if len(data.Samples) != 3 {
		t.Fatalf("samples = %d, want 3", len(data.Samples))
	}
	if data.Samples[0].GameRAM == nil || *data.Samples[0].GameRAM != 100 {
		t.Fatalf("first RAM = %+v", data.Samples[0].GameRAM)
	}
	if data.Samples[1].GameRAM != nil || data.Samples[1].GameCPU != nil {
		t.Fatalf("nil game values were lost: %+v", data.Samples[1])
	}
	if data.view.RAMChart.Empty {
		t.Fatal("RAM chart is empty")
	}
	if len(data.view.RAMChart.Lines) != 2 {
		t.Fatalf("RAM chart lines = %d, want a gap between two runs", len(data.view.RAMChart.Lines))
	}
	if len(data.view.CPUChart.Lines) == 0 {
		t.Fatal("CPU chart has no system series")
	}

	// 24h uses 4-minute buckets and never returns more than 360 points.
	if spec, ok := metricsWindowSpecFor("24h"); !ok || spec.step != 4*time.Minute {
		t.Fatalf("24h spec = %+v ok=%v", spec, ok)
	}
}

func TestMetricsAPIAndDashboardRejectBadWindow(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	now := time.Date(2026, 10, 27, 16, 0, 0, 0, time.UTC)
	env.h.metrics = newMetricsStoreWithReader(&sequenceMetricsReader{values: []metricsRaw{{
		GameRAM: ptrInt64(1024), GameCPU: ptrFloat64(12.5), SysCPU: ptrFloat64(3.5),
		SysRAMUsed: 2048, DiskFree: ptrUint64(4096),
	}}}, func() time.Time { return now })
	env.h.metrics.sample()

	rec := env.request(t, http.MethodGet, "/api/v1/admin/metrics?window=1h", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET metrics = %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Window  string          `json:"window"`
			Step    int             `json:"step"`
			Samples []metricsSample `json:"samples"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if !response.OK || response.Data.Window != "1h" || response.Data.Step != 15 ||
		len(response.Data.Samples) != 1 {
		t.Fatalf("metrics response = %+v", response)
	}

	for _, path := range []string{
		"/api/v1/admin/metrics?window=2h",
		"/api/v1/admin/metrics?window=1h&window=6h",
	} {
		rec = env.request(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", path, rec.Code)
		}
	}

	rec = env.request(t, http.MethodGet, "/admin?window=2h", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /admin?window=2h = %d, want 400", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "style=") {
		t.Fatal("dashboard contains an inline style")
	}
}

func TestMetricsRouteRequiresSession(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	rec := panelRequest(t, env.handler, http.MethodGet,
		"/api/v1/admin/metrics", nil, nil, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("anonymous metrics = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}
