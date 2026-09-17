//go:build linux

package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const amdVendorID = 0x1002

// queryAMDGPUs enumerates AMD GPUs via /sys/class/drm and amdgpu sysfs.
func queryAMDGPUs() []GPUInfo {
	var gpus []GPUInfo

	cards, _ := filepath.Glob("/sys/class/drm/card*")
	for _, card := range cards {
		if !cardNameRe.MatchString(filepath.Base(card)) {
			continue
		}
		deviceDir := filepath.Join(card, "device")
		vendor, ok := readUintHex(filepath.Join(deviceDir, "vendor"))
		if !ok || vendor != amdVendorID {
			continue
		}

		g := GPUInfo{
			Vendor: "amd",
			Name:   pciDeviceName(pciSlotName(deviceDir)),
		}
		if g.Name == "" {
			g.Name = "AMD Radeon GPU"
		}

		if v, ok := readUint(filepath.Join(deviceDir, "gpu_busy_percent")); ok {
			g.UtilPct = float64(v)
		}
		if v, ok := readUint(filepath.Join(deviceDir, "mem_info_vram_used")); ok {
			g.MemUsedMB = float64(v) / (1024 * 1024)
		}
		if v, ok := readUint(filepath.Join(deviceDir, "mem_info_vram_total")); ok {
			g.MemTotalMB = float64(v) / (1024 * 1024)
		}

		g.TempC = maxHwmonTempC(deviceDir)
		g.PowerW = hwmonPowerWatts(deviceDir)

		g.FreqMHz, g.MaxFreqMHz = readAMDClocks(deviceDir, card)

		gpus = append(gpus, g)
	}
	return gpus
}

// readAMDClocks determines the current and maximum shader clock. It prefers the
// hwmon frequency input and falls back to the DPM clock table.
func readAMDClocks(deviceDir, cardDir string) (current, max int) {
	for _, hw := range hwmonDeviceDirs(deviceDir) {
		if v, ok := readUint(filepath.Join(hw, "freq1_input")); ok && v > 0 {
			current = int(v / 1000000) // Hz -> MHz
		}
	}
	max = current

	// pp_dpm_sclk lists "... <MHz>Mhz" per state; the active one is starred.
	for _, p := range []string{
		filepath.Join(deviceDir, "pp_dpm_sclk"),
		filepath.Join(cardDir, "device", "pp_dpm_sclk"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			mhz := parseMHz(line)
			if mhz > max {
				max = mhz
			}
			if strings.Contains(line, "*") && mhz > 0 {
				current = mhz
			}
		}
		break
	}
	if current == 0 {
		current = max
	}
	return current, max
}

// parseMHz extracts the numeric clock from a DPM line like "1: 1800Mhz *".
func parseMHz(line string) int {
	lower := strings.ToLower(line)
	idx := strings.Index(lower, "mhz")
	if idx <= 0 {
		return 0
	}
	// Walk backwards over digits (and an optional leading dot).
	end := idx
	start := end
	for start > 0 && (line[start-1] >= '0' && line[start-1] <= '9' || line[start-1] == '.') {
		start--
	}
	if start == end {
		return 0
	}
	if f, err := strconv.ParseFloat(line[start:end], 64); err == nil {
		return int(f)
	}
	return 0
}
