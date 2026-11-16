//go:build !windows

package system

import (
	"fmt"
	"os"
	"os/exec"
)

// IsElevated checks if current process is running as root on Linux/Unix.
func IsElevated() bool {
	return os.Geteuid() == 0
}

// RunElevated runs an executable elevated via sudo.
func RunElevated(executable string, args []string) error {
	cmdArgs := append([]string{executable}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w", err)
	}
	return nil
}
