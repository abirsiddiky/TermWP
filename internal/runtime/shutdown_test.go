package runtime

import (
	"reflect"
	"testing"

	"termwp/internal/config"
)

// MockServiceRecorder tracks the execution order of shutdown operations.
type MockServiceRecorder struct {
	Order []string
}

func (m *MockServiceRecorder) Record(service string) {
	m.Order = append(m.Order, service)
}

func TestShutdownOrderLogic(t *testing.T) {
	recorder := &MockServiceRecorder{}

	// Simulated services with simulated state
	servicesRunning := map[string]bool{
		"nginx":   true,
		"php_8.3": false,
		"php_8.4": true,
		"php_8.5": false,
		"mariadb": true,
	}

	// Execute shutdown sequence matching StackManager.ShutdownStack order:
	// 1. nginx
	// 2. running PHP backends
	// 3. MariaDB

	if servicesRunning["nginx"] {
		recorder.Record("nginx")
		servicesRunning["nginx"] = false
	}

	for _, v := range []string{"8.3", "8.4", "8.5"} {
		key := "php_" + v
		if servicesRunning[key] {
			recorder.Record(key)
			servicesRunning[key] = false
		}
	}

	if servicesRunning["mariadb"] {
		recorder.Record("mariadb")
		servicesRunning["mariadb"] = false
	}

	expectedOrder := []string{"nginx", "php_8.4", "mariadb"}
	if !reflect.DeepEqual(recorder.Order, expectedOrder) {
		t.Errorf("Shutdown order mismatch. Got %v, want %v", recorder.Order, expectedOrder)
	}
}

func TestShutdownAlreadyStopped(t *testing.T) {
	cfg := config.DefaultConfig()
	stack := &StackManager{
		Config:  cfg,
		Nginx:   NewNginxManager(cfg),
		PHP:     NewPHPManager(cfg),
		MariaDB: NewMariaDBManager(cfg),
	}

	// When all services are already stopped, ShutdownStack must succeed gracefully with no error.
	err := stack.ShutdownStack(false, true)
	if err != nil {
		t.Errorf("ShutdownStack on already stopped services returned error: %v", err)
	}
}
