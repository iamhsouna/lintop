//go:build linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"lintop/internal/i18n"
)

// BatteryInfo describes the current battery state.
type BatteryInfo struct {
	Present   bool   `json:"present" yaml:"present" xml:"Present" toon:"present"`
	Percent   *int   `json:"percent,omitempty" yaml:"percent,omitempty" xml:"Percent,omitempty" toon:"percent"`
	Charging  bool   `json:"charging" yaml:"charging" xml:"Charging" toon:"charging"`
	OnACPower bool   `json:"on_ac_power" yaml:"on_ac_power" xml:"OnACPower" toon:"on_ac_power"`
	State     string `json:"state" yaml:"state" xml:"State" toon:"state"`
}

var (
	hasBatteryOnce    sync.Once
	hasBatteryPresent bool
)

func findBatteryDir() string {
	matches, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(matches) > 0 {
		return matches[0]
	}
	// Some systems use a default battery name.
	if _, err := os.Stat("/sys/class/power_supply/battery"); err == nil {
		return "/sys/class/power_supply/battery"
	}
	return ""
}

// GetBatteryInfo reads the battery state from /sys/class/power_supply.
func GetBatteryInfo() BatteryInfo {
	dir := findBatteryDir()
	if dir == "" {
		return BatteryInfo{}
	}
	info := BatteryInfo{Present: true}

	if capStr := readTrim(filepath.Join(dir, "capacity")); capStr != "" {
		if p, err := strconv.Atoi(capStr); err == nil && p >= 0 && p <= 100 {
			info.Percent = &p
		}
	}

	status := readTrim(filepath.Join(dir, "status"))
	info.State = status
	switch status {
	case "Charging":
		info.Charging = true
	case "Full":
		info.OnACPower = true
	case "Not charging":
		info.OnACPower = true
	}

	return info
}

// HasBattery reports whether the host has a battery.
func HasBattery() bool {
	hasBatteryOnce.Do(func() {
		hasBatteryPresent = GetBatteryInfo().Present
	})
	return hasBatteryPresent
}

func (b BatteryInfo) Displayable() bool {
	return b.Present && b.Percent != nil
}

func batteryStateLabel(bat BatteryInfo) string {
	switch {
	case bat.Charging:
		return i18n.T("Info_BatteryCharging")
	case bat.OnACPower:
		return i18n.T("Info_BatteryAC")
	default:
		return i18n.T("Info_BatteryDischarging")
	}
}

func formatBatteryLine() string {
	bat := GetBatteryInfo()
	if !bat.Displayable() {
		return ""
	}
	return fmt.Sprintf("%s: %d%% (%s)", i18n.T("Info_Battery"), *bat.Percent, batteryStateLabel(bat))
}

var _ = strings.TrimSpace
