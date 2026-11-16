package runtime

import (
	"strings"
	"testing"
)

func TestRenderSiteConfigHTTP(t *testing.T) {
	conf, err := RenderSiteConfig("myblog.local", "myblog.local", false, 9084, 80, 443)
	if err != nil {
		t.Fatalf("RenderSiteConfig failed: %v", err)
	}

	// Must contain domain
	if !strings.Contains(conf, "server_name myblog.local;") {
		t.Errorf("missing server_name myblog.local")
	}

	// Must contain pretty permalinks
	if !strings.Contains(conf, "try_files $uri $uri/ /index.php?$args;") {
		t.Errorf("missing pretty permalinks try_files directive")
	}

	// Must point to fastcgi port 9084
	if !strings.Contains(conf, "fastcgi_pass 127.0.0.1:9084;") {
		t.Errorf("missing or incorrect fastcgi_pass directive")
	}

	// Must contain client_max_body_size 256M
	if !strings.Contains(conf, "client_max_body_size 256M;") {
		t.Errorf("missing client_max_body_size 256M")
	}

	// Must not have ssl directive for HTTP site
	if strings.Contains(conf, "ssl_certificate") {
		t.Errorf("HTTP site should not contain ssl_certificate")
	}

	// Path directives should not contain Windows backslashes
	for _, line := range strings.Split(conf, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "root ") || strings.HasPrefix(trimmed, "access_log ") || strings.HasPrefix(trimmed, "error_log ") {
			if strings.Contains(trimmed, `\`) {
				t.Errorf("nginx path directive contains backslashes: %s", trimmed)
			}
		}
	}
}

func TestRenderSiteConfigHTTPS(t *testing.T) {
	conf, err := RenderSiteConfig("secure.local", "secure.local", true, 9083, 80, 443)
	if err != nil {
		t.Fatalf("RenderSiteConfig failed: %v", err)
	}

	// Must contain 301 redirect
	if !strings.Contains(conf, "return 301 https://$host$request_uri;") {
		t.Errorf("missing HTTPS 301 redirect")
	}

	// Must listen on 443 ssl with http2
	if !strings.Contains(conf, "listen 443 ssl;") || !strings.Contains(conf, "http2 on;") {
		t.Errorf("missing 443 ssl or http2 on directive")
	}

	// Must reference cert and key
	if !strings.Contains(conf, "ssl_certificate") || !strings.Contains(conf, "secure.local.pem") {
		t.Errorf("missing ssl_certificate for secure.local.pem")
	}
	if !strings.Contains(conf, "ssl_certificate_key") || !strings.Contains(conf, "secure.local-key.pem") {
		t.Errorf("missing ssl_certificate_key for secure.local-key.pem")
	}

	// Must specify modern TLS
	if !strings.Contains(conf, "TLSv1.2 TLSv1.3") {
		t.Errorf("missing modern TLS protocols")
	}

	// Must point to PHP 8.3 port 9083
	if !strings.Contains(conf, "fastcgi_pass 127.0.0.1:9083;") {
		t.Errorf("missing fastcgi_pass 127.0.0.1:9083")
	}
}
