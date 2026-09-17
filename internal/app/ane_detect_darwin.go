//go:build darwin

package app

// hasANE reports whether the Apple Neural Engine is available. On macOS this
// is always true because mactop already requires Apple Silicon.
func hasANE() bool { return isAppleSilicon() }
