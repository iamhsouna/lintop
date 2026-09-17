//go:build linux

package app

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

type NativeMemoryMetrics struct {
	Total     uint64
	Used      uint64
	Available uint64
	SwapTotal uint64
	SwapUsed  uint64
}

// totalMemoryBytes returns MemTotal from /proc/meminfo.
func totalMemoryBytes() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb * 1024, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}

func meminfoValue(key string) uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key+":") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

func GetNativeMemoryMetrics() (NativeMemoryMetrics, error) {
	total, err := totalMemoryBytes()
	if err != nil {
		return NativeMemoryMetrics{}, err
	}
	available := meminfoValue("MemAvailable")
	swapTotal := meminfoValue("SwapTotal")
	swapFree := meminfoValue("SwapFree")
	used := uint64(0)
	if total > available {
		used = total - available
	}
	return NativeMemoryMetrics{
		Total:     total,
		Used:      used,
		Available: available,
		SwapTotal: swapTotal,
		SwapUsed:  swapTotal - swapFree,
	}, nil
}

// GetNativeUptime returns system uptime in seconds.
func GetNativeUptime() (uint64, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0, fmt.Errorf("invalid /proc/uptime")
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return uint64(secs), nil
}

type NativeDiskUsage struct {
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

type NativePartitionInfo struct {
	Device     string
	Mountpoint string
	Fstype     string
}

func GetNativePartitions(all bool) ([]NativePartitionInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var parts []NativePartitionInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		if !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		parts = append(parts, NativePartitionInfo{
			Device:     fields[0],
			Mountpoint: unescapeMount(fields[1]),
			Fstype:     fields[2],
		})
	}
	return parts, nil
}

func GetNativeDiskUsage(path string) (NativeDiskUsage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return NativeDiskUsage{}, err
	}
	bs := uint64(st.Bsize)
	total := st.Blocks * bs
	free := st.Bfree * bs
	avail := st.Bavail * bs
	used := total - free
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return NativeDiskUsage{Total: total, Used: used, Free: avail, UsedPercent: pct}, nil
}

type NativeNetMetric struct {
	Name        string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

func GetNativeNetworkMetrics() (map[string]NativeNetMetric, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	metrics := make(map[string]NativeNetMetric)
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		line := scanner.Text()
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "lo" {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 16 {
			continue
		}
		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[1], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[9], 10, 64)
		metrics[name] = NativeNetMetric{
			Name:        name,
			BytesRecv:   rxBytes,
			PacketsRecv: rxPackets,
			BytesSent:   txBytes,
			PacketsSent: txPackets,
		}
	}
	return metrics, nil
}

type NativeDiskMetric struct {
	Name       string
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
	ReadTime   uint64
	WriteTime  uint64
}

func GetNativeDiskMetrics() (map[string]NativeDiskMetric, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result := make(map[string]NativeDiskMetric)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		// Skip partitions and virtual devices to avoid double counting.
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "md") {
			continue
		}
		reads, _ := strconv.ParseUint(fields[3], 10, 64)
		sectorsRead, _ := strconv.ParseUint(fields[5], 10, 64)
		readTime, _ := strconv.ParseUint(fields[6], 10, 64)
		writes, _ := strconv.ParseUint(fields[7], 10, 64)
		sectorsWritten, _ := strconv.ParseUint(fields[9], 10, 64)
		writeTime, _ := strconv.ParseUint(fields[10], 10, 64)
		result[name] = NativeDiskMetric{
			Name:       name,
			ReadBytes:  sectorsRead * 512,
			WriteBytes: sectorsWritten * 512,
			ReadOps:    reads,
			WriteOps:   writes,
			ReadTime:   readTime,
			WriteTime:  writeTime,
		}
	}
	return result, nil
}

type NativeHostInfo struct {
	Hostname      string
	OSVersion     string
	KernelVersion string
	Uptime        uint64
	BootTime      uint64
}

func GetNativeHostInfo() (NativeHostInfo, error) {
	hostname, _ := os.Hostname()
	kernel, _ := sysctlStringByName("kern.osrelease")
	osVersion, _ := sysctlStringByName("kern.osproductversion")
	uptime, _ := GetNativeUptime()
	now := time.Now().Unix()
	return NativeHostInfo{
		Hostname:      hostname,
		OSVersion:     osVersion,
		KernelVersion: kernel,
		Uptime:        uptime,
		BootTime:      uint64(now) - uptime,
	}, nil
}

// ---- Core topology ----

type CoreType int

const (
	CoreTypeUnknown CoreType = 0
	CoreTypeE       CoreType = 1
	CoreTypeP       CoreType = 2
	CoreTypeS       CoreType = 3
	CoreTypeM       CoreType = 4
)

type CoreTopologyEntry struct {
	CPUID    int
	CoreType CoreType
}

// GetCoreTopology returns a uniform P-core topology for Linux CPUs.
func GetCoreTopology() ([]CoreTopologyEntry, error) {
	n := runtime.NumCPU()
	result := make([]CoreTopologyEntry, n)
	for i := 0; i < n; i++ {
		result[i] = CoreTopologyEntry{CPUID: i, CoreType: CoreTypeP}
	}
	return result, nil
}

// BuildCoreLabels returns nil so the generic P-core label fallback is used.
func BuildCoreLabels() ([]string, int, int, int, []int) {
	return nil, 0, 0, 0, nil
}

// ---- GPU ----

type GPUProcessStat struct {
	PID       int
	GPUTimeNs uint64
}

// GetGPUProcessStats returns nil: per-process GPU time is not exposed by
// nvidia-smi in a directly comparable counter.
func GetGPUProcessStats() map[int]uint64 { return nil }

func GetGPUCoreCountFast() int { return len(queryAllGPUs()) }

func GetMaxGPUFrequency() int { return maxGPUFreqMHz() }

// ---- Thunderbolt / USB / Storage (Linux versions of the IOKit queries) ----

type ThunderboltSwitchInfo struct {
	UID                uint64
	ParentUID          uint64
	RouterID           int
	VendorID           int
	DeviceID           int
	VendorName         string
	DeviceName         string
	PortCount          int
	Depth              int
	ThunderboltVersion int
	LinkSpeed          uint64
	CurrentSpeed       uint64
	LinkWidth          int
}

func GetThunderboltSwitchesIOKit() []ThunderboltSwitchInfo { return nil }

type USBDeviceInfo struct {
	VendorID    int
	ProductID   int
	LocationID  uint32
	VendorName  string
	ProductName string
	Serial      string
}

func GetUSBDevicesIOKit() []USBDeviceInfo { return nil }

type StorageDeviceInfo struct {
	Name       string
	BSDName    string
	Protocol   string
	MediumType string
	IsInternal bool
	IsWhole    bool
	SizeBytes  uint64
}

// GetStorageDevicesIOKit returns whole block devices from /sys/block.
func GetStorageDevicesIOKit() []StorageDeviceInfo {
	var result []StorageDeviceInfo
	entries, _ := filepath.Glob("/sys/block/*")
	for _, e := range entries {
		name := filepath.Base(e)
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		info := StorageDeviceInfo{Name: name, BSDName: name, IsWhole: true}
		if b, err := os.ReadFile(filepath.Join(e, "size")); err == nil {
			if sectors, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); err == nil {
				info.SizeBytes = sectors * 512
			}
		}
		result = append(result, info)
	}
	return result
}

// ---- Ethernet / Wi-Fi link info ----

type EthernetLinkInfo struct {
	Name          string
	LinkUp        bool
	LinkSpeedMbps uint64
	MediaType     string
}

func GetEthernetLinkInfo() []EthernetLinkInfo {
	var infos []EthernetLinkInfo
	entries, _ := filepath.Glob("/sys/class/net/*")
	for _, e := range entries {
		name := filepath.Base(e)
		if name == "lo" {
			continue
		}
		if _, err := os.Stat(filepath.Join(e, "wireless")); err == nil {
			continue // Wi-Fi handled separately
		}
		info := EthernetLinkInfo{Name: name}
		if state := readTrim(filepath.Join(e, "operstate")); state == "up" {
			info.LinkUp = true
		}
		if speed := readTrim(filepath.Join(e, "speed")); speed != "" {
			if mbps, err := strconv.ParseUint(speed, 10, 64); err == nil && mbps < 1<<32 {
				info.LinkSpeedMbps = mbps
			}
		}
		infos = append(infos, info)
	}
	return infos
}

func FormatLinkSpeed(mbps uint64) string {
	switch {
	case mbps >= 10000:
		return fmt.Sprintf("%.0fGbE", float64(mbps)/1000)
	case mbps >= 1000:
		if mbps%1000 == 0 {
			return fmt.Sprintf("%dGbE", mbps/1000)
		}
		return fmt.Sprintf("%.1fGbE", float64(mbps)/1000)
	case mbps > 0:
		return fmt.Sprintf("%dMbps", mbps)
	default:
		return "--"
	}
}

var _ = sync.Once{}
