//go:build linux

package app

import (
	"sync"
	"time"
)

// GPUInfo is a vendor-neutral snapshot of a single GPU.
type GPUInfo struct {
	Index      int
	Vendor     string // "nvidia", "amd", "intel"
	Name       string
	UtilPct    float64
	MemUsedMB  float64
	MemTotalMB float64
	TempC      float64
	PowerW     float64
	FreqMHz    int
	MaxFreqMHz int
	FanPct     float64
}

var (
	allGPUMu    sync.Mutex
	allGPULast  time.Time
	allGPUCache []GPUInfo
)

const allGPUCacheTTL = 400 * time.Millisecond

// queryAllGPUs returns every detected GPU across all supported vendors.
// Results are cached briefly because the underlying probes (nvidia-smi,
// sysfs walks, intel_gpu_top) are relatively expensive.
func queryAllGPUs() []GPUInfo {
	allGPUMu.Lock()
	defer allGPUMu.Unlock()
	if time.Since(allGPULast) < allGPUCacheTTL && allGPUCache != nil {
		return allGPUCache
	}

	var gpus []GPUInfo
	gpus = append(gpus, queryNvidiaGPUs()...)
	gpus = append(gpus, queryAMDGPUs()...)
	gpus = append(gpus, queryIntelGPUs()...)

	for i := range gpus {
		gpus[i].Index = i
	}

	allGPULast = time.Now()
	allGPUCache = gpus
	return gpus
}

// primaryGPU returns the GPU selected for the main gauges.
func primaryGPU() (GPUInfo, bool) {
	gpus := queryAllGPUs()
	if len(gpus) == 0 {
		return GPUInfo{}, false
	}
	if selectedGPU >= 0 && selectedGPU < len(gpus) {
		return gpus[selectedGPU], true
	}
	return gpus[0], true
}

// maxGPUFreqMHz returns the highest clock observed across all GPUs.
func maxGPUFreqMHz() int {
	best := 0
	for _, g := range queryAllGPUs() {
		if g.MaxFreqMHz > best {
			best = g.MaxFreqMHz
		}
		if g.FreqMHz > best {
			best = g.FreqMHz
		}
	}
	return best
}

// totalGPUPowerWatts sums the power draw of every GPU.
func totalGPUPowerWatts() float64 {
	var total float64
	for _, g := range queryAllGPUs() {
		total += g.PowerW
	}
	return total
}

// gpuVendorLabel returns a short human label for a GPU.
func gpuVendorLabel(g GPUInfo) string {
	switch g.Vendor {
	case "nvidia":
		return "NVIDIA"
	case "amd":
		return "AMD"
	case "intel":
		return "Intel"
	default:
		return "GPU"
	}
}
