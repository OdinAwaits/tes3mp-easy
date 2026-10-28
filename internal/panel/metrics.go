package panel

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	metricsInterval = 15 * time.Second
	metricsHistory  = 24 * time.Hour
	metricsMax      = int(metricsHistory/metricsInterval) + 1
	metricsMaxCPU   = 100.0
	metricsRAMLimit = int64(448 << 20)
	metricsChartX0  = 40.0
	metricsChartX1  = 590.0
	metricsChartY0  = 20.0
	metricsChartY1  = 130.0
	metricsMaxGap   = 45 * time.Second
)

// metricsSample is one point of the in-memory resource history. Game fields are
// nil when the cgroup is missing (server stopped), so the API returns null and
// the chart draws a gap instead of a false zero.
type metricsSample struct {
	TS         time.Time `json:"ts"`
	GameRAM    *int64    `json:"game_ram"`
	GameCPU    *float64  `json:"game_cpu"`
	SysCPU     *float64  `json:"sys_cpu"`
	SysRAMUsed uint64    `json:"sys_ram_used"`
	SwapUsed   uint64    `json:"swap_used"`
	DiskFree   *uint64   `json:"disk_free"`
}

type metricsRaw struct {
	GameRAM    *int64
	GameCPU    *float64
	SysCPU     *float64
	SysRAMUsed uint64
	SwapUsed   uint64
	DiskFree   *uint64
	DiskTotal  *uint64
}

type metricsReader interface {
	read(now time.Time) metricsRaw
}

// systemMetricsReader reads cgroup v2, /proc and statfs directly. The panel
// intentionally does not call ops for metrics: these files are read-only and
// do not require sudo.
type systemMetricsReader struct {
	cgroupDir   string
	meminfoPath string
	statPath    string
	root        string
	numCPU      float64

	lastUsage uint64
	lastAt    time.Time

	lastCPUIdle  uint64
	lastCPUTotal uint64
	lastCPUAt    time.Time
}

func newSystemMetricsReader(root string) *systemMetricsReader {
	return &systemMetricsReader{
		cgroupDir:   "/sys/fs/cgroup/system.slice/tes3mp.service",
		meminfoPath: "/proc/meminfo",
		statPath:    "/proc/stat",
		root:        root,
		numCPU:      float64(runtime.NumCPU()),
	}
}

func (r *systemMetricsReader) read(now time.Time) metricsRaw {
	now = now.UTC()
	var raw metricsRaw

	usage, usageOK := readUintFile(filepath.Join(r.cgroupDir, "cpu.stat"), "usage_usec")
	memory, memoryOK := readIntFile(filepath.Join(r.cgroupDir, "memory.current"))
	if memoryOK {
		value := memory
		raw.GameRAM = &value
	}
	if usageOK {
		switch {
		case r.lastAt.IsZero():
		case !now.After(r.lastAt):
		case now.Sub(r.lastAt) > metricsMaxGap:
		case usage < r.lastUsage:
		default:
			wall := now.Sub(r.lastAt).Seconds()
			if wall > 0 && r.numCPU > 0 {
				value := float64(usage-r.lastUsage) / (wall * 1e6) * 100 / r.numCPU
				value = clampPercent(value)
				raw.GameCPU = &value
			}
		}
		r.lastUsage = usage
		r.lastAt = now
	}

	raw.SysRAMUsed, raw.SwapUsed = readMemoryInfo(r.meminfoPath)
	raw.SysCPU = r.readSystemCPU(now)
	raw.DiskFree, raw.DiskTotal = readDisk(r.root)
	return raw
}

func (r *systemMetricsReader) readSystemCPU(now time.Time) *float64 {
	file, err := os.Open(r.statPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 8 || fields[0] != "cpu" {
		return nil
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	var result *float64
	switch {
	case r.lastCPUAt.IsZero():
	case !now.After(r.lastCPUAt):
	case now.Sub(r.lastCPUAt) > metricsMaxGap:
	case total < r.lastCPUTotal || idle < r.lastCPUIdle:
	default:
		totalDelta := total - r.lastCPUTotal
		idleDelta := idle - r.lastCPUIdle
		if totalDelta > 0 {
			value := (1 - float64(idleDelta)/float64(totalDelta)) * 100
			value = clampPercent(value)
			result = &value
		}
	}
	r.lastCPUTotal = total
	r.lastCPUIdle = idle
	r.lastCPUAt = now
	return result
}

func readUintFile(path, key string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	if key == "" {
		value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		return value, err == nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			return value, err == nil
		}
	}
	return 0, false
}

func readIntFile(path string) (int64, bool) {
	value, ok := readUintFile(path, "")
	if !ok {
		return 0, false
	}
	return int64(value), true
}

func readMemoryInfo(path string) (used, swap uint64) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	values := make(map[string]uint64, 16)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if total > available {
		used = total - available
	}
	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	if swapTotal > swapFree {
		swap = swapTotal - swapFree
	}
	return used, swap
}

func readDisk(root string) (*uint64, *uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		return nil, nil
	}
	free := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	return &free, &total
}

func clampPercent(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > metricsMaxCPU:
		return metricsMaxCPU
	default:
		return value
	}
}

type metricsStore struct {
	mu      sync.RWMutex
	reader  metricsReader
	now     func() time.Time
	samples []metricsSample

	startOnce sync.Once
}

func newMetricsStore(root string) *metricsStore {
	return &metricsStore{
		reader: newSystemMetricsReader(root),
		now:    time.Now,
	}
}

func newMetricsStoreWithReader(reader metricsReader, now func() time.Time) *metricsStore {
	if now == nil {
		now = time.Now
	}
	return &metricsStore{reader: reader, now: now}
}

// start launches the single sampling goroutine. It is safe to call more than
// once; the panel starts it from Server.Serve so unit tests never spawn it.
func (m *metricsStore) start(ctxDone <-chan struct{}) {
	if m == nil {
		return
	}
	m.startOnce.Do(func() {
		go func() {
			m.sample()
			ticker := time.NewTicker(metricsInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctxDone:
					return
				case <-ticker.C:
					m.sample()
				}
			}
		}()
	})
}

func (m *metricsStore) sample() {
	if m == nil || m.reader == nil {
		return
	}
	now := m.now().UTC()
	raw := m.reader.read(now)
	sample := metricsSample{
		TS:         now,
		GameRAM:    raw.GameRAM,
		GameCPU:    raw.GameCPU,
		SysCPU:     raw.SysCPU,
		SysRAMUsed: raw.SysRAMUsed,
		SwapUsed:   raw.SwapUsed,
		DiskFree:   raw.DiskFree,
	}
	m.mu.Lock()
	m.samples = append(m.samples, sample)
	if len(m.samples) > metricsMax {
		copy(m.samples, m.samples[len(m.samples)-metricsMax:])
		m.samples = m.samples[:metricsMax]
	}
	m.mu.Unlock()
}

func (m *metricsStore) snapshot(window string) (metricsWindowData, bool) {
	spec, ok := metricsWindowSpecFor(window)
	if !ok {
		return metricsWindowData{}, false
	}
	var samples []metricsSample
	var now time.Time
	if m != nil {
		m.mu.RLock()
		samples = append(samples, m.samples...)
		m.mu.RUnlock()
		now = m.now().UTC()
	} else {
		now = time.Now().UTC()
	}
	from := now.Add(-spec.duration)
	filtered := make([]metricsSample, 0, len(samples))
	for _, sample := range samples {
		if !sample.TS.Before(from) {
			filtered = append(filtered, sample)
		}
	}
	windowSamples := aggregateSamples(filtered, spec.step)
	available := false
	for _, sample := range windowSamples {
		if sample.GameRAM != nil || sample.GameCPU != nil {
			available = true
			break
		}
	}

	data := metricsWindowData{
		Window:    spec.name,
		Step:      int(spec.step / time.Second),
		Available: available,
		Samples:   windowSamples,
	}
	if len(data.Samples) == 0 {
		data.Samples = []metricsSample{}
	}
	latest, hasLatest := metricsSample{}, false
	if len(samples) > 0 {
		latest = samples[len(samples)-1]
		hasLatest = true
	}
	data.view = buildMetricsView(data, latest, hasLatest, now, spec)
	data.Charts = metricsCharts{
		RAM:  data.view.RAMChart,
		CPU:  data.view.CPUChart,
		Disk: data.view.DiskChart,
	}
	return data, true
}

type metricsWindowSpec struct {
	name     string
	duration time.Duration
	step     time.Duration
}

func metricsWindowSpecFor(window string) (metricsWindowSpec, bool) {
	switch window {
	case "", "1h":
		return metricsWindowSpec{name: "1h", duration: time.Hour, step: 15 * time.Second}, true
	case "6h":
		return metricsWindowSpec{name: "6h", duration: 6 * time.Hour, step: time.Minute}, true
	case "24h":
		return metricsWindowSpec{name: "24h", duration: 24 * time.Hour, step: 4 * time.Minute}, true
	default:
		return metricsWindowSpec{}, false
	}
}

func aggregateSamples(samples []metricsSample, step time.Duration) []metricsSample {
	if len(samples) == 0 {
		return []metricsSample{}
	}
	stepSeconds := int64(step / time.Second)
	if stepSeconds <= 0 {
		return append([]metricsSample{}, samples...)
	}
	result := make([]metricsSample, 0, len(samples))
	bucket := make([]metricsSample, 0, 16)
	var bucketKey int64
	flush := func() {
		if len(bucket) == 0 {
			return
		}
		last := bucket[len(bucket)-1]
		merged := metricsSample{
			TS:         last.TS,
			SysRAMUsed: last.SysRAMUsed,
			SwapUsed:   last.SwapUsed,
			DiskFree:   last.DiskFree,
		}
		for _, item := range bucket {
			merged.GameRAM = maxInt64Ptr(merged.GameRAM, item.GameRAM)
			merged.GameCPU = maxFloat64Ptr(merged.GameCPU, item.GameCPU)
			merged.SysCPU = maxFloat64Ptr(merged.SysCPU, item.SysCPU)
		}
		result = append(result, merged)
		bucket = bucket[:0]
	}
	for _, sample := range samples {
		key := sample.TS.Unix() / stepSeconds
		if len(bucket) > 0 && key != bucketKey {
			flush()
		}
		bucketKey = key
		bucket = append(bucket, sample)
	}
	flush()
	return result
}

func maxInt64Ptr(left, right *int64) *int64 {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	case *right > *left:
		return right
	default:
		return left
	}
}

func maxFloat64Ptr(left, right *float64) *float64 {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	case *right > *left:
		return right
	default:
		return left
	}
}

type metricsWindowData struct {
	Window    string          `json:"window"`
	Step      int             `json:"step"`
	Available bool            `json:"available"`
	Samples   []metricsSample `json:"samples"`
	Charts    metricsCharts   `json:"charts"`

	view metricsView
}

type metricsView struct {
	Window         string
	Step           int
	Available      bool
	Empty          bool
	CurrentRAM     string
	CurrentGameCPU string
	CurrentSysCPU  string
	CurrentSysRAM  string
	CurrentSwap    string
	CurrentDisk    string
	RAMChart       dashboardChart
	CPUChart       dashboardChart
	DiskChart      dashboardChart
}

type dashboardChart struct {
	Title      string      `json:"title"`
	RangeLabel string      `json:"range_label"`
	MaxLabel   string      `json:"max_label"`
	MidLabel   string      `json:"mid_label"`
	MinLabel   string      `json:"min_label"`
	StartLabel string      `json:"start_label"`
	MidTime    string      `json:"mid_time"`
	EndLabel   string      `json:"end_label"`
	Lines      []chartLine `json:"lines"`
	Empty      bool        `json:"empty"`
}

type chartLine struct {
	Class  string `json:"class"`
	Points string `json:"points"`
}

// metricsCharts is the JSON form of the dashboard charts. The server renders
// the polylines once, so the browser never re-implements the geometry and the
// axis labels always match the drawn scale.
type metricsCharts struct {
	RAM  dashboardChart `json:"ram"`
	CPU  dashboardChart `json:"cpu"`
	Disk dashboardChart `json:"disk"`
}

func buildMetricsView(window metricsWindowData, latest metricsSample,
	hasLatest bool, now time.Time, spec metricsWindowSpec,
) metricsView {
	view := metricsView{
		Window:         window.Window,
		Step:           window.Step,
		Available:      window.Available,
		Empty:          len(window.Samples) == 0,
		CurrentRAM:     "—",
		CurrentGameCPU: "—",
		CurrentSysCPU:  "—",
		CurrentSysRAM:  "—",
		CurrentSwap:    "—",
		CurrentDisk:    "—",
	}
	if hasLatest {
		if latest.GameRAM != nil {
			view.CurrentRAM = humanSize(*latest.GameRAM)
		}
		if latest.GameCPU != nil {
			view.CurrentGameCPU = fmt.Sprintf("%.1f %%", *latest.GameCPU)
		}
		if latest.SysCPU != nil {
			view.CurrentSysCPU = fmt.Sprintf("%.1f %%", *latest.SysCPU)
		}
		if latest.SysRAMUsed > 0 {
			view.CurrentSysRAM = humanSize(int64(latest.SysRAMUsed))
		}
		view.CurrentSwap = humanSize(int64(latest.SwapUsed))
		if latest.DiskFree != nil {
			view.CurrentDisk = humanSize(int64(*latest.DiskFree))
		}
	}

	from := now.Add(-spec.duration)
	view.RAMChart = buildChart(window.Samples, "RAM игры", rangeLabel(humanSizeLabel(float64(metricsRAMLimit))),
		from, now, float64(metricsRAMLimit), humanSizeLabel, func(sample metricsSample) *float64 {
			if sample.GameRAM == nil {
				return nil
			}
			value := float64(*sample.GameRAM)
			return &value
		}, "chart-line")
	view.CPUChart = buildDualCPUChart(window.Samples, from, now)

	diskMax := float64(7 << 30)
	if hasLatest && latest.DiskFree != nil {
		diskMax = float64(*latest.DiskFree) * 7 / 3
	}
	if diskMax < float64(1<<30) {
		diskMax = float64(1 << 30)
	}
	view.DiskChart = buildChart(window.Samples, "Свободно на диске", rangeLabel(humanSizeLabel(diskMax)),
		from, now, diskMax, humanSizeLabel, func(sample metricsSample) *float64 {
			if sample.DiskFree == nil {
				return nil
			}
			value := float64(*sample.DiskFree)
			return &value
		}, "chart-line chart-line--muted")
	return view
}

// rangeLabel renders the "0…max" caption from the same formatter as the axis,
// dropping the trailing ".0" that reads badly in running text.
func rangeLabel(maxLabel string) string {
	return "0…" + strings.Replace(maxLabel, ".0 ", " ", 1)
}

// humanSizeLabel formats an axis label for byte-valued charts.
func humanSizeLabel(value float64) string {
	return humanSize(int64(value))
}

// percentLabel formats an axis label for the 0…100 % CPU scale.
func percentLabel(value float64) string {
	return strconv.FormatFloat(value, 'f', 0, 64) + " %"
}

// buildChart renders one chart server-side: the axis labels come from the same
// maxValue used for the geometry, so the browser never re-scales the picture.
func buildChart(samples []metricsSample, title, rangeLabel string,
	from, to time.Time, maxValue float64, format func(float64) string,
	value func(metricsSample) *float64, class string,
) dashboardChart {
	chart := dashboardChart{
		Title:      title,
		RangeLabel: rangeLabel,
		MaxLabel:   format(maxValue),
		MidLabel:   format(maxValue / 2),
		MinLabel:   format(0),
		StartLabel: from.UTC().Format("15:04"),
		MidTime:    from.Add(to.Sub(from) / 2).UTC().Format("15:04"),
		EndLabel:   to.UTC().Format("15:04"),
		Empty:      len(samples) == 0 || value == nil,
	}
	if value == nil {
		chart.Lines = []chartLine{}
		return chart
	}
	chart.Lines = chartLines(samples, value, maxValue, class, from, to)
	if chart.Lines == nil {
		chart.Lines = []chartLine{}
	}
	chart.Empty = len(chart.Lines) == 0
	return chart
}

func buildDualCPUChart(samples []metricsSample, from, to time.Time) dashboardChart {
	chart := buildChart(samples, "CPU игры и системы",
		rangeLabel(percentLabel(metricsMaxCPU)), from, to,
		metricsMaxCPU, percentLabel, nil, "")
	chart.Lines = append(chart.Lines, chartLines(samples,
		func(sample metricsSample) *float64 { return sample.GameCPU },
		metricsMaxCPU, "chart-line", from, to)...)
	chart.Lines = append(chart.Lines, chartLines(samples,
		func(sample metricsSample) *float64 { return sample.SysCPU },
		metricsMaxCPU, "chart-line chart-line--info", from, to)...)
	if chart.Lines == nil {
		chart.Lines = []chartLine{}
	}
	chart.Empty = len(chart.Lines) == 0
	return chart
}

func chartLines(samples []metricsSample, value func(metricsSample) *float64,
	maxValue float64, class string, from, to time.Time,
) []chartLine {
	if maxValue <= 0 || len(samples) == 0 {
		return nil
	}
	total := to.Sub(from).Seconds()
	if total <= 0 {
		total = 1
	}
	var lines []chartLine
	var points []string
	flush := func() {
		if len(points) > 0 {
			lines = append(lines, chartLine{Class: class, Points: strings.Join(points, " ")})
			points = nil
		}
	}
	for _, sample := range samples {
		current := value(sample)
		if current == nil {
			flush()
			continue
		}
		ratioX := clampFloat(sample.TS.Sub(from).Seconds()/total, 0, 1)
		x := metricsChartX0 + ratioX*(metricsChartX1-metricsChartX0)
		ratioY := clampFloat(*current/maxValue, 0, 1)
		y := metricsChartY1 - ratioY*(metricsChartY1-metricsChartY0)
		points = append(points, fmt.Sprintf("%.0f,%.0f", x, y))
	}
	flush()
	return lines
}

func clampFloat(value, min, max float64) float64 {
	switch {
	case value < min:
		return min
	case value > max:
		return max
	default:
		return value
	}
}
