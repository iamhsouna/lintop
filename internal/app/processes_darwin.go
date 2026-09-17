//go:build darwin

package app

/*
#include <sys/sysctl.h>
#include <pwd.h>
#include <unistd.h>
#include <libproc.h>
#include <mach/mach_host.h>
#include <mach/processor_info.h>
#include <mach/mach_init.h>
#include <mach/mach_time.h>

static inline time_t get_proc_starttime(struct kinfo_proc *kp) {
    return kp->kp_proc.p_un.__p_starttime.tv_sec;
}

extern kern_return_t vm_deallocate(vm_map_t target_task, vm_address_t address, vm_size_t size);
*/
import "C"
import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"syscall"
	"time"
	"unsafe"
)

var uidCache = make(map[uint32]string)
var uidCacheMutex sync.RWMutex

func getUsername(uid uint32) string {
	uidCacheMutex.RLock()
	name, ok := uidCache[uid]
	uidCacheMutex.RUnlock()
	if ok {
		return name
	}

	uidCacheMutex.Lock()
	defer uidCacheMutex.Unlock()

	// Double check
	if name, ok := uidCache[uid]; ok {
		return name
	}

	// Use C.getpwuid
	pwd := C.getpwuid(C.uid_t(uid))
	if pwd != nil {
		name = C.GoString(pwd.pw_name)
	} else {
		name = fmt.Sprintf("%d", uid)
	}
	uidCache[uid] = name
	return name
}

type ProcessTimeState struct {
	Time      uint64
	Timestamp time.Time
	Command   string
	CreateSec int64
}

var prevProcessTimes = make(map[int]ProcessTimeState)
var prevProcessTimesMutex sync.Mutex

var timebaseInfo C.mach_timebase_info_data_t
var timebaseOnce sync.Once

func getTimebase() {
	C.mach_timebase_info(&timebaseInfo)
}

func processOsProc(kp C.struct_kinfo_proc, now time.Time, prevProcessTimes map[int]ProcessTimeState, totalMem uint64, numer, denom uint64) (ProcessMetrics, int, ProcessTimeState, bool) {
	pid := int(kp.kp_proc.p_pid)
	if pid == 0 {
		return ProcessMetrics{}, 0, ProcessTimeState{}, false
	}

	comm := C.GoString(&kp.kp_proc.p_comm[0])
	createSec := int64(C.get_proc_starttime(&kp))

	// Fast path: reuse full command name to avoid heavy proc_pidpath syscall on every tick.
	// p_comm is a 16-byte truncation of the full name, so check if cached command starts with it.
	if prevState, ok := prevProcessTimes[pid]; ok && prevState.CreateSec == createSec && prevState.Command != "" && strings.HasPrefix(prevState.Command, comm) {
		comm = prevState.Command
	} else {
		var pathBuf [C.PROC_PIDPATHINFO_MAXSIZE]C.char
		if C.proc_pidpath(C.int(pid), unsafe.Pointer(&pathBuf), C.PROC_PIDPATHINFO_MAXSIZE) > 0 {
			fullPath := C.GoString(&pathBuf[0])
			comm = filepath.Base(fullPath)
		}
	}

	rssBytes := int64(0)
	vszBytes := int64(0)
	totalTimeNs := uint64(0)

	var taskInfo C.struct_proc_taskinfo
	ret := C.proc_pidinfo(C.int(pid), C.PROC_PIDTASKINFO, 0, unsafe.Pointer(&taskInfo), C.int(C.sizeof_struct_proc_taskinfo))
	if ret == C.int(C.sizeof_struct_proc_taskinfo) {
		rssBytes = int64(taskInfo.pti_resident_size)
		vszBytes = int64(taskInfo.pti_virtual_size)
		rawTime := uint64(taskInfo.pti_total_user) + uint64(taskInfo.pti_total_system)
		totalTimeNs = (rawTime * numer) / denom
	}

	cpuPercent := 0.0
	if prevState, ok := prevProcessTimes[pid]; ok {
		timeDelta := totalTimeNs - prevState.Time
		wallDelta := now.Sub(prevState.Timestamp).Nanoseconds()
		if wallDelta > 0 && timeDelta > 0 {
			cpuPercent = (float64(timeDelta) / float64(wallDelta)) * 100.0
		}
	}

	newState := ProcessTimeState{
		Time:      totalTimeNs,
		Timestamp: now,
		Command:   comm,
		CreateSec: createSec,
	}

	memPercent := 0.0
	if totalMem > 0 {
		memPercent = (float64(rssBytes) / float64(totalMem)) * 100.0
	}

	state := processStateString(kp.kp_proc.p_stat)

	uid := uint32(kp.kp_eproc.e_ucred.cr_uid)
	user := getUsername(uid)

	totalSeconds := float64(totalTimeNs) / 1e9
	timeStr := formatTime(totalSeconds)

	pm := ProcessMetrics{
		PID:         pid,
		User:        user,
		CPU:         cpuPercent,
		Memory:      memPercent,
		VSZ:         vszBytes / 1024,
		RSS:         rssBytes / 1024,
		Command:     comm,
		State:       state,
		Started:     "",
		Time:        timeStr,
		LastUpdated: now,
	}
	return pm, pid, newState, true
}

func processStateString(stat C.char) string {
	switch stat {
	case C.SIDL:
		return "I"
	case C.SRUN:
		return "R"
	case C.SSLEEP:
		return "S"
	case C.SSTOP:
		return "T"
	case C.SZOMB:
		return "Z"
	default:
		return "?"
	}
}

func getProcessList(systemGpuPercent float64) ([]ProcessMetrics, error) {
	var mib []C.int
	var mibLen C.u_int

	if filterPID > 0 {
		// Fetch only the specific process by PID
		mib = []C.int{C.CTL_KERN, C.KERN_PROC, C.KERN_PROC_PID, C.int(filterPID)}
		mibLen = 4
	} else {
		// Fetch all processes
		mib = []C.int{C.CTL_KERN, C.KERN_PROC, C.KERN_PROC_ALL}
		mibLen = 3
	}
	var size C.size_t

	if _, err := C.sysctl(&mib[0], mibLen, nil, &size, nil, 0); err != nil {
		return nil, fmt.Errorf("sysctl size check failed: %v", err)
	}

	if size == 0 {
		// PID not found or no processes
		return nil, nil
	}

	buf := make([]byte, size)
	if _, err := C.sysctl(&mib[0], mibLen, unsafe.Pointer(&buf[0]), &size, nil, 0); err != nil {
		return nil, fmt.Errorf("sysctl fetch failed: %v", err)
	}

	count := int(size) / int(C.sizeof_struct_kinfo_proc)
	kprocs := (*[1 << 30]C.struct_kinfo_proc)(unsafe.Pointer(&buf[0]))[:count:count]

	var processes []ProcessMetrics
	now := time.Now()

	prevProcessTimesMutex.Lock()
	defer prevProcessTimesMutex.Unlock()

	nextProcessTimes := make(map[int]ProcessTimeState)

	mibMem := []C.int{6, 24}
	var memSize C.uint64_t
	memLen := C.size_t(unsafe.Sizeof(memSize))
	totalMem := uint64(0)
	if _, err := C.sysctl(&mibMem[0], 2, unsafe.Pointer(&memSize), &memLen, nil, 0); err == nil {
		totalMem = uint64(memSize)
	}

	timebaseOnce.Do(getTimebase)
	numer := uint64(timebaseInfo.numer)
	denom := uint64(timebaseInfo.denom)
	if denom == 0 {
		denom = 1
	}

	for _, kp := range kprocs {
		pm, pid, ns, ok := processOsProc(kp, now, prevProcessTimes, totalMem, numer, denom)
		if ok {
			processes = append(processes, pm)
			nextProcessTimes[pid] = ns
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

// updateProcessGPUMetrics calculates per-process GPU usage and updates process metrics
func updateProcessGPUMetrics(processes []ProcessMetrics, now time.Time, systemGpuPercent float64) {
	gpuProcessStatsMutex.Lock()
	defer gpuProcessStatsMutex.Unlock()

	currentGPUStats := GetGPUProcessStats()
	gpuElapsed := now.Sub(lastGPUProcessStatsTime).Seconds()
	if gpuElapsed <= 0 {
		gpuElapsed = 1
	}

	gpuMsPerSec := make(map[int]float64)
	var totalRawGpuMs float64
	if currentGPUStats != nil && lastGPUProcessStats != nil {
		for pid, currentTime := range currentGPUStats {
			if prevTime, ok := lastGPUProcessStats[pid]; ok && currentTime >= prevTime {
				deltaNs := currentTime - prevTime
				gpuMs := float64(deltaNs) / gpuElapsed / 1_000_000
				gpuMsPerSec[pid] = gpuMs
				totalRawGpuMs += gpuMs
			}
		}
	}

	rawTotalPercent := totalRawGpuMs / 10.0

	scaleFactor := 1.0
	if rawTotalPercent > 0.01 && systemGpuPercent > 0.01 {
		scaleFactor = systemGpuPercent / rawTotalPercent
	}

	for i := range processes {
		if gpuMs, ok := gpuMsPerSec[processes[i].PID]; ok {
			processes[i].GPU = gpuMs * scaleFactor
		}
	}

	lastGPUProcessStats = currentGPUStats
	lastGPUProcessStatsTime = now
}

func GetCPUUsage() ([]CPUUsage, error) {
	var numCPUs C.natural_t
	var cpuLoad *C.processor_cpu_load_info_data_t
	var cpuMsgCount C.mach_msg_type_number_t
	host := C.mach_host_self()
	kernReturn := C.host_processor_info(
		host,
		C.PROCESSOR_CPU_LOAD_INFO,
		&numCPUs,
		(*C.processor_info_array_t)(unsafe.Pointer(&cpuLoad)),
		&cpuMsgCount,
	)
	if kernReturn != C.KERN_SUCCESS {
		return nil, fmt.Errorf("error getting CPU info: %d", kernReturn)
	}
	defer C.vm_deallocate(
		C.mach_task_self_,
		(C.vm_address_t)(uintptr(unsafe.Pointer(cpuLoad))),
		C.vm_size_t(cpuMsgCount)*C.sizeof_processor_cpu_load_info_data_t,
	)
	cpuLoadInfo := (*[1 << 30]C.processor_cpu_load_info_data_t)(unsafe.Pointer(cpuLoad))[:numCPUs:numCPUs]
	cpuUsage := make([]CPUUsage, numCPUs)
	for i := 0; i < int(numCPUs); i++ {
		cpuUsage[i] = CPUUsage{
			User:   float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_USER]),
			System: float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_SYSTEM]),
			Idle:   float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_IDLE]),
			Nice:   float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_NICE]),
		}
	}
	return cpuUsage, nil
}
