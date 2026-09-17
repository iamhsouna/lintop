//go:build darwin

package app

// handleGPUCliFlags has nothing to do on macOS; the Apple GPU is handled by
// the SoC metrics path.
func handleGPUCliFlags() bool { return false }

// initMultiGPUGauges is a no-op on macOS (Apple GPUs are part of the SoC).
func initMultiGPUGauges() {}
