package system

import (
	"strings"
	"testing"

	"termwp/internal/config"
)

func TestDoctorEngine(t *testing.T) {
	cfg := config.DefaultConfig()
	doc := NewDoctorEngine(cfg)

	// Run diagnostics with simulated shutdown = true
	items := doc.RunDiagnostics(true, true, []string{"8.4"})
	if len(items) == 0 {
		t.Fatalf("expected diagnostic items, got 0")
	}

	rendered := RenderDoctorChecklist(items)
	if !strings.Contains(rendered, "Summary:") {
		t.Errorf("rendered doctor checklist missing summary line")
	}

	if !strings.Contains(rendered, "System") {
		t.Errorf("rendered doctor checklist missing System category")
	}

	// Verify shutdown state reported normally
	hasShutdownCheck := false
	for _, item := range items {
		if strings.Contains(item.Details, "stopped (normal") {
			hasShutdownCheck = true
			break
		}
	}
	if !hasShutdownCheck {
		t.Errorf("expected shutdown state to be reported as stopped (normal)")
	}
}
