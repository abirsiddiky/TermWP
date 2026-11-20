package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type mockServiceChecker struct {
	nginxUp   bool
	mariadbUp bool
	phpUp     bool
	httpCode  int
	httpErr   error
}

func (m *mockServiceChecker) IsNginxRunning() bool {
	return m.nginxUp
}

func (m *mockServiceChecker) IsMariaDBReachable() bool {
	return m.mariadbUp
}

func (m *mockServiceChecker) IsPHPReachable(version string) bool {
	return m.phpUp
}

func (m *mockServiceChecker) ProbeHTTP(ctx context.Context, domain string, ssl bool, httpPort, httpsPort int) (int, error) {
	return m.httpCode, m.httpErr
}

func TestComputeSiteStatus(t *testing.T) {
	site := &Site{
		Name:       "example.local",
		Domain:     "example.local",
		PHPVersion: "8.4",
		Enabled:    true,
	}

	// 1. (enabled + all up) = running
	checkerAllUp := &mockServiceChecker{
		nginxUp:   true,
		mariadbUp: true,
		phpUp:     true,
		httpCode:  200,
	}
	st := ComputeSiteStatus(site, checkerAllUp)
	if st.State != StatusRunning {
		t.Errorf("expected state %s, got %s (detail: %s)", StatusRunning, st.State, st.Detail)
	}

	// 2. (enabled + nginx down) = stopped
	checkerNginxDown := &mockServiceChecker{
		nginxUp:   false,
		mariadbUp: true,
		phpUp:     true,
		httpCode:  200,
	}
	st = ComputeSiteStatus(site, checkerNginxDown)
	if st.State != StatusStopped {
		t.Errorf("expected state %s, got %s", StatusStopped, st.State)
	}
	if st.Detail != "nginx down" {
		t.Errorf("expected detail 'nginx down', got '%s'", st.Detail)
	}

	// 3. (enabled + php down) = stopped
	checkerPHPDown := &mockServiceChecker{
		nginxUp:   true,
		mariadbUp: true,
		phpUp:     false,
		httpCode:  200,
	}
	st = ComputeSiteStatus(site, checkerPHPDown)
	if st.State != StatusStopped {
		t.Errorf("expected state %s, got %s", StatusStopped, st.State)
	}
	if st.Detail != "php 8.4 down" {
		t.Errorf("expected detail 'php 8.4 down', got '%s'", st.Detail)
	}

	// 4. (enabled + mariadb down) = stopped
	checkerMariaDBDown := &mockServiceChecker{
		nginxUp:   true,
		mariadbUp: false,
		phpUp:     true,
		httpCode:  200,
	}
	st = ComputeSiteStatus(site, checkerMariaDBDown)
	if st.State != StatusStopped {
		t.Errorf("expected state %s, got %s", StatusStopped, st.State)
	}
	if st.Detail != "mariadb down" {
		t.Errorf("expected detail 'mariadb down', got '%s'", st.Detail)
	}

	// 5. (enabled + http probe 502) = stopped
	checker502 := &mockServiceChecker{
		nginxUp:   true,
		mariadbUp: true,
		phpUp:     true,
		httpCode:  502,
	}
	st = ComputeSiteStatus(site, checker502)
	if st.State != StatusStopped {
		t.Errorf("expected state %s, got %s", StatusStopped, st.State)
	}
	if st.Detail != "HTTP 502" {
		t.Errorf("expected detail 'HTTP 502', got '%s'", st.Detail)
	}

	// 6. (enabled + http connection error) = stopped
	checkerConnErr := &mockServiceChecker{
		nginxUp:   true,
		mariadbUp: true,
		phpUp:     true,
		httpErr:   errors.New("connection refused"),
	}
	st = ComputeSiteStatus(site, checkerConnErr)
	if st.State != StatusStopped {
		t.Errorf("expected state %s, got %s", StatusStopped, st.State)
	}
	if st.Detail != "unreachable" {
		t.Errorf("expected detail 'unreachable', got '%s'", st.Detail)
	}

	// 7. (disabled) = off
	disabledSite := &Site{
		Name:       "disabled.local",
		Domain:     "disabled.local",
		PHPVersion: "8.4",
		Enabled:    false,
	}
	st = ComputeSiteStatus(disabledSite, checkerAllUp)
	if st.State != StatusOff {
		t.Errorf("expected state %s, got %s", StatusOff, st.State)
	}
}

func TestOldStateMigration(t *testing.T) {
	// JSON with legacy format: "status": "on" without "enabled"
	legacyJSON := `{
		"sites": {
			"active.local": {
				"name": "active.local",
				"domain": "active.local",
				"php_version": "8.4",
				"status": "on"
			},
			"disabled.local": {
				"name": "disabled.local",
				"domain": "disabled.local",
				"php_version": "8.4",
				"status": "off"
			}
		}
	}`

	var st State
	if err := json.Unmarshal([]byte(legacyJSON), &st); err != nil {
		t.Fatalf("unmarshal legacy state failed: %v", err)
	}

	// Simulate migration performed in readStateUnlocked
	for _, site := range st.Sites {
		if !site.Enabled && site.Status == StatusOn {
			site.Enabled = true
		}
		site.Status = ""
	}

	activeSite := st.Sites["active.local"]
	if !activeSite.Enabled {
		t.Errorf("expected active.local to be migrated to enabled=true, got %v", activeSite.Enabled)
	}
	if activeSite.Status != "" {
		t.Errorf("expected status to be cleared on load, got %s", activeSite.Status)
	}

	disabledSite := st.Sites["disabled.local"]
	if disabledSite.Enabled {
		t.Errorf("expected disabled.local to be migrated to enabled=false, got %v", disabledSite.Enabled)
	}
}
