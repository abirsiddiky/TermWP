package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"termwp/internal/config"
	"termwp/internal/system"
	"termwp/internal/ui"
)

// NginxManager manages the single global Nginx instance and virtual hosts.
type NginxManager struct {
	cfg *config.GlobalConfig
}

// NewNginxManager creates an initialized NginxManager.
func NewNginxManager(cfg *config.GlobalConfig) *NginxManager {
	return &NginxManager{cfg: cfg}
}

// GetBinaryPath returns the nginx executable path.
func (m *NginxManager) GetBinaryPath() string {
	base := config.NginxRuntimeDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(base, "nginx.exe")
	}
	// On Linux portable builds
	candidates := []string{
		filepath.Join(base, "sbin", "nginx"),
		filepath.Join(base, "bin", "nginx"),
		filepath.Join(base, "nginx"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// PIDFilePath returns the path to nginx.pid.
func (m *NginxManager) PIDFilePath() string {
	return filepath.Join(config.LogsDir(), "nginx.pid")
}

// MasterConfPath returns the path to main nginx.conf.
func (m *NginxManager) MasterConfPath() string {
	return filepath.Join(config.NginxConfDir(), "nginx.conf")
}

// SiteConfPath returns the path to a site's virtual host conf.
func (m *NginxManager) SiteConfPath(name string) string {
	return filepath.Join(config.NginxSitesConfDir(), fmt.Sprintf("%s.conf", strings.ToLower(name)))
}

// EnsureInstalled ensures nginx binary and directory structure exist.
func (m *NginxManager) EnsureInstalled(tracker *ui.StepTracker) error {
	bin := m.GetBinaryPath()
	if _, err := os.Stat(bin); err == nil {
		return m.EnsureMasterConfig()
	}

	src, err := config.GetNginxSource(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	downloader := NewDownloader(tracker)
	if err := downloader.FetchAndInstall(src, config.NginxRuntimeDir()); err != nil {
		return fmt.Errorf("failed installing nginx: %w", err)
	}

	return m.EnsureMasterConfig()
}

// EnsureMasterConfig writes the main nginx.conf if missing or updated.
func (m *NginxManager) EnsureMasterConfig() error {
	confPath := m.MasterConfPath()
	pidPath := filepath.ToSlash(m.PIDFilePath())
	logsDir := filepath.ToSlash(config.LogsDir())
	sitesDir := filepath.ToSlash(config.NginxSitesConfDir())
	tempDir := filepath.ToSlash(filepath.Join(config.RuntimeDir(), "nginx", "temp"))

	// Create all temp directories needed by nginx
	_ = os.MkdirAll(tempDir, 0755)
	_ = os.MkdirAll(filepath.Join(config.RuntimeDir(), "nginx", "temp", "client_body"), 0755)
	_ = os.MkdirAll(filepath.Join(config.RuntimeDir(), "nginx", "temp", "proxy"), 0755)
	_ = os.MkdirAll(filepath.Join(config.RuntimeDir(), "nginx", "temp", "fastcgi"), 0755)
	_ = os.MkdirAll(filepath.Join(config.RuntimeDir(), "nginx", "temp", "uwsgi"), 0755)
	_ = os.MkdirAll(filepath.Join(config.RuntimeDir(), "nginx", "temp", "scgi"), 0755)

	// Copy standard configuration assets from shipping nginx installation if available
	shippingConfDir := filepath.Join(config.RuntimeDir(), "nginx", "conf")
	if entries, err := os.ReadDir(shippingConfDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "nginx.conf" {
				continue
			}
			srcFile := filepath.Join(shippingConfDir, entry.Name())
			destFile := filepath.Join(config.NginxConfDir(), entry.Name())
			if _, err := os.Stat(destFile); os.IsNotExist(err) {
				data, rErr := os.ReadFile(srcFile)
				if rErr == nil {
					_ = os.WriteFile(destFile, data, 0644)
				}
			}
		}
	}

	// Always ensure fastcgi_params exists
	fastcgiParamsPath := filepath.Join(config.NginxConfDir(), "fastcgi_params")
	if _, err := os.Stat(fastcgiParamsPath); os.IsNotExist(err) {
		fastcgiParams := `fastcgi_param  QUERY_STRING       $query_string;
fastcgi_param  REQUEST_METHOD     $request_method;
fastcgi_param  CONTENT_TYPE       $content_type;
fastcgi_param  CONTENT_LENGTH     $content_length;

fastcgi_param  SCRIPT_NAME        $fastcgi_script_name;
fastcgi_param  REQUEST_URI        $request_uri;
fastcgi_param  DOCUMENT_URI       $document_uri;
fastcgi_param  DOCUMENT_ROOT      $document_root;
fastcgi_param  SERVER_PROTOCOL    $server_protocol;
fastcgi_param  REQUEST_SCHEME     $scheme;
fastcgi_param  HTTPS              $https if_not_empty;

fastcgi_param  GATEWAY_INTERFACE  CGI/1.1;
fastcgi_param  SERVER_SOFTWARE    nginx/$nginx_version;

fastcgi_param  REMOTE_ADDR        $remote_addr;
fastcgi_param  REMOTE_PORT        $remote_port;
fastcgi_param  SERVER_ADDR        $server_addr;
fastcgi_param  SERVER_PORT        $server_port;
fastcgi_param  SERVER_NAME        $server_name;

# PHP only, required if PHP was built with --enable-force-cgi-redirect
fastcgi_param  REDIRECT_STATUS    200;
`
		_ = os.WriteFile(fastcgiParamsPath, []byte(fastcgiParams), 0644)
	}

	content := fmt.Sprintf(`# TermWP Master Nginx Configuration
worker_processes auto;
pid "%s";
error_log "%s/nginx_error.log" warn;

events {
    worker_connections 1024;
    multi_accept on;
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';

    access_log "%s/nginx_access.log" main;

    sendfile        on;
    tcp_nopush     on;
    tcp_nodelay    on;
    keepalive_timeout  65;
    types_hash_max_size 2048;
    server_names_hash_bucket_size 128;

    client_max_body_size 256M;
    client_body_buffer_size 128k;

    # Temp paths for portable execution
    client_body_temp_path "%s/client_body" 1 2;
    proxy_temp_path       "%s/proxy" 1 2;
    fastcgi_temp_path     "%s/fastcgi" 1 2;
    uwsgi_temp_path       "%s/uwsgi" 1 2;
    scgi_temp_path        "%s/scgi" 1 2;

    # Gzip settings
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 5;
    gzip_types text/plain text/css text/xml application/json application/javascript application/xml+rss application/atom+xml image/svg+xml;

    # Include all virtual hosts
    include "%s/*.conf";
}
`, pidPath, logsDir, logsDir, tempDir, tempDir, tempDir, tempDir, tempDir, sitesDir)

	// Ensure mime.types exists alongside nginx.conf
	mimeTypesPath := filepath.Join(config.NginxConfDir(), "mime.types")
	if _, err := os.Stat(mimeTypesPath); os.IsNotExist(err) {
		mimes := `types {
    text/html                             html htm shtml;
    text/css                              css;
    text/xml                              xml;
    image/gif                             gif;
    image/jpeg                            jpeg jpg;
    application/javascript                js;
    application/atom+xml                  atom;
    application/rss+xml                   rss;
    font/ttf                              ttf;
    font/woff                             woff;
    font/woff2                            woff2;
    image/png                             png;
    image/svg+xml                         svg svgz;
    image/webp                            webp;
    image/x-icon                          ico;
    application/pdf                       pdf;
    application/zip                       zip;
    audio/mpeg                            mp3;
    video/mp4                             mp4;
}`
		_ = os.WriteFile(mimeTypesPath, []byte(mimes), 0644)
	}

	return os.WriteFile(confPath, []byte(content), 0644)
}

// SiteTemplateData holds data for rendering a site's nginx virtual host.
type SiteTemplateData struct {
	Domain        string
	DocumentRoot  string
	HTTPPort      int
	HTTPSPort     int
	PHPPort       int
	SSL           bool
	CertFile      string
	KeyFile       string
	AccessLog     string
	ErrorLog      string
}

const siteVHostTemplate = `# Nginx virtual host for {{ .Domain }}
{{- if .SSL }}
server {
    listen {{ .HTTPPort }};
    server_name {{ .Domain }};
    return 301 https://$host{{ if ne .HTTPSPort 443 }}:{{ .HTTPSPort }}{{ end }}$request_uri;
}

server {
    listen {{ .HTTPSPort }} ssl;
    http2 on;
    server_name {{ .Domain }};

    ssl_certificate "{{ .CertFile }}";
    ssl_certificate_key "{{ .KeyFile }}";
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    root "{{ .DocumentRoot }}";
    index index.php index.html index.htm;

    access_log "{{ .AccessLog }}";
    error_log "{{ .ErrorLog }}" warn;

    client_max_body_size 256M;

    # Pretty Permalinks
    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    # Pass PHP scripts to FastCGI
    location ~ \.php$ {
        try_files $uri =404;
        fastcgi_split_path_info ^(.+\.php)(/.+)$;
        fastcgi_pass 127.0.0.1:{{ .PHPPort }};
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        fastcgi_param PATH_INFO $fastcgi_path_info;
        fastcgi_buffers 16 16k;
        fastcgi_buffer_size 32k;
        include fastcgi_params;
    }

    # Static assets cache
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires max;
        log_not_found off;
        access_log off;
    }

    # Deny access to hidden files (.git, .env, etc.)
    location ~ /\. {
        deny all;
        access_log off;
        log_not_found off;
    }
}
{{- else }}
server {
    listen {{ .HTTPPort }};
    server_name {{ .Domain }};

    root "{{ .DocumentRoot }}";
    index index.php index.html index.htm;

    access_log "{{ .AccessLog }}";
    error_log "{{ .ErrorLog }}" warn;

    client_max_body_size 256M;

    # Pretty Permalinks
    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    # Pass PHP scripts to FastCGI
    location ~ \.php$ {
        try_files $uri =404;
        fastcgi_split_path_info ^(.+\.php)(/.+)$;
        fastcgi_pass 127.0.0.1:{{ .PHPPort }};
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        fastcgi_param PATH_INFO $fastcgi_path_info;
        fastcgi_buffers 16 16k;
        fastcgi_buffer_size 32k;
        include fastcgi_params;
    }

    # Static assets cache
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires max;
        log_not_found off;
        access_log off;
    }

    # Deny access to hidden files
    location ~ /\. {
        deny all;
        access_log off;
        log_not_found off;
    }
}
{{- end }}
`

// RenderSiteConfig renders the Nginx virtual host content for a site.
func RenderSiteConfig(siteName, domain string, ssl bool, phpPort, httpPort, httpsPort int) (string, error) {
	tmpl, err := template.New("vhost").Parse(siteVHostTemplate)
	if err != nil {
		return "", err
	}

	siteDir := filepath.ToSlash(config.SiteDir(siteName))
	accessLog := filepath.ToSlash(filepath.Join(config.LogsDir(), fmt.Sprintf("%s_access.log", siteName)))
	errorLog := filepath.ToSlash(filepath.Join(config.LogsDir(), fmt.Sprintf("%s_error.log", siteName)))
	certFile := filepath.ToSlash(filepath.Join(config.CertsDir(), fmt.Sprintf("%s.pem", domain)))
	keyFile := filepath.ToSlash(filepath.Join(config.CertsDir(), fmt.Sprintf("%s-key.pem", domain)))

	data := SiteTemplateData{
		Domain:       domain,
		DocumentRoot: siteDir,
		HTTPPort:     httpPort,
		HTTPSPort:    httpsPort,
		PHPPort:      phpPort,
		SSL:          ssl,
		CertFile:     certFile,
		KeyFile:      keyFile,
		AccessLog:    accessLog,
		ErrorLog:     errorLog,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// WriteSiteConfig generates and writes the site's nginx config file.
func (m *NginxManager) WriteSiteConfig(siteName, domain string, ssl bool, phpPort int) error {
	confStr, err := RenderSiteConfig(siteName, domain, ssl, phpPort, m.cfg.HTTPPort, m.cfg.HTTPSPort)
	if err != nil {
		return err
	}
	dest := m.SiteConfPath(siteName)
	return os.WriteFile(dest, []byte(confStr), 0644)
}

// RemoveSiteConfig removes the site's virtual host config file.
func (m *NginxManager) RemoveSiteConfig(name string) error {
	dest := m.SiteConfPath(name)
	if _, err := os.Stat(dest); err == nil {
		return os.Remove(dest)
	}
	return nil
}

// DisableSiteConfig renames the site's virtual host to .conf.disabled.
func (m *NginxManager) DisableSiteConfig(name string) error {
	active := m.SiteConfPath(name)
	disabled := active + ".disabled"
	if _, err := os.Stat(active); err == nil {
		_ = os.Remove(disabled)
		return os.Rename(active, disabled)
	}
	return nil
}

// EnableSiteConfig restores a disabled virtual host config.
func (m *NginxManager) EnableSiteConfig(name string) error {
	active := m.SiteConfPath(name)
	disabled := active + ".disabled"
	if _, err := os.Stat(disabled); err == nil {
		_ = os.Remove(active)
		return os.Rename(disabled, active)
	}
	return nil
}

// PrefixDir returns the runtime directory of nginx to use as its working prefix (-p).
func (m *NginxManager) PrefixDir() string {
	return config.NginxRuntimeDir()
}

// TestConfig runs `nginx -t` to validate configuration syntax.
func (m *NginxManager) TestConfig() error {
	bin := m.GetBinaryPath()
	cmd := exec.Command(bin, "-p", m.PrefixDir(), "-c", m.MasterConfPath(), "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx configuration test failed:\n%s", string(output))
	}
	return nil
}

// IsRunning checks if Nginx process is active.
func (m *NginxManager) IsRunning() bool {
	pid := system.ReadPIDFile(m.PIDFilePath())
	if pid <= 0 {
		return false
	}
	return system.IsProcessRunning(pid)
}

// Start launches the Nginx master process.
func (m *NginxManager) Start() error {
	if m.IsRunning() {
		return nil
	}

	if err := m.EnsureInstalled(nil); err != nil {
		return err
	}

	if err := m.EnsureMasterConfig(); err != nil {
		return err
	}

	if err := m.TestConfig(); err != nil {
		return err
	}

	bin := m.GetBinaryPath()
	args := []string{"-p", m.PrefixDir(), "-c", m.MasterConfPath()}
	stdoutLog := filepath.Join(config.LogsDir(), "nginx_stdout.log")
	stderrLog := filepath.Join(config.LogsDir(), "nginx_stderr.log")

	proc, err := system.SpawnBackgroundProcess(bin, args, m.PrefixDir(), stdoutLog, stderrLog)
	if err != nil {
		return fmt.Errorf("failed starting nginx: %w", err)
	}

	// Wait up to 3s for HTTP port to start listening
	_ = system.WaitForPort(m.cfg.HTTPPort, 3*time.Second)

	// If nginx didn't write its own PID file from nginx.conf, record proc.Pid
	if system.ReadPIDFile(m.PIDFilePath()) <= 0 {
		_ = system.WritePIDFile(m.PIDFilePath(), proc.Pid)
	}
	return nil
}

// Reload reloads the Nginx configuration gracefully without downtime.
func (m *NginxManager) Reload() error {
	if !m.IsRunning() {
		return m.Start()
	}

	bin := m.GetBinaryPath()
	cmd := exec.Command(bin, "-p", m.PrefixDir(), "-c", m.MasterConfPath(), "-s", "reload")
	return cmd.Run()
}

// Stop gracefully stops Nginx (`nginx -s quit`), with 5s timeout before force killing.
func (m *NginxManager) Stop(force bool) (bool, error) {
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

	bin := m.GetBinaryPath()
	stopped, err := system.StopDaemon(pidFile, func() error {
		cmd := exec.Command(bin, "-p", m.PrefixDir(), "-c", m.MasterConfPath(), "-s", "quit")
		return cmd.Run()
	}, 5*time.Second)

	return stopped, err
}
