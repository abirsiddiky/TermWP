package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo holds information about a monitored daemon.
type ProcessInfo struct {
	Name    string
	PID     int
	Running bool
}

// ReadPIDFile reads a PID from the specified file path. Returns 0 if missing or invalid.
func ReadPIDFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	trimmed := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(trimmed)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// WritePIDFile writes the process PID to the target path.
func WritePIDFile(path string, pid int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// RemovePIDFile removes the specified PID file.
func RemovePIDFile(path string) {
	_ = os.Remove(path)
}

// WaitForProcessExit polls until the process with the given PID has terminated or timeout expires.
func WaitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return !IsProcessRunning(pid)
}

// StopDaemon gracefully stops a daemon:
// 1. If gracefully stopped via custom fn, waits up to graceTimeout.
// 2. If PID is still active, forcibly kills the process tree.
func StopDaemon(pidFile string, graceStopFn func() error, graceTimeout time.Duration) (bool, error) {
	pid := ReadPIDFile(pidFile)
	if pid <= 0 || !IsProcessRunning(pid) {
		RemovePIDFile(pidFile)
		return false, nil // Already stopped
	}

	stopped := false
	if graceStopFn != nil {
		_ = graceStopFn()
		if WaitForProcessExit(pid, graceTimeout) {
			stopped = true
		}
	}

	if !stopped && IsProcessRunning(pid) {
		// Force terminate process tree
		if err := KillProcessTree(pid); err != nil {
			return false, fmt.Errorf("failed to terminate process %d: %w", pid, err)
		}
		_ = WaitForProcessExit(pid, 2*time.Second)
	}

	RemovePIDFile(pidFile)
	return true, nil
}
