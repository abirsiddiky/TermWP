package system

import (
	"fmt"
	"net"
	"time"
)

// IsPortAvailable checks if a TCP port is open for binding on localhost.
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// FindAvailablePort returns the preferred port if available, otherwise tests fallback
// or finds the next consecutive free port.
func FindAvailablePort(preferred int, fallback int) int {
	if IsPortAvailable(preferred) {
		return preferred
	}
	if fallback > 0 && IsPortAvailable(fallback) {
		return fallback
	}
	// Search in range fallback+1 .. fallback+100
	start := fallback
	if start <= 0 {
		start = preferred + 1000
	}
	for p := start; p < start+100; p++ {
		if IsPortAvailable(p) {
			return p
		}
	}
	return preferred
}

// WaitForPort waits until a port starts listening (becomes active), up to timeout.
func WaitForPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// WaitForPortClosed waits until a port stops listening, up to timeout.
func WaitForPortClosed(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		time.Sleep(150 * time.Millisecond)
	}
	return false
}
