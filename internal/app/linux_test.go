//go:build linux

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNvidiaCUDACores(t *testing.T) {
	cases := map[string]int{
		"NVIDIA GeForce RTX 3090": 10496,
		"NVIDIA GeForce RTX 4090": 16384,
		"NVIDIA GeForce RTX 3080": 8704,
		"NVIDIA A100-SXM4-40GB":   6912,
		"Some Unknown GPU":        0,
	}
	for name, want := range cases {
		if got := nvidiaCUDACores(name); got != want {
			t.Errorf("nvidiaCUDACores(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestSMCLikeTempKeyGrouping(t *testing.T) {
	cases := []struct {
		chip, label, base string
		wantGroup         string
	}{
		{"k10temp", "Tctl", "temp1", "CPU Core"},
		{"nvme", "Composite", "temp1", "SSD"},
		{"iwlwifi_1", "wifi", "temp1", "Wireless"},
		{"r8169_0", "board", "temp1", "Board"},
		{"jc42", "ambient", "temp1", "Ambient"},
	}
	for _, c := range cases {
		key := smcLikeTempKey(c.chip, c.label, c.base)
		if got := sensorGroupName(key); got != c.wantGroup {
			t.Errorf("smcLikeTempKey(%q,%q,%q)=%q grouped as %q, want %q",
				c.chip, c.label, c.base, key, got, c.wantGroup)
		}
	}
}

func TestParseFloatPrefix(t *testing.T) {
	cases := map[string]float64{
		"866.7 MBit/s": 866.7,
		"54 MBit/s":    54,
		"n/a":          0,
	}
	for in, want := range cases {
		got, err := parseFloatPrefix(in)
		if in == "n/a" {
			if err == nil {
				t.Errorf("parseFloatPrefix(%q) expected error", in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseFloatPrefix(%q) unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseFloatPrefix(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestGetCPUUsage(t *testing.T) {
	usages, err := GetCPUUsage()
	if err != nil {
		t.Fatalf("GetCPUUsage: %v", err)
	}
	if len(usages) == 0 {
		t.Fatal("expected at least one CPU")
	}
}

func TestGetNativeMemoryMetrics(t *testing.T) {
	m, err := GetNativeMemoryMetrics()
	if err != nil {
		t.Fatalf("GetNativeMemoryMetrics: %v", err)
	}
	if m.Total == 0 {
		t.Fatal("expected non-zero total memory")
	}
}

func TestSocMetricsSample(t *testing.T) {
	if err := initSocMetrics(); err != nil {
		t.Fatalf("initSocMetrics: %v", err)
	}
	defer cleanupSocMetrics()
	m := sampleSocMetrics(50)
	if m.CPUTemp <= 0 && m.GPUTemp <= 0 {
		t.Fatal("expected at least one temperature reading")
	}
}

func TestGetVolumes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping filesystem-dependent test in short mode")
	}
	vols := getVolumes()
	if len(vols) == 0 {
		t.Skip("no real mounts detected in this environment")
	}
	for _, v := range vols {
		if v.Total <= 0 {
			t.Errorf("volume %q has non-positive total", v.Name)
		}
	}
}

func TestPlatformGPUName(t *testing.T) {
	// Should never panic; may be empty on machines without NVIDIA GPUs.
	_ = platformGPUName()
	if _, err := os.Stat(filepath.Join("/sys/class/hwmon")); err != nil {
		t.Skip("no hwmon on this system")
	}
}

func TestParseMHz(t *testing.T) {
	cases := map[string]int{
		"1: 1800Mhz *": 1800,
		"0: 500Mhz":    500,
		"3: 2615MHz":   2615,
		"not a clock":  0,
	}
	for in, want := range cases {
		if got := parseMHz(in); got != want {
			t.Errorf("parseMHz(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseQuotedFields(t *testing.T) {
	line := `03:00.0 "VGA compatible controller" "Advanced Micro Devices, Inc. [AMD/ATI]" "Navi 21 [Radeon RX 6800/6800 XT / 6900 XT]"`
	fields := parseQuotedFields(line)
	if len(fields) < 3 {
		t.Fatalf("expected >=3 fields, got %d (%v)", len(fields), fields)
	}
	if fields[2] == "" {
		t.Error("device name field was empty")
	}
}

func TestAMDFromIntelLookups(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) int
		want int
	}{
		{"Navi 21 [Radeon RX 6900 XT]", amdStreamProcessors, 5120},
		{"Radeon RX 7900 XTX", amdStreamProcessors, 6144},
		{"Arc A770 Graphics", intelExecutionUnits, 512},
		{"Unknown Device", amdStreamProcessors, 0},
	}
	for _, c := range cases {
		if got := c.fn(c.name); got != c.want {
			t.Errorf("%q lookup = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestApplyProfile(t *testing.T) {
	saved := currentConfig
	savedCol, savedRev := selectedColumn, sortReverse
	t.Cleanup(func() {
		currentConfig = saved
		selectedColumn, sortReverse = savedCol, savedRev
	})

	sc := 6
	rev := true
	currentConfig.Profiles = map[string]Profile{
		"gaming": {
			DefaultLayout: "multi_gpu",
			Theme:         "nord",
			Interval:      250,
			SortColumn:    &sc,
			SortReverse:   &rev,
		},
	}

	if !applyProfile("gaming") {
		t.Fatal("applyProfile returned false for an existing profile")
	}
	if currentConfig.DefaultLayout != "multi_gpu" {
		t.Errorf("layout = %q, want multi_gpu", currentConfig.DefaultLayout)
	}
	if currentConfig.Theme != "nord" {
		t.Errorf("theme = %q, want nord", currentConfig.Theme)
	}
	if updateInterval != 250 {
		t.Errorf("interval = %d, want 250", updateInterval)
	}
	if selectedColumn != 6 || !sortReverse {
		t.Errorf("sort not applied: col=%d reverse=%v", selectedColumn, sortReverse)
	}
	if applyProfile("does-not-exist") {
		t.Error("applyProfile returned true for a missing profile")
	}
	names := profileNames()
	if len(names) != 1 || names[0] != "gaming" {
		t.Errorf("profileNames = %v, want [gaming]", names)
	}
}

func TestThemeRegistration(t *testing.T) {
	for _, name := range []string{"nord", "gruvbox", "dracula", "tokyonight", "matrix"} {
		if _, ok := colorMap[name]; !ok {
			t.Errorf("theme %q missing from colorMap", name)
		}
		if _, ok := themeHexMap[name]; !ok {
			t.Errorf("theme %q missing from themeHexMap", name)
		}
	}
}
