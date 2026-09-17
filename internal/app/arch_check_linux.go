//go:build linux

package app

// On Linux there is no Apple Silicon requirement. lintop runs on any x86_64
// or ARM64 host; NVIDIA GPUs are detected at runtime via nvidia-smi.
func requireAppleSilicon() {}

func isAppleSilicon() bool { return false }
