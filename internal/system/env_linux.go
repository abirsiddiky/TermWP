//go:build linux

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConfigureEnvironment symlinks termwp to ~/.local/bin/termwp and checks PATH.
func ConfigureEnvironment(binDir, runtimeDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	localBin := filepath.Join(home, ".local", "bin")
	_ = os.MkdirAll(localBin, 0755)

	targetBin := filepath.Join(binDir, "termwp")
	symlinkPath := filepath.Join(localBin, "termwp")

	_ = os.Remove(symlinkPath)
	if err := os.Symlink(targetBin, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink in %s: %w", localBin, err)
	}

	pathEnv := os.Getenv("PATH")
	inPath := false
	for _, p := range strings.Split(pathEnv, ":") {
		if filepath.Clean(p) == filepath.Clean(localBin) {
			inPath = true
			break
		}
	}

	if !inPath {
		rcFiles := []string{
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".zshrc"),
		}
		exportLine := "\n# TermWP CLI path\nexport PATH=\"$HOME/.local/bin:$PATH\"\n"
		for _, rc := range rcFiles {
			if _, err := os.Stat(rc); err == nil {
				data, _ := os.ReadFile(rc)
				if !strings.Contains(string(data), ".local/bin") {
					f, err := os.OpenFile(rc, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						_, _ = f.WriteString(exportLine)
						f.Close()
					}
				}
			}
		}
	}

	return nil
}

// RemoveEnvironment removes ~/.local/bin/termwp symlinks.
func RemoveEnvironment(binDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	symlinkPath := filepath.Join(home, ".local", "bin", "termwp")
	_ = os.Remove(symlinkPath)

	if _, err := os.Stat("/usr/local/bin/termwp"); err == nil {
		_ = os.Remove("/usr/local/bin/termwp")
	}

	return nil
}

// BinaryName returns platform-specific binary name.
func BinaryName() string {
	return "termwp"
}

// ApplyLowPortCapabilities attempts to apply cap_net_bind_service to nginx binary.
func ApplyLowPortCapabilities(runtimeDir string) error {
	nginxPath := filepath.Join(runtimeDir, "nginx", "sbin", "nginx")
	if _, err := os.Stat(nginxPath); err != nil {
		nginxPath = filepath.Join(runtimeDir, "nginx", "nginx")
	}

	if _, err := os.Stat(nginxPath); err == nil {
		if _, err := exec.LookPath("setcap"); err == nil {
			cmd := exec.Command("setcap", "cap_net_bind_service=+ep", nginxPath)
			if err := cmd.Run(); err != nil {
				_ = exec.Command("sudo", "-n", "setcap", "cap_net_bind_service=+ep", nginxPath).Run()
			}
		}
	}
	return nil
}
