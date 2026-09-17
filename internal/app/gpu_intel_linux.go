//go:build linux

package app

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const intelVendorID = 0x8086

type intelGPUSample struct {
	Engines map[string]struct {
		Busy float64 `json:"busy"`
	} `json:"engines"`
	Frequency struct {
		Actual float64 `json:"actual"`
	} `json:"frequency"`
}

var (
	intelSmiPath    string
	intelSmiChecked bool
	intelUtilMu     sync.Mutex
	intelUtilLast   time.Time
	intelUtilValue  float64
	intelUtilTTL    = 2 * time.Second
)

// intelGPUTopPath finds intel_gpu_top once.
func intelGPUTopPath() string {
	if !intelSmiChecked {
		intelSmiChecked = true
		if p, err := exec.LookPath("intel_gpu_top"); err == nil {
			intelSmiPath = p
		}
	}
	return intelSmiPath
}

// queryIntelGPUs enumerates Intel GPUs via /sys/class/drm. Utilization needs
// intel_gpu_top with CAP_PERFMON; when unavailable it reads 0.
func queryIntelGPUs() []GPUInfo {
	var gpus []GPUInfo

	cards, _ := filepath.Glob("/sys/class/drm/card*")
	for _, card := range cards {
		if !cardNameRe.MatchString(filepath.Base(card)) {
			continue
		}
		deviceDir := filepath.Join(card, "device")
		vendor, ok := readUintHex(filepath.Join(deviceDir, "vendor"))
		if !ok || vendor != intelVendorID {
			continue
		}

		g := GPUInfo{
			Vendor: "intel",
			Name:   pciDeviceName(pciSlotName(deviceDir)),
		}
		if g.Name == "" {
			g.Name = "Intel GPU"
		}

		if v, ok := readUint(filepath.Join(card, "gt_cur_freq_mhz")); ok {
			g.FreqMHz = int(v)
		}
		if v, ok := readUint(filepath.Join(card, "gt_max_freq_mhz")); ok {
			g.MaxFreqMHz = int(v)
		}
		if g.MaxFreqMHz == 0 {
			if v, ok := readUint(filepath.Join(card, "gt_act_freq_mhz")); ok {
				g.FreqMHz = int(v)
			}
		}

		g.TempC = maxHwmonTempC(deviceDir)
		g.PowerW = hwmonPowerWatts(deviceDir)
		g.UtilPct = intelUtilization()

		gpus = append(gpus, g)
	}
	return gpus
}

// intelUtilization samples intel_gpu_top for a single period and returns the
// busiest engine percentage. Results are cached to avoid respawning the tool.
func intelUtilization() float64 {
	path := intelGPUTopPath()
	if path == "" {
		return 0
	}

	intelUtilMu.Lock()
	defer intelUtilMu.Unlock()
	if time.Since(intelUtilLast) < intelUtilTTL {
		return intelUtilValue
	}
	intelUtilLast = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "-J", "-s", "250", "-o", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0
	}
	if err := cmd.Start(); err != nil {
		intelUtilValue = 0
		return 0
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// intel_gpu_top streams a JSON array; decode just the first sample without
	// waiting for the (never-closing) array.
	dec := json.NewDecoder(stdout)
	if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
		intelUtilValue = 0
		return 0
	}
	var sample intelGPUSample
	if err := dec.Decode(&sample); err != nil {
		intelUtilValue = 0
		return 0
	}

	best := 0.0
	for _, e := range sample.Engines {
		if e.Busy > best {
			best = e.Busy
		}
	}
	intelUtilValue = best
	return best
}
