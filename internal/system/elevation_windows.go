//go:build windows

package system

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

// IsElevated checks if current process has Administrator privileges on Windows.
func IsElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// RunElevated runs an executable elevated via UAC prompt ("runas") and waits for it to exit.
func RunElevated(executable string, args []string) error {
	// Construct argument string for PowerShell Start-Process with -Verb RunAs -Wait
	escapedArgs := make([]string, len(args))
	for i, a := range args {
		escapedArgs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(a, "'", "''"))
	}
	argList := strings.Join(escapedArgs, ", ")

	psScript := fmt.Sprintf(
		`$proc = Start-Process -FilePath '%s' -ArgumentList @(%s) -Verb RunAs -Wait -PassThru; exit $proc.ExitCode`,
		executable, argList,
	)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("elevation failed or was cancelled by user: %s (%w)", strings.TrimSpace(string(output)), err)
	}
	return nil
}
