package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"termwp/internal/config"
	"termwp/internal/system"
	"termwp/internal/ui"
)

// MariaDBManager manages the portable MariaDB daemon and database operations.
type MariaDBManager struct {
	cfg *config.GlobalConfig
}

// NewMariaDBManager creates an initialized MariaDBManager.
func NewMariaDBManager(cfg *config.GlobalConfig) *MariaDBManager {
	return &MariaDBManager{cfg: cfg}
}

// GetBinaryPath returns the mariadbd daemon executable path.
func (m *MariaDBManager) GetBinaryPath() string {
	base := config.MariaDBRuntimeDir()
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(base, "bin", "mariadbd.exe"),
			filepath.Join(base, "bin", "mysqld.exe"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		return candidates[0]
	}
	candidates := []string{
		filepath.Join(base, "bin", "mariadbd"),
		filepath.Join(base, "bin", "mysqld"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// GetClientPath returns the mariadb CLI client executable.
func (m *MariaDBManager) GetClientPath() string {
	base := config.MariaDBRuntimeDir()
	binName := "mariadb"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	primary := filepath.Join(base, "bin", binName)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	fallback := "mysql"
	if runtime.GOOS == "windows" {
		fallback += ".exe"
	}
	return filepath.Join(base, "bin", fallback)
}

// GetAdminPath returns the mariadb-admin executable.
func (m *MariaDBManager) GetAdminPath() string {
	base := config.MariaDBRuntimeDir()
	binName := "mariadb-admin"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	primary := filepath.Join(base, "bin", binName)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	fallback := "mysqladmin"
	if runtime.GOOS == "windows" {
		fallback += ".exe"
	}
	return filepath.Join(base, "bin", fallback)
}

// GetDumpPath returns the mariadb-dump executable.
func (m *MariaDBManager) GetDumpPath() string {
	base := config.MariaDBRuntimeDir()
	binName := "mariadb-dump"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	primary := filepath.Join(base, "bin", binName)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	fallback := "mysqldump"
	if runtime.GOOS == "windows" {
		fallback += ".exe"
	}
	return filepath.Join(base, "bin", fallback)
}

// PIDFilePath returns the path to mariadb.pid.
func (m *MariaDBManager) PIDFilePath() string {
	return filepath.Join(config.LogsDir(), "mariadb.pid")
}

// EnsureInstalled installs MariaDB binaries and initializes datadir if required.
func (m *MariaDBManager) EnsureInstalled(tracker *ui.StepTracker) error {
	bin := m.GetBinaryPath()
	if _, err := os.Stat(bin); err == nil {
		return m.EnsureDataDirInitialized(tracker)
	}

	src, err := config.GetMariaDBSource(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	downloader := NewDownloader(tracker)
	if err := downloader.FetchAndInstall(src, config.MariaDBRuntimeDir()); err != nil {
		return fmt.Errorf("failed installing MariaDB: %w", err)
	}

	return m.EnsureDataDirInitialized(tracker)
}

// ConfigFilePath returns the path to MariaDB's isolated configuration file.
func (m *MariaDBManager) ConfigFilePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(config.MariaDBDataDir(), "my.ini")
	}
	return filepath.Join(config.MariaDBDataDir(), "my.cnf")
}

// EnsureConfigFile writes or updates the isolated my.ini / my.cnf configuration file.
func (m *MariaDBManager) EnsureConfigFile() error {
	iniPath := m.ConfigFilePath()
	dataDir := filepath.ToSlash(config.MariaDBDataDir())
	baseDir := filepath.ToSlash(config.MariaDBRuntimeDir())
	pidFile := filepath.ToSlash(m.PIDFilePath())

	content := fmt.Sprintf(`[mysqld]
datadir=%s
basedir=%s
port=%d
bind-address=127.0.0.1
pid-file=%s
skip-networking=0

[client]
port=%d
`, dataDir, baseDir, m.cfg.MariaDBPort, pidFile, m.cfg.MariaDBPort)

	return os.WriteFile(iniPath, []byte(content), 0644)
}

// EnsureDataDirInitialized initializes the mysql system tables if empty.
func (m *MariaDBManager) EnsureDataDirInitialized(tracker *ui.StepTracker) error {
	dataDir := config.MariaDBDataDir()
	mysqlTableDir := filepath.Join(dataDir, "mysql")

	if _, err := os.Stat(mysqlTableDir); err == nil {
		return m.EnsureConfigFile() // Already initialized, keep config synced
	}

	if tracker != nil {
		tracker.Start("Initializing MariaDB data directory")
	}

	baseDir := config.MariaDBRuntimeDir()
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		installDbBin := filepath.Join(baseDir, "bin", "mariadb-install-db.exe")
		if _, err := os.Stat(installDbBin); os.IsNotExist(err) {
			installDbBin = filepath.Join(baseDir, "bin", "mysql_install_db.exe")
		}

		if _, err := os.Stat(installDbBin); err == nil {
			cmd = exec.Command(installDbBin,
				fmt.Sprintf("--datadir=%s", dataDir),
				fmt.Sprintf("--password=%s", m.cfg.MariaDBRootPassword),
				fmt.Sprintf("--port=%d", m.cfg.MariaDBPort),
				"--default-user",
				"--allow-remote-root-access",
			)
		} else {
			daemonBin := m.GetBinaryPath()
			cmd = exec.Command(daemonBin,
				fmt.Sprintf("--datadir=%s", dataDir),
				"--initialize-insecure",
			)
		}
	} else {
		// Linux
		installDbBin := filepath.Join(baseDir, "bin", "mariadb-install-db")
		if _, err := os.Stat(installDbBin); os.IsNotExist(err) {
			installDbBin = filepath.Join(baseDir, "scripts", "mariadb-install-db")
			if _, err := os.Stat(installDbBin); os.IsNotExist(err) {
				installDbBin = filepath.Join(baseDir, "bin", "mysql_install_db")
			}
		}

		if _, err := os.Stat(installDbBin); err == nil {
			cmd = exec.Command(installDbBin,
				fmt.Sprintf("--basedir=%s", baseDir),
				fmt.Sprintf("--datadir=%s", dataDir),
				"--auth-root-authentication-method=normal",
			)
		} else {
			daemonBin := m.GetBinaryPath()
			cmd = exec.Command(daemonBin,
				fmt.Sprintf("--basedir=%s", baseDir),
				fmt.Sprintf("--datadir=%s", dataDir),
				"--initialize-insecure",
			)
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if tracker != nil {
			tracker.Fail("MariaDB initialization failed")
		}
		return fmt.Errorf("failed to initialize MariaDB database:\n%s", string(output))
	}

	// Write isolated config file (my.ini on Windows, my.cnf on Linux)
	if err := m.EnsureConfigFile(); err != nil {
		return err
	}

	if tracker != nil {
		tracker.Success("MariaDB data directory initialized")
	}

	return nil
}

// IsRunning checks whether the MariaDB daemon is currently running.
func (m *MariaDBManager) IsRunning() bool {
	pid := system.ReadPIDFile(m.PIDFilePath())
	if pid <= 0 {
		return false
	}
	return system.IsProcessRunning(pid)
}

// Start launches the MariaDB daemon process.
func (m *MariaDBManager) Start() error {
	if m.IsRunning() {
		return nil
	}

	if err := m.EnsureInstalled(nil); err != nil {
		return err
	}

	if err := m.EnsureConfigFile(); err != nil {
		return err
	}

	bin := m.GetBinaryPath()
	pidFile := m.PIDFilePath()
	stdoutLog := filepath.Join(config.LogsDir(), "mariadb_stdout.log")
	stderrLog := filepath.Join(config.LogsDir(), "mariadb_error.log")

	args := []string{
		fmt.Sprintf("--defaults-file=%s", m.ConfigFilePath()),
		"--console",
	}

	proc, err := system.SpawnBackgroundProcess(bin, args, config.MariaDBRuntimeDir(), stdoutLog, stderrLog)
	if err != nil {
		return fmt.Errorf("failed starting MariaDB daemon: %w", err)
	}

	_ = system.WritePIDFile(pidFile, proc.Pid)

	// Wait up to 15s for MariaDB port to become active
	if !system.WaitForPort(m.cfg.MariaDBPort, 15*time.Second) {
		if !system.IsProcessRunning(proc.Pid) {
			logContent, _ := os.ReadFile(stderrLog)
			if len(logContent) == 0 {
				logContent, _ = os.ReadFile(stdoutLog)
			}
			return fmt.Errorf("MariaDB failed to start (PID %d exited):\n%s", proc.Pid, string(logContent))
		}
	}

	// Ensure root user password is set
	_ = m.SetRootPassword(m.cfg.MariaDBRootPassword)

	return nil
}

// TestConnection verifies that root can execute a trivial query with the configured password.
func (m *MariaDBManager) TestConnection() error {
	return m.ExecuteSQL("", m.cfg.MariaDBRootPassword, "SELECT 1;")
}

// SetRootPassword updates the root user password.
func (m *MariaDBManager) SetRootPassword(newPassword string) error {
	if err := m.TestConnection(); err == nil {
		return nil // Password already valid and accepted
	}
	sql := fmt.Sprintf("ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; ALTER USER 'root'@'127.0.0.1' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;", newPassword, newPassword)
	// Try without password first (if fresh install)
	_ = m.ExecuteSQL("mysql", "", sql)
	// Try with existing password
	return m.ExecuteSQL("mysql", m.cfg.MariaDBRootPassword, sql)
}

// ExecuteSQL executes raw SQL via the MariaDB CLI client.
func (m *MariaDBManager) ExecuteSQL(database, rootPass, sqlQuery string) error {
	client := m.GetClientPath()
	args := []string{
		"-h", "127.0.0.1",
		"-P", fmt.Sprintf("%d", m.cfg.MariaDBPort),
		"-u", "root",
	}

	if rootPass != "" {
		args = append(args, fmt.Sprintf("-p%s", rootPass))
	}
	if database != "" {
		args = append(args, database)
	}
	args = append(args, "-e", sqlQuery)

	cmd := exec.Command(client, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SQL execution error: %s (%w)", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CreateDatabaseAndUser provisions a new database and dedicated site user.
func (m *MariaDBManager) CreateDatabaseAndUser(dbName, user, password string) error {
	if !m.IsRunning() {
		if err := m.Start(); err != nil {
			return err
		}
	}

	sql := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; "+
			"CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s'; "+
			"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; "+
			"GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'127.0.0.1'; "+
			"GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; "+
			"FLUSH PRIVILEGES;",
		dbName, user, password, user, password, dbName, user, dbName, user,
	)

	return m.ExecuteSQL("", m.cfg.MariaDBRootPassword, sql)
}

// DropDatabaseAndUser drops a site's database and user.
func (m *MariaDBManager) DropDatabaseAndUser(dbName, user string) error {
	if !m.IsRunning() {
		_ = m.Start()
	}

	sql := fmt.Sprintf(
		"DROP DATABASE IF EXISTS `%s`; "+
			"DROP USER IF EXISTS '%s'@'127.0.0.1'; "+
			"DROP USER IF EXISTS '%s'@'localhost'; "+
			"FLUSH PRIVILEGES;",
		dbName, user, user,
	)

	return m.ExecuteSQL("", m.cfg.MariaDBRootPassword, sql)
}

// ExportDatabase dumps a database to a .sql file.
func (m *MariaDBManager) ExportDatabase(dbName, targetSqlFile string) error {
	dumpBin := m.GetDumpPath()
	_ = os.MkdirAll(filepath.Dir(targetSqlFile), 0755)

	outF, err := os.Create(targetSqlFile)
	if err != nil {
		return err
	}
	defer outF.Close()

	args := []string{
		"-h", "127.0.0.1",
		"-P", fmt.Sprintf("%d", m.cfg.MariaDBPort),
		"-u", "root",
		fmt.Sprintf("-p%s", m.cfg.MariaDBRootPassword),
		dbName,
	}

	cmd := exec.Command(dumpBin, args...)
	cmd.Stdout = outF

	var errBuf strings.Builder
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mariadb-dump failed: %s (%w)", errBuf.String(), err)
	}

	return nil
}

// ImportDatabase imports a .sql file into a database.
func (m *MariaDBManager) ImportDatabase(dbName, sourceSqlFile string) error {
	inF, err := os.Open(sourceSqlFile)
	if err != nil {
		return err
	}
	defer inF.Close()

	client := m.GetClientPath()
	args := []string{
		"-h", "127.0.0.1",
		"-P", fmt.Sprintf("%d", m.cfg.MariaDBPort),
		"-u", "root",
		fmt.Sprintf("-p%s", m.cfg.MariaDBRootPassword),
		dbName,
	}

	cmd := exec.Command(client, args...)
	cmd.Stdin = inF

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("database import failed: %s (%w)", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// Stop stops the MariaDB daemon gracefully using `mariadb-admin shutdown`.
// Waits up to 15s before force killing as a last resort.
func (m *MariaDBManager) Stop(force bool) (bool, error) {
	pidFile := m.PIDFilePath()
	pid := system.ReadPIDFile(pidFile)
	if pid <= 0 || !system.IsProcessRunning(pid) {
		system.RemovePIDFile(pidFile)
		return false, nil // Already stopped
	}

	if force {
		_ = system.KillProcessTree(pid)
		system.RemovePIDFile(pidFile)
		return true, nil
	}

	adminBin := m.GetAdminPath()
	stopped, err := system.StopDaemon(pidFile, func() error {
		cmd := exec.Command(adminBin,
			"-h", "127.0.0.1",
			"-P", fmt.Sprintf("%d", m.cfg.MariaDBPort),
			"-u", "root",
			fmt.Sprintf("-p%s", m.cfg.MariaDBRootPassword),
			"shutdown",
		)
		return cmd.Run()
	}, 15*time.Second)

	return stopped, err
}
