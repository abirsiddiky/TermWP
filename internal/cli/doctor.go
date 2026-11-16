package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"termwp/internal/config"
	"termwp/internal/core"
	termwpruntime "termwp/internal/runtime"
	"termwp/internal/system"
	"termwp/internal/ui"
)

var (
	doctorFixFlag bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run comprehensive system diagnostics and environment health checks",
	Long: `Inspects system environment, runtime binaries, ports, hosts file,
database connectivity, certificates, and site integrity.

Checks performed:
  - System architecture, OS permissions, disk space
  - Runtime binaries (nginx, PHP 8.3/8.4/8.5, MariaDB, WP-CLI, mkcert)
  - Required PHP extensions for WordPress
  - Port availability/conflicts (80, 443, 3306, 9083-9085)
  - Stack daemon states (respects intentional shutdown status)
  - Hosts file synchronisation and mDNS collision detection (.local on Linux)
  - Per-site virtual hosts, database health, SSL certificate validity
  - Internet connectivity to TermWP download sources

Use --fix to automatically repair safe discrepancies:
  termwp doctor
  termwp doctor --fix`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeDoctor(doctorFixFlag)
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFixFlag, "fix", false, "Automatically repair safely resolvable issues")
}

func executeDoctor(fix bool) error {
	fmt.Println(ui.TitleStyle.Render("🩺 TermWP System Doctor"))
	fmt.Println(ui.SubtitleStyle.Render("Running health inspection across system, runtimes, and sites...\n"))

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed loading configuration: %w", err)
	}

	store := core.GetStateStore()
	isShutdown, _ := store.IsShutdown()
	activePHP, _ := store.GetActivePHPVersions()

	if fix {
		fmt.Println(ui.FormatInfo("Auto-repair enabled: reconciling environment structure..."))
		_ = config.EnsureDirectories()
		phpMgr := termwpruntime.NewPHPManager(cfg)
		for _, v := range []string{"8.3", "8.4", "8.5"} {
			cli := filepath.Join(config.PHPRuntimeDir(v), "php.exe")
			if runtime.GOOS != "windows" {
				cli = filepath.Join(config.PHPRuntimeDir(v), "bin", "php")
			}
			if _, err := os.Stat(cli); err == nil {
				_ = phpMgr.EnsurePHPIni(v)
			}
		}
	}

	engine := system.NewDoctorEngine(cfg)
	results := engine.RunDiagnostics(isShutdown, activePHP)

	fmt.Print(system.RenderDoctorChecklist(results))
	return nil
}
