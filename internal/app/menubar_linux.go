//go:build linux

package app

// Linux has no equivalent of the macOS menu bar or floating overlay HUD. The
// flags are accepted for compatibility and these workers are inert.

import (
	"encoding/json"
	"io"
	"os/exec"
	"sync"
)

type MenuBarMetricsPayload struct {
	SocMetrics    SocMetrics     `json:"soc_metrics"`
	CPUMetrics    CPUMetrics     `json:"cpu_metrics"`
	GPUMetrics    GPUMetrics     `json:"gpu_metrics"`
	NetDisk       NetDiskMetrics `json:"net_disk"`
	SystemInfo    SystemInfo     `json:"system_info"`
	MaxFP32TFLOPs float64        `json:"max_fp32_tflops"`
	CPUPercent    float64        `json:"cpu_percent"`
	ThermalState  string         `json:"thermal_state"`
	RDMAStatus    string         `json:"rdma_status"`
}

var (
	overlayMu             sync.Mutex
	overlayWorkerStdin    io.WriteCloser
	overlayWorkerCmd      *exec.Cmd
	overlayMetricsEncoder *json.Encoder

	menubarMu             sync.Mutex
	menubarWorkerStdin    io.WriteCloser
	menubarWorkerCmd      *exec.Cmd
	menubarMetricsEncoder *json.Encoder
)

func startMenuBarWorker() {}

func startOverlayWorker() {}

func startMenuBarProcess() error { return nil }

func startOverlayProcess() error { return nil }

func pushMenuBarMetricsToWorker(sm SocMetrics, cpuMetrics CPUMetrics, gpuMetrics GPUMetrics, netDisk NetDiskMetrics, sysInfo SystemInfo, maxFP32TFLOPs float64, cpuPercent float64, thermalState string, rdmaStatus string) {
}

func pushOverlayMetrics(sm SocMetrics, cpuMetrics CPUMetrics, gpuMetrics GPUMetrics, netDisk NetDiskMetrics, sysInfo SystemInfo, maxFP32TFLOPs float64, cpuPercent float64, thermalState string, rdmaStatus string) {
}
