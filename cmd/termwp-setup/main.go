package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"termwp/internal/config"
	termwpruntime "termwp/internal/runtime"
	"termwp/internal/system"
	"termwp/internal/ui"
)

func main() {
	skipDoctor := flag.Bool("skip-doctor", false, "Skip post-installation doctor health checks")
	flag.Parse()

	fmt.Println(ui.TitleStyle.Render("⚡ TermWP Native Installer & Upgrader"))
	fmt.Println(ui.SubtitleStyle.Render(fmt.Sprintf("Target OS: %s/%s | Isolated Home: %s\n", runtime.GOOS, runtime.GOARCH, config.HomeDir())))

	tracker := ui.NewStepTracker(false, false)

	// 1. Ensure Directory Structure
	tracker.Start("Preparing TermWP isolated directory layout")
	if err := config.EnsureDirectories(); err != nil {
		tracker.Fail("Failed to create directory structure: " + err.Error())
		os.Exit(1)
	}
	tracker.Success("Directory layout prepared")

	// 2. Initialize Configuration
	tracker.Start("Loading configuration")
	cfg, err := config.LoadConfig()
	if err != nil {
		tracker.Fail("Failed to load or generate config: " + err.Error())
		os.Exit(1)
	}
	tracker.Success(fmt.Sprintf("Configuration active (MariaDB port: %d, Default PHP: %s)", cfg.MariaDBPort, cfg.DefaultPHP))

	// 3. Install termwp CLI Binary
	binTarget := filepath.Join(config.BinDir(), system.BinaryName())
	tracker.Start(fmt.Sprintf("Installing CLI binary (%s)", system.BinaryName()))

	installedCLI := false
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)

	candidateSources := []string{
		filepath.Join(execDir, system.BinaryName()),
		filepath.Join(".", system.BinaryName()),
		filepath.Join("..", system.BinaryName()),
	}

	for _, src := range candidateSources {
		if stat, err := os.Stat(src); err == nil && !stat.IsDir() {
			if filepath.Clean(src) != filepath.Clean(binTarget) {
				if err := copyExecutable(src, binTarget); err == nil {
					installedCLI = true
					break
				}
			} else {
				installedCLI = true
				break
			}
		}
	}

	if !installedCLI {
		releaseURL := fmt.Sprintf("https://github.com/termwp-binaries/releases/releases/latest/download/termwp-%s-%s", runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			releaseURL += ".exe"
		}
		dl := termwpruntime.NewDownloader(tracker)
		if err := dl.DownloadFile(releaseURL, binTarget); err == nil {
			_ = os.Chmod(binTarget, 0755)
			installedCLI = true
		}
	}

	if installedCLI {
		tracker.Success("TermWP CLI binary installed in " + config.BinDir())
	} else {
		tracker.Warn("TermWP binary could not be copied automatically. Ensure " + system.BinaryName() + " is in " + config.BinDir())
	}

	// 4. Download and Install Runtime Stack
	fmt.Println(ui.TitleStyle.Render("\n📦 Installing Isolated Runtime Components"))
	dl := termwpruntime.NewDownloader(tracker)

	// Nginx
	if nginxSrc, err := config.GetNginxSource(runtime.GOOS, runtime.GOARCH); err == nil {
		if err := dl.FetchAndInstall(nginxSrc, config.NginxRuntimeDir()); err != nil {
			tracker.Fail("Failed installing Nginx: " + err.Error())
		} else {
			tracker.Success(fmt.Sprintf("Nginx %s installed", nginxSrc.Version))
		}
	}

	// MariaDB
	if mariaSrc, err := config.GetMariaDBSource(runtime.GOOS, runtime.GOARCH); err == nil {
		if err := dl.FetchAndInstall(mariaSrc, config.MariaDBRuntimeDir()); err != nil {
			tracker.Fail("Failed installing MariaDB: " + err.Error())
		} else {
			tracker.Success(fmt.Sprintf("MariaDB %s installed", mariaSrc.Version))
		}
	}

	// PHP 8.3, 8.4, 8.5
	for _, phpVer := range []string{"8.3", "8.4", "8.5"} {
		if phpSrc, err := config.GetPHPSource(phpVer, runtime.GOOS, runtime.GOARCH); err == nil {
			if err := dl.FetchAndInstall(phpSrc, config.PHPRuntimeDir(phpVer)); err != nil {
				tracker.Fail(fmt.Sprintf("Failed installing PHP %s: %s", phpVer, err.Error()))
			} else {
				tracker.Success(fmt.Sprintf("PHP %s installed", phpVer))
			}
		}
	}

	// WP-CLI
	wpcliSrc := config.GetWPCLISource()
	if err := dl.FetchAndInstall(wpcliSrc, config.ToolsDir()); err != nil {
		tracker.Fail("Failed installing WP-CLI: " + err.Error())
	} else {
		tracker.Success("WP-CLI latest installed")
	}

	// mkcert
	if mkcertSrc, err := config.GetMkcertSource(runtime.GOOS, runtime.GOARCH); err == nil {
		if err := dl.FetchAndInstall(mkcertSrc, config.ToolsDir()); err != nil {
			tracker.Fail("Failed installing mkcert: " + err.Error())
		} else {
			tracker.Success(fmt.Sprintf("mkcert %s installed", mkcertSrc.Version))
		}
	}

	// 5. Environment & System Capabilities
	fmt.Println(ui.TitleStyle.Render("\n⚙️  Configuring Environment & Permissions"))

	tracker.Start("Configuring user environment PATH and symlinks")
	if err := system.ConfigureEnvironment(config.BinDir(), config.RuntimeDir()); err != nil {
		tracker.Warn("User environment configuration: " + err.Error())
	} else {
		tracker.Success("Environment PATH and execution symlinks configured")
	}

	if err := system.ApplyLowPortCapabilities(config.RuntimeDir()); err != nil {
		tracker.Warn("Low-port binding capability: " + err.Error())
	}

	// 6. mkcert Local CA Trust
	tracker.Start("Installing local trusted SSL Root CA")
	mkcertMgr := termwpruntime.NewMkcertManager()
	if err := mkcertMgr.InstallCA(); err != nil {
		tracker.Warn("Local Root CA install notice: " + err.Error())
	} else {
		tracker.Success("Local Root CA installed in system trust store")
	}

	// 7. Post-install Doctor
	if !*skipDoctor {
		fmt.Println(ui.TitleStyle.Render("\n🩺 Verifying Installation Integrity"))
		if _, err := os.Stat(binTarget); err == nil {
			cmd := exec.Command(binTarget, "doctor", "--fix")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	}

	// 8. Completion Banner
	cardContent := fmt.Sprintf(`🎉 TermWP installation successfully completed!

Target Directory: %s
CLI Location:     %s

Next Steps:
  1. Open a new terminal session
  2. Run 'termwp show' to view environment status
  3. Run 'termwp add mysite.local' to launch your first WordPress site`, config.HomeDir(), binTarget)

	fmt.Println(ui.CardStyle.Render(cardContent))
}

func copyExecutable(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.Remove(dest)
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
