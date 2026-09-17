//go:build darwin

package app

// platformGPUName returns the marketing name of the GPU, if known. Apple GPUs
// are described by the SoC model, so this is empty on macOS.
func platformGPUName() string { return "" }

// gpuFP32TFLOPs estimates Apple GPU FP32 throughput. Each GPU core issues
// 256 FLOPs/cycle; 0.000256 = 256 / 1e6 when clock is in MHz.
func gpuFP32TFLOPs(gpuCoreCount, maxFreqMHz int) float64 {
	if maxFreqMHz <= 0 || gpuCoreCount <= 0 {
		return 0
	}
	return float64(gpuCoreCount) * float64(maxFreqMHz) * 0.000256
}

func gpuFP16TFLOPs(gpuCoreCount, maxFreqMHz int) float64 {
	return gpuFP32TFLOPs(gpuCoreCount, maxFreqMHz) * 2
}
