package runtime

import (
	"fmt"

	"termwp/internal/config"
	"termwp/internal/ui"
)

// StackManager coordinates all background daemons and runtimes.
type StackManager struct {
	Config  *config.GlobalConfig
	Nginx   *NginxManager
	PHP     *PHPManager
	MariaDB *MariaDBManager
	WPCLI   *WPCLIManager
	Mkcert  *MkcertManager
}

var (
	defaultStack *StackManager
)

// GetStackManager returns the initialized StackManager singleton.
func GetStackManager() (*StackManager, error) {
	if defaultStack != nil {
		return defaultStack, nil
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global configuration: %w", err)
	}

	phpMgr := NewPHPManager(cfg)
	defaultStack = &StackManager{
		Config:  cfg,
		Nginx:   NewNginxManager(cfg),
		PHP:     phpMgr,
		MariaDB: NewMariaDBManager(cfg),
		WPCLI:   NewWPCLIManager(phpMgr),
		Mkcert:  NewMkcertManager(),
	}

	return defaultStack, nil
}

// CheckServices returns the live status of all background daemons.
func (sm *StackManager) CheckServices() ui.ServiceStatus {
	phpStatus := make(map[string]bool)
	for _, v := range []string{"8.3", "8.4", "8.5"} {
		phpStatus[v] = sm.PHP.IsPHPRunning(v)
	}

	return ui.ServiceStatus{
		NginxRunning:   sm.Nginx.IsRunning(),
		MariaDBRunning: sm.MariaDB.IsRunning(),
		PHPRunning:     phpStatus,
	}
}

// EnsureStackRunning ensures MariaDB, required PHP versions, and Nginx are running.
func (sm *StackManager) EnsureStackRunning(activePHPVersions []string) error {
	if len(activePHPVersions) == 0 {
		return nil
	}

	// 1. MariaDB
	if !sm.MariaDB.IsRunning() {
		if err := sm.MariaDB.Start(); err != nil {
			return fmt.Errorf("failed to start MariaDB: %w", err)
		}
	}

	// 2. Needed PHP backends
	for _, v := range activePHPVersions {
		if !sm.PHP.IsPHPRunning(v) {
			if err := sm.PHP.StartPHP(v); err != nil {
				return fmt.Errorf("failed to start PHP %s: %w", v, err)
			}
		}
	}

	// 3. Nginx
	if !sm.Nginx.IsRunning() {
		if err := sm.Nginx.Start(); err != nil {
			return fmt.Errorf("failed to start nginx: %w", err)
		}
	} else {
		_ = sm.Nginx.Reload()
	}

	return nil
}

// ShutdownStack performs complete, ordered graceful shutdown:
// 1. nginx (waits up to 5s, then force kill)
// 2. all running PHP backends (8.3, 8.4, 8.5)
// 3. MariaDB (mariadb-admin shutdown with stored root credentials, waits up to 15s)
// 4. Confirmation of each stopped service
func (sm *StackManager) ShutdownStack(force bool, quiet bool) error {
	// Record which services were running beforehand
	phpVersions := []string{"8.3", "8.4", "8.5"}
	phpWasRunning := make(map[string]bool)
	for _, v := range phpVersions {
		phpWasRunning[v] = sm.PHP.IsPHPRunning(v)
	}
	nginxWasRunning := sm.Nginx.IsRunning()
	mariadbWasRunning := sm.MariaDB.IsRunning()

	// 1. Stop nginx
	if nginxWasRunning {
		_, err := sm.Nginx.Stop(force)
		if err != nil && !quiet {
			fmt.Println(ui.FormatWarning(fmt.Sprintf("nginx stop warning: %v", err)))
		}
		if !quiet {
			fmt.Println(ui.FormatSuccess("nginx stopped"))
		}
	} else if !quiet {
		fmt.Println(ui.FormatInfo("nginx already stopped"))
	}

	// 2. Stop running PHP backends
	for _, v := range phpVersions {
		if phpWasRunning[v] {
			_, err := sm.PHP.StopPHP(v, force)
			if err != nil && !quiet {
				fmt.Println(ui.FormatWarning(fmt.Sprintf("PHP %s stop warning: %v", v, err)))
			}
			if !quiet {
				fmt.Println(ui.FormatSuccess(fmt.Sprintf("PHP %s stopped", v)))
			}
		}
	}

	// 3. Stop MariaDB
	if mariadbWasRunning {
		_, err := sm.MariaDB.Stop(force)
		if err != nil && !quiet {
			fmt.Println(ui.FormatWarning(fmt.Sprintf("MariaDB stop warning: %v", err)))
		}
		if !quiet {
			fmt.Println(ui.FormatSuccess("MariaDB stopped"))
		}
	} else if !quiet {
		fmt.Println(ui.FormatInfo("MariaDB already stopped"))
	}

	if !quiet {
		fmt.Println(ui.BoldStyle.Render("\nTermWP is fully shut down."))
	}

	return nil
}
