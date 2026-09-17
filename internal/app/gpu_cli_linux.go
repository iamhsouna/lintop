//go:build linux

package app

import (
	"fmt"
	"strings"

	w "github.com/metaspartan/gotui/v5/widgets"
)

// handleGPUCliFlags processes --list-gpus. Returns true when it handled the
// request and the program should exit.
func handleGPUCliFlags() bool {
	if !listGPUs {
		return false
	}
	gpus := queryAllGPUs()
	if len(gpus) == 0 {
		fmt.Println("No GPUs detected (looked for nvidia-smi, amdgpu sysfs and Intel DRM).")
		return true
	}
	fmt.Printf("%-4s %-7s %-30s %8s %12s %8s %8s\n", "IDX", "VENDOR", "NAME", "UTIL%", "VRAM(MB)", "TEMP", "WATTS")
	for _, g := range gpus {
		fmt.Printf("%-4d %-7s %-30s %8.1f %12.0f %8.1f %8.1f\n",
			g.Index, g.Vendor, truncateName(g.Name, 30), g.UtilPct, g.MemUsedMB, g.TempC, g.PowerW)
	}
	return true
}

func truncateName(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return strings.TrimSpace(s[:n-3]) + "..."
}

// initMultiGPUGauges creates one gauge widget per detected GPU for the
// multi_gpu layout.
func initMultiGPUGauges() {
	gpus := queryAllGPUs()
	multiGpuGauges = nil
	for i, g := range gpus {
		gauge := w.NewGauge()
		gauge.BorderRounded = true
		gauge.Percent = 0
		label := g.Name
		if label == "" {
			label = gpuVendorLabel(g)
		}
		gauge.Title = fmt.Sprintf(" GPU%d · %s ", i, label)
		multiGpuGauges = append(multiGpuGauges, gauge)
	}
}
