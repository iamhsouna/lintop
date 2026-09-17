//go:build linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FanInfo represents a single system fan's state
type FanInfo struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ActualRPM int    `json:"actual_rpm"`
	MinRPM    int    `json:"min_rpm"`
	MaxRPM    int    `json:"max_rpm"`
	TargetRPM int    `json:"target_rpm"`
	Mode      int    `json:"mode"` // 0=auto, 1=forced
}

// TempSensor represents a single temperature sensor reading
type TempSensor struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// SocMetrics mirrors the mactop SoC metrics structure. On Linux the fields are
// populated from /proc, /sys/class/hwmon, RAPL and nvidia-smi. Apple-only
// rails (ANE, DRAM bandwidth) stay at zero.
type SocMetrics struct {
	CPUPower        float64      `json:"cpu_power"`
	GPUPower        float64      `json:"gpu_power"`
	ANEPower        float64      `json:"ane_power"`
	DRAMPower       float64      `json:"dram_power"`
	GPUSRAMPower    float64      `json:"gpu_sram_power"`
	SystemPower     float64      `json:"system_power"`
	TotalPower      float64      `json:"total_power"`
	GPUFreqMHz      int32        `json:"gpu_freq_mhz"`
	GPUActive       float64      `json:"gpu_active"`
	EClusterActive  float64      `json:"e_cluster_active"`
	PClusterActive  float64      `json:"p_cluster_active"`
	SClusterActive  float64      `json:"s_cluster_active,omitempty"`
	EClusterFreqMHz int32        `json:"e_cluster_freq_mhz"`
	PClusterFreqMHz int32        `json:"p_cluster_freq_mhz"`
	SClusterFreqMHz int32        `json:"s_cluster_freq_mhz,omitempty"`
	SocTemp         float32      `json:"soc_temp"`
	CPUTemp         float32      `json:"cpu_temp"`
	GPUTemp         float32      `json:"gpu_temp"`
	DRAMReadBW      float64      `json:"dram_read_bw_gbs"`
	DRAMWriteBW     float64      `json:"dram_write_bw_gbs"`
	DRAMBWCombined  float64      `json:"dram_bw_combined_gbs"`
	ANEReadBW       float64      `json:"ane_read_bw_gbs"`
	ANEWriteBW      float64      `json:"ane_write_bw_gbs"`
	ANEBWCombined   float64      `json:"ane_bw_combined_gbs"`
	ANEActive       float64      `json:"ane_active"`
	Fans            []FanInfo    `json:"-"`
	TempSensors     []TempSensor `json:"-"`
	PerGPU          []GPUSample  `json:"per_gpu,omitempty"`
}

// CPU energy state for RAPL-based package power estimation.
var (
	raplEnergyPath  string
	raplMaxEnergy   float64
	lastRAPLEnergy  float64
	lastRAPLTime    time.Time
	raplInitialized bool
)

func initSocMetrics() error {
	initRAPL()
	return nil
}

func cleanupSocMetrics() {}

// initRAPL locates an Intel/AMD RAPL powercap energy counter.
func initRAPL() {
	if raplInitialized {
		return
	}
	raplInitialized = true
	matches, _ := filepath.Glob("/sys/class/powercap/intel-rapl:*/energy_uj")
	for _, m := range matches {
		raplEnergyPath = m
		if b, err := os.ReadFile(filepath.Join(filepath.Dir(m), "max_energy_range_uj")); err == nil {
			raplMaxEnergy, _ = strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		}
		break
	}
	if raplEnergyPath == "" {
		// AMD exposes the same interface under a different driver name.
		matches, _ = filepath.Glob("/sys/class/powercap/*/energy_uj")
		if len(matches) > 0 {
			raplEnergyPath = matches[0]
		}
	}
}

// readRAPLPowerWatts returns package power draw in watts since the last call.
func readRAPLPowerWatts() float64 {
	if raplEnergyPath == "" {
		return 0
	}
	b, err := os.ReadFile(raplEnergyPath)
	if err != nil {
		return 0
	}
	energy, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return 0
	}
	now := time.Now()
	watts := 0.0
	if !lastRAPLTime.IsZero() {
		dt := now.Sub(lastRAPLTime).Seconds()
		de := energy - lastRAPLEnergy
		if de < 0 && raplMaxEnergy > 0 {
			de += raplMaxEnergy // counter wrapped
		}
		if dt > 0 && de >= 0 {
			watts = de / 1e6 / dt
		}
	}
	lastRAPLEnergy = energy
	lastRAPLTime = now
	return watts
}

type hwmonSensor struct {
	Key   string
	Name  string
	Value float64
}

// readHwmonTemps enumerates every hwmon temperature input on the system.
// Keys are shaped like SMC keys (T + category letter + index) so the existing
// grouping logic classifies them correctly.
func readHwmonTemps() []TempSensor {
	var sensors []TempSensor
	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hw := range hwmons {
		chipName := readTrim(filepath.Join(hw, "name"))
		inputs, _ := filepath.Glob(filepath.Join(hw, "temp*_input"))
		for _, in := range inputs {
			b, err := os.ReadFile(in)
			if err != nil {
				continue
			}
			milli, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			if err != nil {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(in), "_input")
			label := readTrim(filepath.Join(hw, base+"_label"))
			if label == "" {
				label = chipName
			}
			sensors = append(sensors, TempSensor{
				Key:   smcLikeTempKey(chipName, label, base),
				Name:  label,
				Value: milli / 1000.0,
			})
		}
	}
	return sensors
}

// smcLikeTempKey maps Linux hwmon sensors onto SMC-style keys so
// sensorGroupName (shared with the macOS UI) groups them sensibly.
func smcLikeTempKey(chip, label, base string) string {
	idx := 0
	if n, err := strconv.Atoi(strings.TrimPrefix(base, "temp")); err == nil {
		idx = n
	}
	l := strings.ToLower(label)
	switch {
	case strings.Contains(chip, "k10temp"), strings.Contains(chip, "coretemp"),
		strings.Contains(chip, "zenpower"), strings.Contains(chip, "cpu"):
		return fmt.Sprintf("TC%dP", idx)
	case strings.Contains(chip, "nvme"), strings.Contains(chip, "ssd"):
		return fmt.Sprintf("TS%dP", idx)
	case strings.Contains(chip, "iwlwifi"), strings.Contains(chip, "wifi"),
		label == "wireless":
		return fmt.Sprintf("TW%dP", idx)
	case strings.Contains(chip, "r8169"), strings.Contains(chip, "igb"),
		strings.Contains(chip, "e1000"), strings.Contains(chip, "board"):
		return fmt.Sprintf("TB%dP", idx)
	case strings.Contains(chip, "amdgpu"), strings.Contains(chip, "nouveau"),
		strings.Contains(chip, "nvidia"):
		return fmt.Sprintf("TR%dP", idx)
	case strings.Contains(chip, "jc42"), strings.Contains(chip, "acpitz"),
		strings.Contains(l, "ambient"), strings.Contains(l, "board"):
		return fmt.Sprintf("TA%dP", idx)
	case strings.Contains(chip, "mem"), strings.Contains(l, "dimm"):
		return fmt.Sprintf("TM%dP", idx)
	default:
		return fmt.Sprintf("TP%dP", idx)
	}
}

// readHwmonFans enumerates hwmon fan tachometers, if any.
func readHwmonFans() []FanInfo {
	var fans []FanInfo
	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	id := 0
	for _, hw := range hwmons {
		chipName := readTrim(filepath.Join(hw, "name"))
		inputs, _ := filepath.Glob(filepath.Join(hw, "fan*_input"))
		for _, in := range inputs {
			b, err := os.ReadFile(in)
			if err != nil {
				continue
			}
			rpm, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				continue
			}
			label := readTrim(filepath.Join(hw, strings.TrimSuffix(filepath.Base(in), "_input")+"_label"))
			if label == "" {
				label = chipName
			}
			fans = append(fans, FanInfo{
				ID:        id,
				Name:      label,
				ActualRPM: rpm,
				TargetRPM: rpm,
				Mode:      0,
			})
			id++
		}
	}
	return fans
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// cpuTempFromHwmon picks the most plausible CPU package temperature.
func cpuTempFromHwmon(sensors []TempSensor) float64 {
	var best float64
	for _, s := range sensors {
		if strings.HasPrefix(s.Key, "TC") {
			if s.Value > best {
				best = s.Value
			}
		}
	}
	if best == 0 {
		for _, s := range sensors {
			n := strings.ToLower(s.Name + " " + s.Key)
			if strings.Contains(n, "package") || strings.Contains(n, "tctl") ||
				strings.Contains(n, "tdie") || strings.Contains(n, "cpu") {
				if s.Value > best {
					best = s.Value
				}
			}
		}
	}
	if best == 0 {
		for _, s := range sensors {
			if s.Value > best {
				best = s.Value
			}
		}
	}
	return best
}

func sampleSocMetrics(durationMs int) SocMetrics {
	var m SocMetrics

	sensors := readHwmonTemps()
	m.TempSensors = sensors
	m.Fans = readHwmonFans()

	cpuTemp := cpuTempFromHwmon(sensors)
	m.CPUTemp = float32(cpuTemp)
	m.SocTemp = float32(cpuTemp)

	cpuWatts := readRAPLPowerWatts()
	m.CPUPower = cpuWatts

	gpus := queryAllGPUs()
	m.PerGPU = make([]GPUSample, 0, len(gpus))
	for _, g := range gpus {
		m.PerGPU = append(m.PerGPU, GPUSample{
			Index:       g.Index,
			Vendor:      g.Vendor,
			Name:        g.Name,
			UtilPercent: g.UtilPct,
			MemUsedMB:   g.MemUsedMB,
			MemTotalMB:  g.MemTotalMB,
			TempC:       g.TempC,
			PowerW:      g.PowerW,
			FreqMHz:     g.FreqMHz,
			MaxFreqMHz:  g.MaxFreqMHz,
			FanPercent:  g.FanPct,
		})
		// Surface each GPU temperature through the same sensor list the fan /
		// thermals layout renders.
		if g.TempC > 0 {
			name := "GPU"
			if len(gpus) > 1 {
				name = fmt.Sprintf("%s %s #%d", gpuVendorLabel(g), "GPU", g.Index)
			}
			m.TempSensors = append(m.TempSensors, TempSensor{
				Key:   fmt.Sprintf("TR%dP", g.Index),
				Name:  name,
				Value: g.TempC,
			})
		}
	}

	if g, ok := primaryGPU(); ok {
		m.GPUActive = g.UtilPct
		m.GPUFreqMHz = int32(g.FreqMHz)
		m.GPUTemp = float32(g.TempC)
		m.GPUPower = g.PowerW
	}

	gpuWatts := totalGPUPowerWatts()
	m.TotalPower = cpuWatts + gpuWatts + m.ANEPower + m.DRAMPower
	m.SystemPower = m.TotalPower

	return m
}

// getSocThermalState returns a coarse thermal pressure level. Linux has no
// direct equivalent of NSProcessInfo.thermalState, so map the hottest CPU
// reading onto the same scale.
func getSocThermalState() int {
	sensors := readHwmonTemps()
	t := cpuTempFromHwmon(sensors)
	switch {
	case t >= 95:
		return 3
	case t >= 85:
		return 2
	case t >= 75:
		return 1
	default:
		return 0
	}
}

// ---- Diagnostics (macOS-only features, no-ops on Linux) ----

func DebugIOReport()     {}
func DumpIOReportDebug() {}
func DumpAllSMCTemps()   {}

// ---- Fan control (unsupported on Linux, returned as errors so the UI can
// report that the feature is unavailable) ----

var errFanUnsupported = fmt.Errorf("fan control is not supported on Linux")

func SetFanForceTest(enabled bool) error   { return errFanUnsupported }
func SetFanMode(fanIndex, mode int) error  { return errFanUnsupported }
func SetFanTarget(fanIndex, rpm int) error { return errFanUnsupported }
func ResetFansToAuto() error               { return errFanUnsupported }
