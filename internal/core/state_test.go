package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"termwp/internal/config"
)

func setupTestStore(t *testing.T) (*StateStore, string) {
	tempDir, err := os.MkdirTemp("", "termwp_state_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Setenv("TERMWP_HOME", tempDir)
	config.ResetHomeDirCache()

	if err := config.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories failed: %v", err)
	}

	store := NewStateStore(filepath.Join(tempDir, "state.lock"))
	return store, tempDir
}

func TestStateStoreLifecycle(t *testing.T) {
	store, tempDir := setupTestStore(t)
	defer os.RemoveAll(tempDir)

	// 1. Initial read should be empty
	sites, err := store.ListSites()
	if err != nil {
		t.Fatalf("ListSites failed: %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("expected 0 sites, got %d", len(sites))
	}

	// 2. Add site
	s1 := &Site{
		Name:       "blog.local",
		Domain:     "blog.local",
		PHPVersion: "8.4",
		SSL:        false,
		Status:     StatusOn,
		DBName:     "wp_blog_local",
		DBUser:     "user_blog",
		DBPass:     "secretpass123",
		CreatedAt:  time.Now(),
	}
	if err := store.AddSite(s1); err != nil {
		t.Fatalf("AddSite failed: %v", err)
	}

	// 3. Duplicate should fail
	if err := store.AddSite(s1); err == nil {
		t.Errorf("expected duplicate AddSite to fail, but it succeeded")
	}

	// 4. Retrieve site
	retrieved, err := store.GetSite("blog.local")
	if err != nil {
		t.Fatalf("GetSite failed: %v", err)
	}
	if retrieved.DBName != "wp_blog_local" {
		t.Errorf("DBName mismatch: got %s, want wp_blog_local", retrieved.DBName)
	}

	// 5. Update site status
	if err := store.SetSiteStatus("blog.local", StatusOff); err != nil {
		t.Fatalf("SetSiteStatus failed: %v", err)
	}

	retrieved, _ = store.GetSite("blog.local")
	if retrieved.Status != StatusOff {
		t.Errorf("expected status %s, got %s", StatusOff, retrieved.Status)
	}

	// 6. Test shutdown state
	isShutdown, err := store.IsShutdown()
	if err != nil {
		t.Fatalf("IsShutdown failed: %v", err)
	}
	if isShutdown {
		t.Errorf("expected shutdown to be false initially")
	}

	if err := store.SetShutdown(true); err != nil {
		t.Fatalf("SetShutdown failed: %v", err)
	}
	isShutdown, _ = store.IsShutdown()
	if !isShutdown {
		t.Errorf("expected shutdown to be true")
	}

	// 7. Delete site
	if err := store.DeleteSite("blog.local"); err != nil {
		t.Fatalf("DeleteSite failed: %v", err)
	}
	_, err = store.GetSite("blog.local")
	if err == nil {
		t.Errorf("expected GetSite to fail after deletion")
	}
}
