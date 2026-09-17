//go:build linux

package app

import (
	"os"
	"syscall"
)

// redirectStderr points file descriptor 2 at the log file. Dup3 is used
// because Dup2 is not available on every Linux architecture (e.g. arm64).
func redirectStderr(logfile *os.File) {
	_ = syscall.Dup3(int(logfile.Fd()), 2, 0)
}
