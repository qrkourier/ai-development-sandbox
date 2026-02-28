package resources

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
)

// Monitor periodically collects system resource metrics
type Monitor struct {
	state    *state.AppState
	logger   *log.Logger
	interval time.Duration
}

// NewMonitor creates a resource monitor
func NewMonitor(appState *state.AppState, logger *log.Logger) *Monitor {
	interval := time.Duration(appState.Config().Supervision.ResourceSampleIntervalSec) * time.Second
	if interval == 0 {
		interval = 60 * time.Second
	}
	return &Monitor{
		state:    appState,
		logger:   logger,
		interval: interval,
	}
}

// Run starts the periodic resource collection loop
func (m *Monitor) Run(ctx context.Context) {
	// Collect immediately on start
	m.collect()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collect()
			m.checkAlerts()
		}
	}
}

func (m *Monitor) collect() {
	snap := state.ResourceSnapshot{
		Timestamp: time.Now(),
	}

	// CPU usage
	snap.CPUPercent = getCPUPercent()

	// RAM
	snap.RAMUsedMB, snap.RAMTotalMB = getRAMUsage()

	// GPU (NVIDIA)
	snap.GPUPercent, snap.VRAMUsedMB, snap.VRAMTotalMB = getGPUUsage()

	// Disk
	snap.DiskUsedGB, snap.DiskTotalGB = getDiskUsage()

	m.state.AddResourceSnapshot(snap)
}

func (m *Monitor) checkAlerts() {
	cfg := m.state.Config()
	snap := m.state.LatestResource()
	if snap == nil {
		return
	}

	if snap.RAMTotalMB > 0 {
		availPercent := float64(snap.RAMTotalMB-snap.RAMUsedMB) / float64(snap.RAMTotalMB) * 100
		if availPercent < float64(cfg.Alerts.RAMAvailablePercentMin) {
			m.logger.Warn("Low RAM", "available_percent", availPercent)
		}
	}

	if snap.DiskTotalGB > 0 {
		freeGB := snap.DiskTotalGB - snap.DiskUsedGB
		if freeGB < float64(cfg.Alerts.DiskFreeGBMin) {
			m.logger.Warn("Low disk space", "free_gb", freeGB)
		}
	}

	if snap.GPUPercent > float64(cfg.Alerts.GPUUtilizationPercent) {
		m.logger.Warn("High GPU utilization", "percent", snap.GPUPercent)
	}
}

func getCPUPercent() float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	// Read /proc/stat for CPU usage
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}
	var total, idle int64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseInt(fields[i], 10, 64)
		total += v
		if i == 4 { // idle is the 4th value after "cpu"
			idle = v
		}
	}
	if total == 0 {
		return 0
	}
	return float64(total-idle) / float64(total) * 100
}

func getRAMUsage() (used, total int64) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var totalKB, availKB int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			totalKB = val
		case "MemAvailable:":
			availKB = val
		}
	}
	total = totalKB / 1024
	used = (totalKB - availKB) / 1024
	return
}

func getGPUUsage() (percent float64, usedMB, totalMB int64) {
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ", ")
	if len(fields) < 3 {
		return 0, 0, 0
	}
	percent, _ = strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
	u, _ := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
	t, _ := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
	return percent, u, t
}

func getDiskUsage() (used, total float64) {
	cmd := exec.Command("df", "-BG", "--output=used,size", "/")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return 0, 0
	}
	u, _ := strconv.ParseFloat(strings.TrimRight(fields[0], "G"), 64)
	t, _ := strconv.ParseFloat(strings.TrimRight(fields[1], "G"), 64)
	return u, t
}

// DetectGPU returns VRAM in MB and total system RAM in MB
func DetectGPU() (vramMB int64, ramMB int64) {
	_, _, vramMB = getGPUUsage()
	_, ramTotal := getRAMUsage()
	return vramMB, ramTotal
}

// SelectOllamaModel selects the best Ollama model based on available hardware
func SelectOllamaModel() string {
	vram, _ := DetectGPU()
	switch {
	case vram >= 24000:
		return "qwen2.5-coder:32b-instruct-q8_0"
	case vram >= 12000:
		return "qwen2.5-coder:14b-instruct-q8_0"
	case vram >= 8000:
		return "qwen2.5-coder:7b-instruct-fp16"
	default:
		return "qwen2.5-coder:7b-instruct-q4_0"
	}
}
