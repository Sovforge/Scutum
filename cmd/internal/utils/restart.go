package utils

import (
	"fmt"
	"os"
	"syscall"
)

// SelfRestart replaces the running process with a fresh copy of itself via
// execve. All goroutines and connections are terminated immediately — callers
// must flush any pending responses before calling this. It never returns on
// success; on failure it returns the underlying error.
func SelfRestart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}
