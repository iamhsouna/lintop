//go:build linux

package app

// Display FPS tracking relies on macOS CGDisplayStream. On Linux there is no
// equivalent permission model, so these are inert and report zero.

type DisplayFPSMetrics struct {
	FPS             uint32
	FrameIntervalMs float64
}

func StartDisplayFPSCounter() bool { return false }

func StopDisplayFPSCounter() {}

func HasScreenRecordingAccess() bool { return false }

func GetDisplayFPSMetrics() DisplayFPSMetrics { return DisplayFPSMetrics{} }

func DumpDisplayFPSDiagnostics() {}
