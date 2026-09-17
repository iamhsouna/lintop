//go:build linux

package app

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	nvSmiOnce    sync.Once
	nvSmiPath    string
	nvSmiMissing bool
	nvMaxFreq    int
	nvMaxFreqMu  sync.Mutex
	nvLastQuery  time.Time
	nvLastResult []GPUInfo
	nvQueryMu    sync.Mutex
)

// findNvidiaSmi locates the nvidia-smi binary once.
func findNvidiaSmi() string {
	nvSmiOnce.Do(func() {
		if p, err := exec.LookPath("nvidia-smi"); err == nil {
			nvSmiPath = p
			return
		}
		nvSmiMissing = true
	})
	if nvSmiMissing {
		return ""
	}
	return nvSmiPath
}

// NVIDIA GPU queries are relatively expensive; reuse a recent snapshot for a
// short window so multiple callers in one tick don't each spawn the binary.
const nvCacheTTL = 250 * time.Millisecond

// queryNvidiaGPUs returns one entry per NVIDIA GPU, or nil when unavailable.
func queryNvidiaGPUs() []GPUInfo {
	path := findNvidiaSmi()
	if path == "" {
		return nil
	}

	nvQueryMu.Lock()
	defer nvQueryMu.Unlock()
	if time.Since(nvLastQuery) < nvCacheTTL && nvLastResult != nil {
		return nvLastResult
	}

	args := []string{
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,clocks.sm,clocks.max.sm,fan.speed",
		"--format=csv,noheader,nounits",
	}
	out, err := exec.Command(path, args...).Output()
	if err != nil {
		nvLastQuery = time.Now()
		nvLastResult = nil
		return nil
	}

	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 10 {
			continue
		}
		g := GPUInfo{
			Vendor:     "nvidia",
			Name:       strings.TrimSpace(fields[1]),
			UtilPct:    floatSafe(fields[2]),
			MemUsedMB:  floatSafe(fields[3]),
			MemTotalMB: floatSafe(fields[4]),
			TempC:      floatSafe(fields[5]),
			PowerW:     floatSafe(fields[6]),
			FreqMHz:    atoiSafe(fields[7]),
			MaxFreqMHz: atoiSafe(fields[8]),
			FanPct:     floatSafe(fields[9]),
		}
		gpus = append(gpus, g)
	}

	if len(gpus) > 0 {
		nvMaxFreqMu.Lock()
		if gpus[0].MaxFreqMHz > nvMaxFreq {
			nvMaxFreq = gpus[0].MaxFreqMHz
		}
		nvMaxFreqMu.Unlock()
	}

	nvLastQuery = time.Now()
	nvLastResult = gpus
	return gpus
}

func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	// nvidia-smi prints "N/A" or "[Not Supported]" for unreadable fields.
	if s == "" || s == "N/A" || strings.HasPrefix(s, "[") {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

func floatSafe(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// nvidiaMaxFreqMHz returns the largest SM clock observed across NVIDIA GPUs.
func nvidiaMaxFreqMHz() int {
	nvMaxFreqMu.Lock()
	defer nvMaxFreqMu.Unlock()
	return nvMaxFreq
}

// queryNvidiaComputeApps returns per-PID VRAM usage for active compute
// processes. nvidia-smi does not expose a monotonic GPU-time counter, so this
// is the best available per-process signal.
func queryNvidiaComputeApps() map[int]uint64 {
	path := findNvidiaSmi()
	if path == "" {
		return nil
	}
	out, err := exec.Command(path,
		"--query-compute-apps=pid,used_memory",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	result := make(map[int]uint64)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		pid := atoiSafe(fields[0])
		memMB := atoiSafe(fields[1])
		if pid > 0 && memMB > 0 {
			result[pid] = uint64(memMB) * 1024 * 1024
		}
	}
	return result
}
