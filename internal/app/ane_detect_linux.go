//go:build linux

package app

import (
	"os"
	"strings"
)

// hasANE reports whether an Apple Neural Engine is present. On Linux this is
// only true on Apple Silicon running a Linux distribution (e.g. Asahi Linux),
// detected from the device tree.
func hasANE() bool { return isAppleHardware() }

// isAppleHardware detects Apple Silicon under Linux via the device tree
// "compatible" property (e.g. "apple,t8103", "apple,j274").
func isAppleHardware() bool {
	paths := []string{
		"/sys/firmware/devicetree/base/compatible",
		"/proc/device-tree/compatible",
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(b)), "apple,") {
			return true
		}
	}
	return false
}
