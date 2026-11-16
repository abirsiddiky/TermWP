//go:build windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// IsProcessRunning checks whether a process with the given PID is currently running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	var exitCode uint32
	err = windows.GetExitCodeProcess(h, &exitCode)
	if err != nil {
		return false
	}
	return exitCode == 259 // STILL_ACTIVE
}

// SpawnBackgroundProcess launches a detached process without a console window.
func SpawnBackgroundProcess(executable string, args []string, dir string, stdoutPath, stderrPath string) (*os.Process, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = dir

	// CREATE_NO_WINDOW = 0x08000000, CREATE_NEW_PROCESS_GROUP = 0x00000200
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
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

// KillProcessTree terminates the target process and all child processes.
func KillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	// Use taskkill /F /T /PID to terminate the whole tree reliably
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	_ = cmd.Run()

	// Double check
	if IsProcessRunning(pid) {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
		if err == nil {
			_ = windows.TerminateProcess(h, 1)
			_ = windows.CloseHandle(h)
		}
	}
	return nil
}
