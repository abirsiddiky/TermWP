package system

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// LogTarget defines a specific log file to monitor.
type LogTarget struct {
	Path  string
	Label string
	Style lipgloss.Style
}

// ReadLastLines reads the last n lines from a file and returns the lines along with the file offset.
func ReadLastLines(path string, n int) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return nil, 0, nil
	}

	// Read entire file or seek back if small/medium
	// For dev logs, reading lines with ring buffer
	scanner := bufio.NewScanner(f)
	var ring []string

	for scanner.Scan() {
		ring = append(ring, scanner.Text())
		if len(ring) > n {
			ring = ring[1:]
		}
	}

	return ring, fileSize, scanner.Err()
}

// StreamLogs monitors and displays output from multiple log targets concurrently in pure Go.
func StreamLogs(ctx context.Context, targets []LogTarget, lines int, follow bool) error {
	offsets := make([]int64, len(targets))

	// Print initial lines
	for i, t := range targets {
		if _, err := os.Stat(t.Path); os.IsNotExist(err) {
			continue
		}

		lastLines, offset, err := ReadLastLines(t.Path, lines)
		if err != nil {
			continue
		}
		offsets[i] = offset

		for _, l := range lastLines {
			prefix := t.Style.Render(fmt.Sprintf("[%s]", t.Label))
			fmt.Printf("%s %s\n", prefix, l)
		}
	}

	if !follow {
		return nil
	}

	// Polling loop for new content
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for i, t := range targets {
				stat, err := os.Stat(t.Path)
				if err != nil {
					continue
				}

				currentSize := stat.Size()
				if currentSize < offsets[i] {
					// File truncated or rotated: reset to start
					offsets[i] = 0
				}

				if currentSize > offsets[i] {
					f, err := os.Open(t.Path)
					if err != nil {
						continue
					}

					_, err = f.Seek(offsets[i], io.SeekStart)
					if err != nil {
						f.Close()
						continue
					}

					scanner := bufio.NewScanner(f)
					for scanner.Scan() {
						line := scanner.Text()
						prefix := t.Style.Render(fmt.Sprintf("[%s]", t.Label))
						fmt.Printf("%s %s\n", prefix, line)
					}

					newOffset, _ := f.Seek(0, io.SeekCurrent)
					offsets[i] = newOffset
					f.Close()
				}
			}
		}
	}
}

// IsWPDebugLogEnabled checks if WP_DEBUG_LOG is enabled in wp-config.php.
func IsWPDebugLogEnabled(siteDir string) bool {
	configPath := filepath.Join(siteDir, "wp-config.php")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, "'WP_DEBUG_LOG', true") || strings.Contains(content, "\"WP_DEBUG_LOG\", true")
}

// EnableWPDebugLog modifies wp-config.php to enable WP_DEBUG_LOG.
func EnableWPDebugLog(siteDir string) error {
	configPath := filepath.Join(siteDir, "wp-config.php")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)
	if strings.Contains(content, "WP_DEBUG_LOG") {
		content = strings.ReplaceAll(content, "define( 'WP_DEBUG_LOG', false );", "define( 'WP_DEBUG_LOG', true );")
		content = strings.ReplaceAll(content, "define('WP_DEBUG_LOG', false);", "define('WP_DEBUG_LOG', true);")
		content = strings.ReplaceAll(content, "define( \"WP_DEBUG_LOG\", false );", "define( \"WP_DEBUG_LOG\", true );")
	} else {
		// Insert before /* That's all, stop editing! */
		anchor := "/* That's all, stop editing!"
		replacement := "define( 'WP_DEBUG_LOG', true );\n\n/* That's all, stop editing!"
		if strings.Contains(content, anchor) {
			content = strings.Replace(content, anchor, replacement, 1)
		} else {
			content += "\ndefine( 'WP_DEBUG_LOG', true );\n"
		}
	}

	return os.WriteFile(configPath, []byte(content), 0644)
}
