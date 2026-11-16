//go:build windows

package system

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var (
	moduser32               = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeoutW = moduser32.NewProc("SendMessageTimeoutW")
)

const (
	HWND_BROADCAST   = 0xffff
	WM_SETTINGCHANGE = 0x001A
	SMTO_ABORTIFHUNG = 0x0002
)

// ConfigureEnvironment adds binDir to User PATH and broadcasts environment change.
func ConfigureEnvironment(binDir, runtimeDir string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open Environment registry key: %w", err)
	}
	defer k.Close()

	existingPath, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to read User Path from registry: %w", err)
	}

	paths := strings.Split(existingPath, ";")
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), binDir) {
			return nil // already configured
		}
	}

	newPath := binDir
	if existingPath != "" {
		newPath = binDir + ";" + existingPath
	}

	if err := k.SetStringValue("Path", newPath); err != nil {
		return fmt.Errorf("failed to write updated User Path to registry: %w", err)
	}

	_ = os.Setenv("PATH", binDir+";"+os.Getenv("PATH"))

	broadcastSettingChange()
	return nil
}

// RemoveEnvironment removes binDir from User PATH and broadcasts environment change.
func RemoveEnvironment(binDir string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	existingPath, _, err := k.GetStringValue("Path")
	if err != nil {
		return nil
	}

	parts := strings.Split(existingPath, ";")
	var newParts []string
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" && !strings.EqualFold(clean, binDir) {
			newParts = append(newParts, clean)
		}
	}

	newPath := strings.Join(newParts, ";")
	_ = k.SetStringValue("Path", newPath)

	broadcastSettingChange()
	return nil
}

// BinaryName returns platform-specific binary name.
func BinaryName() string {
	return "termwp.exe"
}

// ApplyLowPortCapabilities is a no-op on Windows.
func ApplyLowPortCapabilities(runtimeDir string) error {
	return nil
}

func broadcastSettingChange() {
	envStr, _ := syscall.UTF16PtrFromString("Environment")
	var result uintptr
	_, _, _ = procSendMessageTimeoutW.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envStr)),
		uintptr(SMTO_ABORTIFHUNG),
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}
