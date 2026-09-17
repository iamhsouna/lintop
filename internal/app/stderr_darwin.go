//go:build darwin

package app

import (
	"os"
	"syscall"
)

// redirectStderr points file descriptor 2 at the log file.
func redirectStderr(logfile *os.File) {
	_ = syscall.Dup2(int(logfile.Fd()), 2)
}
