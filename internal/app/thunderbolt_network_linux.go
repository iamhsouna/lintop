//go:build linux

package app

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ThunderboltNetStats holds per-interface stats for a Thunderbolt network interface
type ThunderboltNetStats struct {
	InterfaceName  string  `json:"interface_name"`
	BytesIn        uint64  `json:"bytes_in"`
	BytesOut       uint64  `json:"bytes_out"`
	BytesInPerSec  float64 `json:"bytes_in_per_sec"`
	BytesOutPerSec float64 `json:"bytes_out_per_sec"`
	PacketsIn      uint64  `json:"packets_in"`
	PacketsOut     uint64  `json:"packets_out"`
}

var (
	tbNetMutex          sync.Mutex
	lastTBNetStats      map[string]NativeNetMetric
	lastTBNetUpdateTime time.Time
)

// linuxThunderboltInterfaces identifies Thunderbolt/USB4 network interfaces.
func linuxThunderboltInterfaces() map[string]bool {
	members := make(map[string]bool)
	entries, _ := filepath.Glob("/sys/class/net/*")
	for _, e := range entries {
		name := filepath.Base(e)
		if strings.HasPrefix(name, "tb") || strings.Contains(strings.ToLower(name), "thunderbolt") {
			members[name] = true
			continue
		}
		// USB4/Thunderbolt net devices expose a device link into the tb tree.
		if target, err := filepath.EvalSymlinks(filepath.Join(e, "device")); err == nil {
			if strings.Contains(target, "thunderbolt") {
				members[name] = true
			}
		}
	}
	return members
}

// GetThunderboltNetStats returns network statistics for Thunderbolt interfaces.
func GetThunderboltNetStats() []ThunderboltNetStats {
	tbNetMutex.Lock()
	defer tbNetMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(lastTBNetUpdateTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	statsMap, err := GetNativeNetworkMetrics()
	if err != nil {
		return nil
	}

	members := linuxThunderboltInterfaces()
	var result []ThunderboltNetStats

	currentStats := make(map[string]NativeNetMetric)
	for name, stat := range statsMap {
		if !members[name] {
			continue
		}
		currentStats[name] = stat
		tbStat := ThunderboltNetStats{
			InterfaceName: name,
			BytesIn:       stat.BytesRecv,
			BytesOut:      stat.BytesSent,
			PacketsIn:     stat.PacketsRecv,
			PacketsOut:    stat.PacketsSent,
		}
		if prev, ok := lastTBNetStats[name]; ok && !lastTBNetUpdateTime.IsZero() {
			if stat.BytesRecv >= prev.BytesRecv {
				tbStat.BytesInPerSec = float64(stat.BytesRecv-prev.BytesRecv) / elapsed
			}
			if stat.BytesSent >= prev.BytesSent {
				tbStat.BytesOutPerSec = float64(stat.BytesSent-prev.BytesSent) / elapsed
			}
		}
		result = append(result, tbStat)
	}

	lastTBNetStats = currentStats
	lastTBNetUpdateTime = now
	return result
}

// WiFiLinkInfo represents Wi-Fi interface link information
type WiFiLinkInfo struct {
	InterfaceName  string
	PHYMode        string
	WiFiGeneration string
	TxRateMbps     int
	IsConnected    bool
}

// GetWiFiLinkInfo returns Wi-Fi link info using iw, or nil if unavailable.
func GetWiFiLinkInfo() *WiFiLinkInfo {
	entries, _ := filepath.Glob("/sys/class/net/*/wireless")
	if len(entries) == 0 {
		return nil
	}
	iface := filepath.Base(filepath.Dir(entries[0]))

	info := &WiFiLinkInfo{InterfaceName: iface}
	if readTrim(filepath.Join("/sys/class/net", iface, "operstate")) == "up" {
		info.IsConnected = true
	}

	if _, err := exec.LookPath("iw"); err != nil {
		return info
	}
	out, err := exec.Command("iw", "dev", iface, "link").Output()
	if err != nil {
		return info
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "tx bitrate:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				if v, err := parseFloatPrefix(fields[2]); err == nil {
					info.TxRateMbps = int(v)
				}
			}
		}
	}
	return info
}

func parseFloatPrefix(s string) (float64, error) {
	end := 0
	for end < len(s) && (s[end] == '.' || (s[end] >= '0' && s[end] <= '9')) {
		end++
	}
	if end == 0 {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(s[:end], 64)
}
