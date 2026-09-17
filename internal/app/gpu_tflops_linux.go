//go:build linux

package app

import "strings"

// platformGPUName returns the selected GPU's marketing name, if present.
func platformGPUName() string {
	if g, ok := primaryGPU(); ok {
		return g.Name
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
	return lookupCores(name, lookup)
}

// amdStreamProcessors returns the shader (stream processor) count for common
// AMD GPUs. Returns 0 when unknown.
func amdStreamProcessors(name string) int {
	lookup := []struct {
		match string
		cores int
	}{
		{"7900 XTX", 6144}, {"7900 XT", 5376}, {"7900 GRE", 5120},
		{"7800 XT", 3840}, {"7700 XT", 3456}, {"7600", 2048},
		{"6950 XT", 5120}, {"6900 XT", 5120}, {"6800 XT", 4608}, {"6800", 3840},
		{"6750 XT", 2560}, {"6700 XT", 2560}, {"6650 XT", 2048}, {"6600 XT", 2048},
		{"6600", 1792}, {"6500 XT", 1024}, {"6400", 768},
		{"5700 XT", 2560}, {"5700", 2304}, {"5600 XT", 2304},
		{"Vega 64", 4096}, {"Vega 56", 3584}, {"Radeon VII", 3840},
		{"MI250", 13312}, {"MI210", 6656}, {"MI100", 7680}, {"MI50", 3840},
		{"W7900", 6144}, {"W7800", 5120}, {"W6800", 3840},
	}
	return lookupCores(name, lookup)
}

// intelExecutionUnits returns the Xe EU count for common Intel GPUs.
func intelExecutionUnits(name string) int {
	lookup := []struct {
		match string
		cores int
	}{
		{"Arc A770", 512}, {"Arc A750", 448}, {"Arc A580", 384},
		{"Arc A380", 128}, {"Arc A310", 96},
		{"Arc B580", 160}, {"Arc B570", 144},
		{"Iris Xe", 96}, {"UHD Graphics", 32}, {"HD Graphics", 24},
	}
	return lookupCores(name, lookup)
}

func lookupCores(name string, lookup []struct {
	match string
	cores int
}) int {
	upper := strings.ToUpper(name)
	for _, e := range lookup {
		if strings.Contains(upper, strings.ToUpper(e.match)) {
			return e.cores
		}
	}
	return 0
}

// gpuFP32TFLOPs estimates raw FP32 throughput across every detected GPU.
//   - NVIDIA CUDA core: 2 FLOP/cycle
//   - AMD stream processor: 2 FLOP/cycle
//   - Intel Xe EU (8 FP32 lanes): 16 FLOP/cycle
func gpuFP32TFLOPs(gpuCoreCount, maxFreqMHz int) float64 {
	gpus := queryAllGPUs()
	if len(gpus) == 0 {
		return 0
	}
	var total float64
	for _, g := range gpus {
		clock := g.MaxFreqMHz
		if clock == 0 {
			clock = g.FreqMHz
		}
		if clock == 0 || g.Name == "" {
			continue
		}
		var flops float64
		switch g.Vendor {
		case "nvidia":
			flops = float64(nvidiaCUDACores(g.Name)) * 2.0
		case "amd":
			flops = float64(amdStreamProcessors(g.Name)) * 2.0
		case "intel":
			flops = float64(intelExecutionUnits(g.Name)) * 16.0
		}
		total += flops * float64(clock) * 1e-6
	}
	return total
}

func gpuFP16TFLOPs(gpuCoreCount, maxFreqMHz int) float64 {
	return gpuFP32TFLOPs(gpuCoreCount, maxFreqMHz) * 2
}
