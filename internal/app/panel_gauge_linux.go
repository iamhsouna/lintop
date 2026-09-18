//go:build linux

package app

// showGPUTempGauge reports whether the GPU temperature gauge should occupy the
// secondary gauge slot (the slot the ANE gauge uses on Apple Silicon). On
// Linux this is only when no Apple Neural Engine is present but a GPU is.
func showGPUTempGauge() bool {
	return !hasANE() && len(queryAllGPUs()) > 0
}
