//go:build darwin

package app

import (
	"os/exec"
	"strings"
)

// systemThemeIsLight reports whether the macOS appearance is Light.
func systemThemeIsLight() (bool, error) {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// Key absent means Light mode on macOS.
		return true, nil
	}
	return !strings.EqualFold(strings.TrimSpace(string(out)), "Dark"), nil
}
