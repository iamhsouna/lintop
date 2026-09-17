//go:build linux

package app

import (
	"os/exec"
	"strings"
)

// systemThemeIsLight reads the desktop color-scheme preference. Returns an
// error when no desktop preference can be determined, letting the caller fall
// back to terminal-based detection.
func systemThemeIsLight() (bool, error) {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return false, err
	}
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil {
		return false, err
	}
	scheme := strings.ToLower(strings.TrimSpace(string(out)))
	switch {
	case strings.Contains(scheme, "prefer-dark"):
		return false, nil
	case strings.Contains(scheme, "prefer-light"):
		return true, nil
	default:
		return false, nil
	}
}
