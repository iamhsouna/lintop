//go:build linux

package app

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"lintop/internal/i18n"
)

var (
	cachedSOCInfoResult SystemInfo
	socInfoOnce         sync.Once
)

type VolumeInfo struct {
	Name      string
	Total     float64
	Used      float64
	Available float64
	UsedPct   float64
}

// getVolumes lists real mounted filesystems with their usage.
func getVolumes() []VolumeInfo {
	var volumes []VolumeInfo

	f, err := os.Open("/proc/mounts")
	if err != nil {
		return volumes
	}
	defer f.Close()

	// Pseudo/virtual filesystems that aren't meaningful "disks".
	skipFS := map[string]bool{
		"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true,
		"tmpfs": true, "cgroup": true, "cgroup2": true, "pstore": true,
		"securityfs": true, "debugfs": true, "tracefs": true, "configfs": true,
		"fusectl": true, "mqueue": true, "hugetlbfs": true, "bpf": true,
		"binfmt_misc": true, "autofs": true, "ramfs": true, "efivarfs": true,
		"nsfs": true, "squashfs": true, "overlay": true, "fuse.gvfsd-fuse": true,
		"fusermount": true, "rpc_pipefs": true,
	}

	seenDevice := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mountpoint, fstype := fields[0], unescapeMount(fields[1]), fields[2]
		if skipFS[fstype] || strings.HasPrefix(fstype, "fuse.") {
			continue
		}
		if !strings.HasPrefix(device, "/dev/") && device != "overlay" {
			continue
		}
		if seenDevice[device] {
			continue
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(mountpoint, &st); err != nil {
			continue
		}
		bs := uint64(st.Bsize)
		total := st.Blocks * bs
		free := st.Bfree * bs
		avail := st.Bavail * bs
		if total == 0 {
			continue
		}
		used := total - free
		seenDevice[device] = true

		name := mountpoint
		if mountpoint == "/" {
			name = "root"
		} else {
			name = strings.TrimPrefix(mountpoint, "/")
			name = strings.ReplaceAll(name, "/", "_")
		}
		if len(name) > 12 {
			name = name[:12]
		}
		volumes = append(volumes, VolumeInfo{
			Name:      name,
			Total:     float64(total) / 1e9,
			Used:      float64(used) / 1e9,
			Available: float64(avail) / 1e9,
			UsedPct:   float64(used) / float64(total) * 100,
		})
		if len(volumes) >= 6 {
			break
		}
	}
	return volumes
}

func unescapeMount(s string) string {
	s = strings.ReplaceAll(s, "\\040", " ")
	s = strings.ReplaceAll(s, "\\011", "\t")
	s = strings.ReplaceAll(s, "\\012", "\n")
	return s
}

func getSOCInfo() SystemInfo {
	socInfoOnce.Do(func() {
		cachedSOCInfoResult = computeSOCInfo()
	})
	return cachedSOCInfoResult
}

func computeSOCInfo() SystemInfo {
	cores := runtime.NumCPU()
	name := cpuBrandString()
	gpuCount := len(queryAllGPUs())
	return SystemInfo{
		Name:         name,
		CoreCount:    cores,
		PCoreCount:   cores,
		GPUCoreCount: gpuCount,
	}
}

// sysctlStringByName emulates the macOS sysctl lookups used by the UI on Linux.
func sysctlStringByName(name string) (string, error) {
	switch name {
	case "kern.osrelease":
		var uts syscall.Utsname
		if err := syscall.Uname(&uts); err != nil {
			return "", err
		}
		return charsToString(uts.Release[:]), nil
	case "kern.osproductversion":
		return osPrettyName(), nil
	case "kern.hostname":
		return os.Hostname()
	}
	return "", fmt.Errorf("unknown sysctl %s", name)
}

func sysctlIntByName(name string) (int, error) {
	switch name {
	case "hw.memsize":
		total, err := totalMemoryBytes()
		if err != nil {
			return 0, err
		}
		return int(total), nil
	case "hw.pagesize":
		return os.Getpagesize(), nil
	}
	return 0, fmt.Errorf("unknown sysctl %s", name)
}

func charsToString(ca []int8) string {
	b := make([]byte, 0, len(ca))
	for _, c := range ca {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

func osPrettyName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(after, `"`)
		}
	}
	return "Linux"
}

func cpuBrandString() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") || strings.HasPrefix(line, "Model") {
			if _, v, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return runtime.GOARCH
}

func getCPUInfo() map[string]string {
	return map[string]string{
		"machdep.cpu.brand_string": cpuBrandString(),
		"machdep.cpu.core_count":   strconv.Itoa(runtime.NumCPU()),
	}
}

func getPerfLevelCores() map[string]int {
	return map[string]int{"E": 0, "P": runtime.NumCPU(), "S": 0}
}

func getPerfLevelCoresLegacy() map[string]int {
	return getPerfLevelCores()
}

func getGPUCores() string {
	gpus := queryAllGPUs()
	if len(gpus) > 0 {
		return strconv.Itoa(len(gpus))
	}
	return "?"
}

func getTotalRAMGB() int {
	total, err := totalMemoryBytes()
	if err != nil {
		return 0
	}
	return int(total / (1024 * 1024 * 1024))
}

// GetGPUMaxFreqMHz returns the maximum SM clock reported by nvidia-smi.
func GetGPUMaxFreqMHz() int {
	if f := maxGPUFreqMHz(); f > 0 {
		return f
	}
	return 0
}

// ---- Thermal state ----

type thermalStateLevel int

const (
	thermalStateUnknown  thermalStateLevel = -1
	thermalStateNominal  thermalStateLevel = 0
	thermalStateModerate thermalStateLevel = 1
	thermalStateHeavy    thermalStateLevel = 2
	thermalStateTrapping thermalStateLevel = 3
	thermalStateSleeping thermalStateLevel = 4
)

func getThermalStateLevel() thermalStateLevel {
	switch getSocThermalState() {
	case 0:
		return thermalStateNominal
	case 1:
		return thermalStateModerate
	case 2:
		return thermalStateHeavy
	case 3:
		return thermalStateTrapping
	case 4:
		return thermalStateSleeping
	default:
		return thermalStateUnknown
	}
}

func thermalStateString(level thermalStateLevel) string {
	switch level {
	case thermalStateNominal:
		return i18n.T("Metrics_ThermalNominal")
	case thermalStateModerate:
		return i18n.T("Metrics_ThermalModerate")
	case thermalStateHeavy:
		return i18n.T("Metrics_ThermalHeavy")
	case thermalStateTrapping:
		return i18n.T("Metrics_ThermalTrapping")
	case thermalStateSleeping:
		return i18n.T("Metrics_ThermalSleeping")
	default:
		return i18n.T("Metrics_ThermalUnknown")
	}
}

func thermalStateThrottled(level thermalStateLevel) bool {
	return level >= thermalStateModerate
}

func getThermalStateString() (string, bool) {
	level := getThermalStateLevel()
	return thermalStateString(level), thermalStateThrottled(level)
}
