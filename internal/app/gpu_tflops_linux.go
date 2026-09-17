//go:build linux

package app

import "strings"

// platformGPUName returns the first NVIDIA GPU's marketing name, if present.
func platformGPUName() string {
	gpus := queryNvidiaGPUs()
	if len(gpus) > 0 {
		return gpus[0].Name
	}
	return ""
}

// nvidiaCUDACores returns the FP32 CUDA core count for well-known NVIDIA GPUs.
// Returns 0 when unknown (TFLOPs are then simply omitted).
func nvidiaCUDACores(name string) int {
	lookup := []struct {
		match string
		cores int
	}{
		{"RTX 5090", 21760}, {"RTX 5080", 10752}, {"RTX 5070 Ti", 8960}, {"RTX 5070", 6144},
		{"RTX 4090", 16384}, {"RTX 4080", 9728}, {"RTX 4070 Ti", 7680}, {"RTX 4070", 5888}, {"RTX 4060", 3072},
		{"RTX 3090 Ti", 10752}, {"RTX 3090", 10496}, {"RTX 3080 Ti", 10240}, {"RTX 3080", 8704},
		{"RTX 3070 Ti", 6144}, {"RTX 3070", 5888}, {"RTX 3060 Ti", 4864}, {"RTX 3060", 3584},
		{"RTX 3050", 2560},
		{"RTX 2080 Ti", 4352}, {"RTX 2080", 2944}, {"RTX 2070", 2304}, {"RTX 2060", 1920},
		{"GTX 1080 Ti", 3584}, {"GTX 1080", 2560}, {"GTX 1070", 1920}, {"GTX 1060", 1280},
		{"TITAN RTX", 4608}, {"TITAN V", 5120},
		{"A100", 6912}, {"H100", 16896}, {"A40", 10752}, {"A30", 3584},
		{"A10", 9216}, {"A6000", 10752}, {"A5000", 8192}, {"A4000", 6144},
		{"L40", 18176}, {"L4", 7680}, {"RTX A6000", 10752}, {"RTX A5000", 8192},
		{"V100", 5120}, {"T4", 2560},
	}
	upper := strings.ToUpper(name)
	for _, e := range lookup {
		if strings.Contains(upper, strings.ToUpper(e.match)) {
			return e.cores
		}
	}
	return 0
}

// gpuFP32TFLOPs estimates raw FP32 throughput. On NVIDIA each CUDA core issues
// 2 FLOPs/cycle; the boost clock comes from nvidia-smi.
func gpuFP32TFLOPs(gpuCoreCount, maxFreqMHz int) float64 {
	if maxFreqMHz <= 0 {
		return 0
	}
	gpus := queryNvidiaGPUs()
	if len(gpus) == 0 {
		return 0
	}
	cores := 0
	for _, g := range gpus {
		gc := nvidiaCUDACores(g.Name)
		if gc == 0 {
			return 0
		}
		cores += gc
		// Use each GPU's own clock when available.
	}
	clock := maxFreqMHz
	if gpus[0].MaxFreqMHz > 0 {
		clock = gpus[0].MaxFreqMHz
	}
	return float64(cores) * 2.0 * float64(clock) * 1e-6
}

func gpuFP16TFLOPs(gpuCoreCount, maxFreqMHz int) float64 {
	return gpuFP32TFLOPs(gpuCoreCount, maxFreqMHz) * 2
}
