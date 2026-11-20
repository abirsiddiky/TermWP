package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"termwp/internal/config"
	"termwp/internal/runtime"
)

// SiteLiveStatus represents the real-time runtime state of a WordPress site.
type SiteLiveStatus struct {
	State  string `json:"state"`            // "running", "stopped", "off"
	Detail string `json:"detail,omitempty"` // e.g. "nginx down", "php 8.4 down", "mariadb down", etc.
}

// ServiceChecker abstracts service state and network probe checks for testability.
type ServiceChecker interface {
	IsNginxRunning() bool
	IsMariaDBReachable() bool
	IsPHPReachable(version string) bool
	ProbeHTTP(ctx context.Context, domain string, ssl bool, httpPort, httpsPort int) (statusCode int, err error)
}

// RealServiceChecker implements ServiceChecker using real daemons and network calls.
type RealServiceChecker struct {
	Config *config.GlobalConfig
	Stack  *runtime.StackManager
}

// NewRealServiceChecker constructs a RealServiceChecker.
func NewRealServiceChecker(cfg *config.GlobalConfig, stack *runtime.StackManager) *RealServiceChecker {
	return &RealServiceChecker{
		Config: cfg,
		Stack:  stack,
	}
}

func (r *RealServiceChecker) IsNginxRunning() bool {
	if r.Stack == nil || r.Stack.Nginx == nil {
		return false
	}
	httpPort := 80
	httpsPort := 443
	if r.Config != nil {
		if r.Config.HTTPPort > 0 {
			httpPort = r.Config.HTTPPort
		}
		if r.Config.HTTPSPort > 0 {
			httpsPort = r.Config.HTTPSPort
		}
	}
	return r.Stack.Nginx.IsRunning() || isTCPPortListening(httpPort, 200*time.Millisecond) || isTCPPortListening(httpsPort, 200*time.Millisecond)
}

func (r *RealServiceChecker) IsMariaDBReachable() bool {
	if r.Stack == nil || r.Stack.MariaDB == nil {
		return false
	}
	port := 3306
	if r.Config != nil && r.Config.MariaDBPort > 0 {
		port = r.Config.MariaDBPort
	}
	return isTCPPortListening(port, 400*time.Millisecond)
}

func (r *RealServiceChecker) IsPHPReachable(version string) bool {
	if r.Stack == nil || r.Stack.PHP == nil {
		return false
	}
	port := r.Stack.PHP.GetPort(version)
	return isTCPPortListening(port, 400*time.Millisecond)
}

func (r *RealServiceChecker) ProbeHTTP(ctx context.Context, domain string, ssl bool, httpPort, httpsPort int) (int, error) {
	proto := "http"
	port := httpPort
	if ssl {
		proto = "https"
		port = httpsPort
	}
	if port <= 0 {
		if ssl {
			port = 443
		} else {
			port = 80
		}
	}

	probeURL := fmt.Sprintf("%s://127.0.0.1:%d/", proto, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return 0, err
	}
	req.Host = domain

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         domain,
		},
		DisableKeepAlives: true,
		DialContext: (&net.Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow redirects: a 301/302 response from Nginx/WordPress is a healthy live response!
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// ComputeSiteStatus calculates the live runtime state of a site given a ServiceChecker.
func ComputeSiteStatus(site *Site, checker ServiceChecker) SiteLiveStatus {
	if !site.Enabled {
		return SiteLiveStatus{
			State:  StatusOff,
			Detail: "",
		}
	}

	nginxUp := checker.IsNginxRunning()
	mariadbUp := checker.IsMariaDBReachable()
	phpUp := checker.IsPHPReachable(site.PHPVersion)

	if !nginxUp || !mariadbUp || !phpUp {
		var downParts []string
		if !nginxUp {
			downParts = append(downParts, "nginx down")
		}
		if !phpUp {
			downParts = append(downParts, fmt.Sprintf("php %s down", site.PHPVersion))
		}
		if !mariadbUp {
			downParts = append(downParts, "mariadb down")
		}
		detail := strings.Join(downParts, ", ")
		if len(downParts) >= 2 {
			detail = "services down"
		}
		return SiteLiveStatus{
			State:  StatusStopped,
			Detail: detail,
		}
	}

	// Daemons are running: perform HTTP probe
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	httpPort := 80
	httpsPort := 443
	if realChecker, ok := checker.(*RealServiceChecker); ok && realChecker.Config != nil {
		httpPort = realChecker.Config.HTTPPort
		httpsPort = realChecker.Config.HTTPSPort
	}

	statusCode, err := checker.ProbeHTTP(ctx, site.Domain, site.SSL, httpPort, httpsPort)
	if err != nil {
		return SiteLiveStatus{
			State:  StatusStopped,
			Detail: "unreachable",
		}
	}

	if statusCode >= 500 {
		return SiteLiveStatus{
			State:  StatusStopped,
			Detail: fmt.Sprintf("HTTP %d", statusCode),
		}
	}

	return SiteLiveStatus{
		State:  StatusRunning,
		Detail: "",
	}
}

// CheckSiteStatus computes the real live status of a site in production.
func CheckSiteStatus(ctx context.Context, site *Site, cfg *config.GlobalConfig, stack *runtime.StackManager) SiteLiveStatus {
	if !site.Enabled {
		return SiteLiveStatus{
			State:  StatusOff,
			Detail: "",
		}
	}

	if stack == nil {
		var err error
		stack, err = runtime.GetStackManager()
		if err != nil {
			return SiteLiveStatus{
				State:  StatusStopped,
				Detail: "stack error",
			}
		}
	}

	if cfg == nil {
		cfg, _ = config.LoadConfig()
		if cfg == nil {
			cfg = config.DefaultConfig()
		}
	}

	checker := NewRealServiceChecker(cfg, stack)
	return ComputeSiteStatus(site, checker)
}

// CheckAllSitesStatusParallel queries live status for all sites concurrently.
func CheckAllSitesStatusParallel(ctx context.Context, sites []*Site, cfg *config.GlobalConfig, stack *runtime.StackManager) map[string]SiteLiveStatus {
	results := make(map[string]SiteLiveStatus, len(sites))
	var mu sync.Mutex
	var wg sync.WaitGroup

	if stack == nil {
		stack, _ = runtime.GetStackManager()
	}
	if cfg == nil {
		cfg, _ = config.LoadConfig()
	}
	checker := NewRealServiceChecker(cfg, stack)

	for _, s := range sites {
		wg.Add(1)
		go func(site *Site) {
			defer wg.Done()
			st := ComputeSiteStatus(site, checker)
			mu.Lock()
			results[site.Name] = st
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return results
}

func isTCPPortListening(port int, timeout time.Duration) bool {
	if port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
