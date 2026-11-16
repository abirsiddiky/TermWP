package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"termwp/internal/config"
	"termwp/internal/ui"
)

// MkcertManager manages local trusted SSL certificates and the root CA.
type MkcertManager struct{}

// NewMkcertManager creates an initialized MkcertManager.
func NewMkcertManager() *MkcertManager {
	return &MkcertManager{}
}

// EnsureInstalled downloads mkcert if missing and makes it executable.
func (m *MkcertManager) EnsureInstalled(tracker *ui.StepTracker) error {
	bin := config.MkcertPath()
	if _, err := os.Stat(bin); err == nil {
		return nil
	}

	src, err := config.GetMkcertSource(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	downloader := NewDownloader(tracker)
	return downloader.FetchAndInstall(src, config.ToolsDir())
}

// InstallCA installs the local CA into the system trust store.
func (m *MkcertManager) InstallCA() error {
	if err := m.EnsureInstalled(nil); err != nil {
		return err
	}

	cmd := exec.Command(config.MkcertPath(), "-install")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkcert -install failed: %s (%w)", string(output), err)
	}
	return nil
}

// UninstallCA uninstalls the local CA from the system trust store.
func (m *MkcertManager) UninstallCA() error {
	if _, err := os.Stat(config.MkcertPath()); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command(config.MkcertPath(), "-uninstall")
	return cmd.Run()
}

// GenerateSiteCert generates a trusted TLS certificate for domain and wildcard *.domain.
func (m *MkcertManager) GenerateSiteCert(domain string) (certPath, keyPath string, err error) {
	if err := m.InstallCA(); err != nil {
		return "", "", err
	}

	certPath = filepath.Join(config.CertsDir(), fmt.Sprintf("%s.pem", domain))
	keyPath = filepath.Join(config.CertsDir(), fmt.Sprintf("%s-key.pem", domain))

	// Wildcard SAN for subdomains
	wildcard := fmt.Sprintf("*.%s", domain)

	cmd := exec.Command(config.MkcertPath(),
		"-cert-file", certPath,
		"-key-file", keyPath,
		domain, wildcard, "localhost", "127.0.0.1",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("certificate generation failed for %s: %s (%w)", domain, string(output), err)
	}

	return certPath, keyPath, nil
}

// RemoveSiteCert deletes certificates generated for a domain.
func (m *MkcertManager) RemoveSiteCert(domain string) error {
	certPath := filepath.Join(config.CertsDir(), fmt.Sprintf("%s.pem", domain))
	keyPath := filepath.Join(config.CertsDir(), fmt.Sprintf("%s-key.pem", domain))
	_ = os.Remove(certPath)
	_ = os.Remove(keyPath)
	return nil
}

// CheckCAInstalled checks if the mkcert root CA exists.
func (m *MkcertManager) CheckCAInstalled() bool {
	if _, err := os.Stat(config.MkcertPath()); os.IsNotExist(err) {
		return false
	}
	cmd := exec.Command(config.MkcertPath(), "-CAROOT")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	caRoot := filepath.Clean(string(out))
	rootCert := filepath.Join(caRoot, "rootCA.pem")
	_, err = os.Stat(rootCert)
	return err == nil
}
