package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"termwp/internal/config"
)

const (
	StatusOn  = "on"
	StatusOff = "off"
)

// Site represents a single registered WordPress site.
type Site struct {
	Name       string    `json:"name"`
	Domain     string    `json:"domain"`
	PHPVersion string    `json:"php_version"`
	SSL        bool      `json:"ssl"`
	Status     string    `json:"status"` // "on" or "off"
	DBName     string    `json:"db_name"`
	DBUser     string    `json:"db_user"`
	DBPass     string    `json:"db_pass"`
	SiteTitle  string    `json:"site_title,omitempty"`
	AdminUser  string    `json:"admin_user,omitempty"`
	AdminEmail string    `json:"admin_email,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// State represents the persisted state.json file.
type State struct {
	Sites       map[string]*Site `json:"sites"`
	Shutdown    bool             `json:"shutdown"`
	LastUpdated time.Time        `json:"last_updated"`
}

// StateStore manages thread-safe and process-safe access to state.json.
type StateStore struct {
	mu       sync.RWMutex
	fileLock *flock.Flock
}

var (
	defaultStore *StateStore
	storeOnce    sync.Once
)

// GetStateStore returns the singleton StateStore instance.
func GetStateStore() *StateStore {
	storeOnce.Do(func() {
		defaultStore = NewStateStore(config.StateLockPath())
	})
	return defaultStore
}

// NewStateStore creates a new StateStore with the given lock file path.
func NewStateStore(lockPath string) *StateStore {
	return &StateStore{
		fileLock: flock.New(lockPath),
	}
}

// WithLock executes a callback while holding the file lock on state.json.
func (s *StateStore) WithLock(ctx context.Context, fn func(st *State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	locked, err := s.fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("error attempting to acquire state lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire state lock: another termwp process may be running")
	}
	defer func() {
		_ = s.fileLock.Unlock()
	}()

	st, err := s.readStateUnlocked()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return s.writeStateUnlocked(st)
}

// ReadState reads the state without holding an exclusive lock (shared read lock).
func (s *StateStore) ReadState(ctx context.Context) (*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	locked, err := s.fileLock.TryRLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("error acquiring read lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("could not acquire state read lock")
	}
	defer func() {
		_ = s.fileLock.Unlock()
	}()

	return s.readStateUnlocked()
}

func (s *StateStore) readStateUnlocked() (*State, error) {
	filePath := config.StateFilePath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &State{
			Sites:       make(map[string]*Site),
			Shutdown:    false,
			LastUpdated: time.Now(),
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	if st.Sites == nil {
		st.Sites = make(map[string]*Site)
	}
	return &st, nil
}

func (s *StateStore) writeStateUnlocked(st *State) error {
	st.LastUpdated = time.Now()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	tmpPath := config.StateFilePath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp state file: %w", err)
	}

	if err := os.Rename(tmpPath, config.StateFilePath()); err != nil {
		_ = os.Remove(config.StateFilePath())
		if err := os.Rename(tmpPath, config.StateFilePath()); err != nil {
			return fmt.Errorf("failed to atomic write state file: %w", err)
		}
	}

	return nil
}

// GetSite retrieves a site by name.
func (s *StateStore) GetSite(name string) (*Site, error) {
	st, err := s.ReadState(nil)
	if err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	site, ok := st.Sites[name]
	if !ok {
		return nil, fmt.Errorf("site '%s' not found", name)
	}
	return site, nil
}

// ListSites returns all sites sorted by name.
func (s *StateStore) ListSites() ([]*Site, error) {
	st, err := s.ReadState(nil)
	if err != nil {
		return nil, err
	}

	sites := make([]*Site, 0, len(st.Sites))
	for _, site := range st.Sites {
		sites = append(sites, site)
	}

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].Name < sites[j].Name
	})

	return sites, nil
}

// AddSite registers a new site. Returns an error if it already exists.
func (s *StateStore) AddSite(site *Site) error {
	return s.WithLock(nil, func(st *State) error {
		key := strings.ToLower(site.Name)
		if _, exists := st.Sites[key]; exists {
			return fmt.Errorf("site '%s' already exists", site.Name)
		}
		if site.CreatedAt.IsZero() {
			site.CreatedAt = time.Now()
		}
		site.UpdatedAt = time.Now()
		st.Sites[key] = site
		return nil
	})
}

// UpdateSite updates an existing site.
func (s *StateStore) UpdateSite(site *Site) error {
	return s.WithLock(nil, func(st *State) error {
		key := strings.ToLower(site.Name)
		if _, exists := st.Sites[key]; !exists {
			return fmt.Errorf("site '%s' does not exist", site.Name)
		}
		site.UpdatedAt = time.Now()
		st.Sites[key] = site
		return nil
	})
}

// DeleteSite deletes a site from state.json.
func (s *StateStore) DeleteSite(name string) error {
	return s.WithLock(nil, func(st *State) error {
		key := strings.ToLower(name)
		if _, exists := st.Sites[key]; !exists {
			return fmt.Errorf("site '%s' does not exist", name)
		}
		delete(st.Sites, key)
		return nil
	})
}

// SetSiteStatus changes a site's status to "on" or "off".
func (s *StateStore) SetSiteStatus(name string, status string) error {
	return s.WithLock(nil, func(st *State) error {
		key := strings.ToLower(name)
		site, exists := st.Sites[key]
		if !exists {
			return fmt.Errorf("site '%s' does not exist", name)
		}
		site.Status = status
		site.UpdatedAt = time.Now()
		return nil
	})
}

// SetShutdown records whether TermWP is currently shutdown.
func (s *StateStore) SetShutdown(shutdown bool) error {
	return s.WithLock(nil, func(st *State) error {
		st.Shutdown = shutdown
		return nil
	})
}

// IsShutdown checks if the shutdown flag is set.
func (s *StateStore) IsShutdown() (bool, error) {
	st, err := s.ReadState(nil)
	if err != nil {
		return false, err
	}
	return st.Shutdown, nil
}

// GetActivePHPVersions returns a sorted deduplicated slice of PHP versions
// used by active ("on") sites.
func (s *StateStore) GetActivePHPVersions() ([]string, error) {
	st, err := s.ReadState(nil)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, site := range st.Sites {
		if site.Status == StatusOn && site.PHPVersion != "" {
			seen[site.PHPVersion] = true
		}
	}

	versions := make([]string, 0, len(seen))
	for v := range seen {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return versions, nil
}

// HasActiveSites returns true if any site is currently "on".
func (s *StateStore) HasActiveSites() (bool, error) {
	st, err := s.ReadState(nil)
	if err != nil {
		return false, err
	}
	for _, site := range st.Sites {
		if site.Status == StatusOn {
			return true, nil
		}
	}
	return false, nil
}
