//go:build linux

package app

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func GetCPUUsage() ([]CPUUsage, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var usages []CPUUsage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] == "cpu" {
			continue // skip the aggregate "cpu" line
		}
		user, _ := strconv.ParseFloat(fields[1], 64)
		nice, _ := strconv.ParseFloat(fields[2], 64)
		system, _ := strconv.ParseFloat(fields[3], 64)
		idle, _ := strconv.ParseFloat(fields[4], 64)
		usages = append(usages, CPUUsage{User: user, Nice: nice, System: system, Idle: idle})
	}
	return usages, nil
}

type ProcessTimeState struct {
	Time      uint64
	Timestamp time.Time
	Command   string
	CreateSec int64
}

var (
	prevProcessTimes      = make(map[int]ProcessTimeState)
	prevProcessTimesMutex sync.Mutex
	uidNameCache          = make(map[uint32]string)
	uidNameCacheMutex     sync.RWMutex
)

func linuxUsername(uid uint32) string {
	uidNameCacheMutex.RLock()
	name, ok := uidNameCache[uid]
	uidNameCacheMutex.RUnlock()
	if ok {
		return name
	}
	if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
		name = u.Username
	} else {
		name = strconv.FormatUint(uint64(uid), 10)
	}
	uidNameCacheMutex.Lock()
	uidNameCache[uid] = name
	uidNameCacheMutex.Unlock()
	return name
}

func linuxProcessState(stateByte byte) string {
	switch stateByte {
	case 'R':
		return "R"
	case 'S', 'D':
		return "S"
	case 'T', 't':
		return "T"
	case 'Z':
		return "Z"
	case 'I':
		return "I"
	default:
		return "?"
	}
}

func readProcessUID(pid int) uint32 {
	f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 32)
				return uint32(v)
			}
		}
	}
	return 0
}

func getProcessList(systemGpuPercent float64) ([]ProcessMetrics, error) {
	totalMem, _ := totalMemoryBytes()
	now := time.Now()
	prevProcessTimesMutex.Lock()
	defer prevProcessTimesMutex.Unlock()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	nextProcessTimes := make(map[int]ProcessTimeState)
	var processes []ProcessMetrics

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if filterPID > 0 && pid != filterPID {
			continue
		}

		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		s := string(data)
		open := strings.IndexByte(s, '(')
		closeIdx := strings.LastIndexByte(s, ')')
		if open < 0 || closeIdx < 0 || closeIdx+2 > len(s) {
			continue
		}
		comm := s[open+1 : closeIdx]
		rest := strings.Fields(s[closeIdx+2:])
		if len(rest) < 22 {
			continue
		}

		state := linuxProcessState(rest[0][0])
		utime, _ := strconv.ParseUint(rest[11], 10, 64)
		stime, _ := strconv.ParseUint(rest[12], 10, 64)
		starttime, _ := strconv.ParseUint(rest[19], 10, 64)
		vsize, _ := strconv.ParseUint(rest[20], 10, 64)
		rssPages, _ := strconv.ParseInt(rest[21], 10, 64)

		// /proc/[pid]/stat reports time in USER_HZ (100 ticks/sec).
		totalTicks := utime + stime
		totalNs := totalTicks * (1e9 / 100)
		rssBytes := rssPages * int64(os.Getpagesize())

		cpuPercent := 0.0
		if prev, ok := prevProcessTimes[pid]; ok {
			timeDelta := int64(totalNs) - int64(prev.Time)
			wallDelta := now.Sub(prev.Timestamp).Nanoseconds()
			if wallDelta > 0 && timeDelta > 0 {
				cpuPercent = (float64(timeDelta) / float64(wallDelta)) * 100.0
			}
		}

		memPercent := 0.0
		if totalMem > 0 {
			memPercent = float64(rssBytes) / float64(totalMem) * 100.0
		}

		uid := readProcessUID(pid)
		timeStr := formatTime(float64(totalNs) / 1e9)

		processes = append(processes, ProcessMetrics{
			PID:         pid,
			User:        linuxUsername(uid),
			CPU:         cpuPercent,
			Memory:      memPercent,
			VSZ:         int64(vsize) / 1024,
			RSS:         rssBytes / 1024,
			Command:     comm,
			State:       state,
			Time:        timeStr,
			LastUpdated: now,
		})
		nextProcessTimes[pid] = ProcessTimeState{
			Time:      totalNs,
			Timestamp: now,
			Command:   comm,
			CreateSec: int64(starttime),
		}
	}

	prevProcessTimes = nextProcessTimes

	updateProcessGPUMetrics(processes, now, systemGpuPercent)

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPU > processes[j].CPU
	})

	if filterPID == 0 && len(processes) > 500 {
		processes = processes[:500]
	}

	return processes, nil
}

// updateProcessGPUMetrics attributes system-wide GPU utilization to processes
// with active NVIDIA compute contexts, weighted by their VRAM footprint.
func updateProcessGPUMetrics(processes []ProcessMetrics, now time.Time, systemGpuPercent float64) {
	if systemGpuPercent <= 0 {
		return
	}
	apps := queryNvidiaComputeApps()
	if len(apps) == 0 {
		return
	}
	var total uint64
	for _, m := range apps {
		total += m
	}
	if total == 0 {
		return
	}
	// Convert the system-wide percentage (0-100) into the ms/s scale the UI
	// expects for the GPU column (1000 ms/s == 100%).
	weighted := make(map[int]float64, len(apps))
	for pid, mem := range apps {
		weighted[pid] = systemGpuPercent * 10.0 * (float64(mem) / float64(total))
	}
	for i := range processes {
		if g, ok := weighted[processes[i].PID]; ok {
			processes[i].GPU = g
		}
	}
}

var _ = fmt.Sprintf
