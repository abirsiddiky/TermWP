package system

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	BlockBeginMarker = "# TermWP BEGIN"
	BlockEndMarker   = "# TermWP END"
)

// HostsFilePath returns the path to the system hosts file.
func HostsFilePath() string {
	if runtime.GOOS == "windows" {
		systemRoot := os.Getenv("SystemRoot")
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		return filepath.Join(systemRoot, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// BackupHostsFile creates a safety backup of the hosts file before editing.
func BackupHostsFile(hostsPath string) error {
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		return err
	}
	backupPath := hostsPath + ".termwp.bak"
	return os.WriteFile(backupPath, data, 0644)
}

// ParseHostsFile parses lines into before-block, inside-block, and after-block sections.
func ParseHostsFile(content string) (before []string, inside []string, after []string, hasBlock bool) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	inBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == BlockBeginMarker {
			inBlock = true
			hasBlock = true
			continue
		}
		if trimmed == BlockEndMarker {
			inBlock = false
			continue
		}

		if inBlock {
			inside = append(inside, line)
		} else if !hasBlock {
			before = append(before, line)
		} else {
			after = append(after, line)
		}
	}

	return before, inside, after, hasBlock
}

// ParseManagedDomains extracts domains configured inside the TermWP block.
func ParseManagedDomains(inside []string) map[string]string {
	domains := make(map[string]string)
	for _, line := range inside {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			ip := fields[0]
			for _, d := range fields[1:] {
				domains[strings.ToLower(d)] = ip
			}
		}
	}
	return domains
}

// BuildHostsContent reconstructs hosts content with updated TermWP managed domains.
func BuildHostsContent(before []string, after []string, domains map[string]string) string {
	var sb strings.Builder

	// Write before-block lines
	for _, line := range before {
		sb.WriteString(line + "\n")
	}

	// Only add TermWP block if there are domains to manage
	if len(domains) > 0 {
		// Ensure a blank line before block if needed
		if len(before) > 0 && strings.TrimSpace(before[len(before)-1]) != "" {
			sb.WriteString("\n")
		}

		sb.WriteString(BlockBeginMarker + "\n")

		// Sort domains alphabetically
		sortedKeys := make([]string, 0, len(domains))
		for d := range domains {
			sortedKeys = append(sortedKeys, d)
		}
		sort.Strings(sortedKeys)

		for _, d := range sortedKeys {
			ip := domains[d]
			if ip == "" {
				ip = "127.0.0.1"
			}
			sb.WriteString(fmt.Sprintf("%-16s %s\n", ip, d))
		}

		sb.WriteString(BlockEndMarker + "\n")
	}

	// Write after-block lines
	for _, line := range after {
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

// ReadAndModifyHosts reads the system hosts file, applies the modification function, and writes back.
func ReadAndModifyHosts(modifyFn func(domains map[string]string)) error {
	hostsPath := HostsFilePath()

	data, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("failed reading hosts file %s: %w", hostsPath, err)
	}

	before, inside, after, _ := ParseHostsFile(string(data))
	domains := ParseManagedDomains(inside)

	modifyFn(domains)

	newContent := BuildHostsContent(before, after, domains)

	// Backup before write
	_ = BackupHostsFile(hostsPath)

	err = os.WriteFile(hostsPath, []byte(newContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

// AddHostsEntry adds a domain to the TermWP block. Automatically handles elevation if needed.
func AddHostsEntry(domain, ip string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if ip == "" {
		ip = "127.0.0.1"
	}

	err := ReadAndModifyHosts(func(domains map[string]string) {
		domains[domain] = ip
	})

	if err != nil {
		// If permission denied and not elevated, trigger automatic elevation
		if !IsElevated() {
			self, exeErr := os.Executable()
			if exeErr == nil {
				return RunElevated(self, []string{"hosts", "add", domain, ip})
			}
		}
		return fmt.Errorf("permission denied writing to hosts file (%s). Run with administrator/root privileges: %w", HostsFilePath(), err)
	}

	return nil
}

// RemoveHostsEntry removes a domain from the TermWP block.
func RemoveHostsEntry(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	err := ReadAndModifyHosts(func(domains map[string]string) {
		delete(domains, domain)
	})

	if err != nil {
		if !IsElevated() {
			self, exeErr := os.Executable()
			if exeErr == nil {
				return RunElevated(self, []string{"hosts", "remove", domain})
			}
		}
		return fmt.Errorf("permission denied writing to hosts file (%s): %w", HostsFilePath(), err)
	}

	return nil
}

// ClearAllHostsEntries removes all TermWP managed domains and deletes the block.
func ClearAllHostsEntries() error {
	err := ReadAndModifyHosts(func(domains map[string]string) {
		for k := range domains {
			delete(domains, k)
		}
	})

	if err != nil {
		if !IsElevated() {
			self, exeErr := os.Executable()
			if exeErr == nil {
				return RunElevated(self, []string{"hosts", "clear"})
			}
		}
		return fmt.Errorf("permission denied writing to hosts file (%s): %w", HostsFilePath(), err)
	}

	return nil
}

// GetManagedHostsDomains returns all domains currently inside the TermWP block.
func GetManagedHostsDomains() ([]string, error) {
	data, err := os.ReadFile(HostsFilePath())
	if err != nil {
		return nil, err
	}
	_, inside, _, _ := ParseHostsFile(string(data))
	domainsMap := ParseManagedDomains(inside)

	domains := make([]string, 0, len(domainsMap))
	for d := range domainsMap {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	return domains, nil
}
