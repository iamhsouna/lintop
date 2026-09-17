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
