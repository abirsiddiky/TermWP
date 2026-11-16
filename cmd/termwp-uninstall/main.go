package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"termwp/internal/config"
	termwpruntime "termwp/internal/runtime"
	"termwp/internal/system"
	"termwp/internal/ui"
)

func main() {
	keepData := flag.Bool("keep-data", false, "Preserve WordPress site files and database data in home backup folder")
	force := flag.Bool("force", false, "Skip confirmation prompt")
	flag.BoolVar(force, "f", false, "Skip confirmation prompt (shorthand)")
	flag.Parse()

	fmt.Println(ui.TitleStyle.Render("🗑️  TermWP Native Uninstaller"))
	fmt.Println(ui.SubtitleStyle.Render(fmt.Sprintf("Target Directory: %s\n", config.HomeDir())))

	if _, err := os.Stat(config.HomeDir()); os.IsNotExist(err) {
		fmt.Println(ui.WarningStyle.Render("TermWP installation directory not found. Nothing to uninstall."))
		return
	}

	if !*force {
		fmt.Print(ui.WarningStyle.Render("Are you sure you want to completely uninstall TermWP? (y/N): "))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println(ui.MutedStyle.Render("Uninstallation aborted."))
			return
		}
		fmt.Println()
	}

	tracker := ui.NewStepTracker(false, false)

	// 1. Terminate running background services
	tracker.Start("Stopping all running background services")
	binPath := filepath.Join(config.BinDir(), system.BinaryName())
	if _, err := os.Stat(binPath); err == nil {
		cmd := exec.Command(binPath, "shutdown", "--force", "--quiet")
		_ = cmd.Run()
	}
	tracker.Success("Background services stopped")

	// 2. Uninstall mkcert Root CA
	tracker.Start("Uninstalling local trusted SSL Root CA")
	mkcertMgr := termwpruntime.NewMkcertManager()
	if err := mkcertMgr.UninstallCA(); err != nil {
		tracker.Warn("mkcert root CA removal notice: " + err.Error())
	} else {
		tracker.Success("Local Root CA uninstalled from system trust store")
	}

	// 3. Clean hosts file entries
	tracker.Start("Cleaning hosts file entries")
	if err := system.ClearAllHostsEntries(); err != nil {
		tracker.Warn("Hosts file cleanup notice: " + err.Error())
	} else {
		tracker.Success("Cleaned all TermWP entries from hosts file")
	}

	// 4. Remove from User PATH / remove symlinks
	tracker.Start("Removing TermWP from user PATH and symlinks")
	if err := system.RemoveEnvironment(config.BinDir()); err != nil {
		tracker.Warn("Environment cleanup notice: " + err.Error())
	} else {
		tracker.Success("Removed TermWP from user PATH")
	}

	// 5. Backup data if requested
	if *keepData {
		home, err := os.UserHomeDir()
		if err == nil {
			timestamp := time.Now().Format("20060102_150405")
			backupDir := filepath.Join(home, fmt.Sprintf("TermWP_Backup_%s", timestamp))
			tracker.Start("Preserving backup of sites and database data in " + backupDir)
			_ = os.MkdirAll(backupDir, 0755)

			_ = copyDir(config.SitesDir(), filepath.Join(backupDir, "sites"))
			_ = copyDir(config.DataDir(), filepath.Join(backupDir, "data"))
			tracker.Success("Backup preserved at " + backupDir)
		}
	}

	// 6. Delete TermWP home directory
	tracker.Start("Deleting TermWP home directory")
	time.Sleep(1 * time.Second) // allow process handles to fully close
	if err := os.RemoveAll(config.HomeDir()); err != nil {
		tracker.Warn("Some files could not be removed immediately: " + err.Error())
	} else {
		tracker.Success("TermWP directory removed")
	}

	fmt.Println(ui.CardStyle.Render("✔ TermWP has been completely uninstalled."))
}

func copyDir(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
