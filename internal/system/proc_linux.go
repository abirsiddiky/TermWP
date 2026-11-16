//go:build !windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// IsProcessRunning checks whether a process with the given PID is currently running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, sending signal 0 checks for process existence without affecting it
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// SpawnBackgroundProcess launches a detached process in its own process group.
func SpawnBackgroundProcess(executable string, args []string, dir string, stdoutPath, stderrPath string) (*os.Process, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = dir

	// Detach into new process group so child processes can be managed together
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if stdoutPath != "" {
		_ = os.MkdirAll(filepath.Dir(stdoutPath), 0755)
		outF, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open stdout log: %w", err)
		}
		cmd.Stdout = outF
	}

	if stderrPath != "" {
		_ = os.MkdirAll(filepath.Dir(stderrPath), 0755)
		errF, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open stderr log: %w", err)
		}
		cmd.Stderr = errF
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to spawn process %s: %w", executable, err)
	}

	return cmd.Process, nil
}

// KillProcessTree terminates the target process and its process group.
func KillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}

	// Try graceful SIGTERM to process group
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	_ = syscall.Kill(pid, syscall.SIGTERM)

	time.Sleep(300 * time.Millisecond)

	if IsProcessRunning(pid) {
		// Force kill with SIGKILL
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}

	return nil
}
