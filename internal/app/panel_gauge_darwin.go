//go:build darwin

package app

// showGPUTempGauge reports whether the GPU temperature gauge should occupy the
// secondary gauge slot. On macOS the ANE gauge fills that slot on Apple
// Silicon, so the GPU temperature gauge is never used.
func showGPUTempGauge() bool {
	return false
}
