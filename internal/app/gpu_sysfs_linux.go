//go:build linux

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var cardNameRe = regexp.MustCompile(`^card\d+$`)

// readUint reads a decimal unsigned integer from a sysfs file.
func readUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// readUintHex reads a "0x...." value from a sysfs file.
func readUintHex(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(b)), "0x")), 16, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// pciSlotName returns the PCI BDF for a DRM card device.
func pciSlotName(deviceDir string) string {
	b, err := os.ReadFile(filepath.Join(deviceDir, "uevent"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if after, ok := strings.CutPrefix(line, "PCI_SLOT_NAME="); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// pciDeviceName uses lspci to resolve a marketing name for a PCI device.
func pciDeviceName(bdf string) string {
	if bdf == "" {
		return ""
	}
	if _, err := exec.LookPath("lspci"); err != nil {
		return ""
	}
	out, err := exec.Command("lspci", "-m", "-s", bdf).Output()
	if err != nil {
		return ""
	}
	fields := parseQuotedFields(string(out))
	// lspci -m emits: slot "class" "vendor" "device" "svendor" "sdevice".
	if len(fields) >= 3 {
		return fields[2]
	}
	if len(fields) >= 1 {
		return fields[len(fields)-1]
	}
	return ""
}

// parseQuotedFields extracts the contents of each double-quoted field.
func parseQuotedFields(s string) []string {
	parts := strings.Split(s, `"`)
	var fields []string
	for i := 1; i < len(parts); i += 2 {
		fields = append(fields, parts[i])
	}
	return fields
}

// hwmonDeviceDirs lists the hwmon directories exposed by a PCI device.
func hwmonDeviceDirs(deviceDir string) []string {
	dirs, _ := filepath.Glob(filepath.Join(deviceDir, "hwmon", "hwmon*"))
	return dirs
}

// maxHwmonMicroWatts averages the power sensors exposed by a device.
func hwmonPowerWatts(deviceDir string) float64 {
	for _, hw := range hwmonDeviceDirs(deviceDir) {
		// power1_average is in microwatts on amdgpu/intel hwmon.
		if v, ok := readUint(filepath.Join(hw, "power1_average")); ok {
			return float64(v) / 1e6
		}
		if v, ok := readUint(filepath.Join(hw, "power1_input")); ok {
			return float64(v) / 1e6
		}
	}
	return 0
}

// maxHwmonTempC returns the highest temperature reported by a device's hwmon.
func maxHwmonTempC(deviceDir string) float64 {
	best := 0.0
	for _, hw := range hwmonDeviceDirs(deviceDir) {
		inputs, _ := filepath.Glob(filepath.Join(hw, "temp*_input"))
		for _, in := range inputs {
			if v, ok := readUint(in); ok {
				// Guard against bogus sensor values.
				c := float64(v) / 1000.0
				if c > best && c < 200 {
					best = c
				}
			}
		}
	}
	return best
}
