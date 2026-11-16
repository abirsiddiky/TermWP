package cli

import (
	"testing"
)

func TestValidateSiteName(t *testing.T) {
	tests := []struct {
		name    string
		site    string
		wantErr bool
	}{
		{"valid .local", "mysite.local", false},
		{"valid .test", "wp-project.test", false},
		{"valid subdomain", "blog.demo.local", false},
		{"uppercase converted", "MySite.Local", false},
		{"valid numbers", "site123.local", false},
		{"no tld", "mysite", true},
		{"has spaces", "my site.local", true},
		{"invalid characters", "my$ite.local", true},
		{"empty string", "", true},
		{"reserved localhost", "localhost", true},
		{"reserved termwp", "termwp.local", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSiteName(tt.site)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSiteName(%q) error = %v, wantErr %v", tt.site, err, tt.wantErr)
			}
		})
	}
}

func TestParseAddArgs(t *testing.T) {
	defaultPHP := "8.4"

	tests := []struct {
		name        string
		args        []string
		wantSite    string
		wantPHP     string
		wantSSL     bool
		wantAddSSL  bool
		wantErr     bool
	}{
		{
			name:        "only site name (defaults)",
			args:        []string{"mysite.local"},
			wantSite:    "mysite.local",
			wantPHP:     "8.4",
			wantSSL:     false,
			wantAddSSL:  false,
			wantErr:     false,
		},
		{
			name:        "site + ssl",
			args:        []string{"mysite.local", "ssl"},
			wantSite:    "mysite.local",
			wantPHP:     "8.4",
			wantSSL:     true,
			wantAddSSL:  false,
			wantErr:     false,
		},
		{
			name:        "site + php8.3",
			args:        []string{"mysite.local", "php8.3"},
			wantSite:    "mysite.local",
			wantPHP:     "8.3",
			wantSSL:     false,
			wantAddSSL:  false,
			wantErr:     false,
		},
		{
			name:        "site + php8.3 + ssl",
			args:        []string{"mysite.local", "php8.3", "ssl"},
			wantSite:    "mysite.local",
			wantPHP:     "8.3",
			wantSSL:     true,
			wantAddSSL:  false,
			wantErr:     false,
		},
		{
			name:        "site + ssl + php8.3 (order reversed)",
			args:        []string{"mysite.local", "ssl", "php8.3"},
			wantSite:    "mysite.local",
			wantPHP:     "8.3",
			wantSSL:     true,
			wantAddSSL:  false,
			wantErr:     false,
		},
		{
			name:        "site + 8.5 (without php prefix)",
			args:        []string{"mysite.local", "8.5", "ssl"},
			wantSite:    "mysite.local",
			wantPHP:     "8.5",
			wantSSL:     true,
			wantAddSSL:  false,
			wantErr:     false,
		},
		{
			name:        "add ssl existing site",
			args:        []string{"ssl", "existing.local"},
			wantSite:    "existing.local",
			wantPHP:     "8.4",
			wantSSL:     true,
			wantAddSSL:  true,
			wantErr:     false,
		},
		{
			name:        "invalid php version",
			args:        []string{"mysite.local", "php7.4"},
			wantErr:     true,
		},
		{
			name:        "unknown extra token",
			args:        []string{"mysite.local", "ssl", "fast"},
			wantErr:     true,
		},
		{
			name:        "empty args",
			args:        []string{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ParseAddArgs(tt.args, defaultPHP)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAddArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err == nil {
				if res.SiteName != tt.wantSite {
					t.Errorf("SiteName = %v, want %v", res.SiteName, tt.wantSite)
				}
				if res.PHPVersion != tt.wantPHP {
					t.Errorf("PHPVersion = %v, want %v", res.PHPVersion, tt.wantPHP)
				}
				if res.SSL != tt.wantSSL {
					t.Errorf("SSL = %v, want %v", res.SSL, tt.wantSSL)
				}
				if res.IsAddExisting != tt.wantAddSSL {
					t.Errorf("IsAddExisting = %v, want %v", res.IsAddExisting, tt.wantAddSSL)
				}
			}
		})
	}
}

func TestParseCloneArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantSource string
		wantTarget string
		wantErr    bool
	}{
		{
			name:       "valid clone",
			args:       []string{"site.local", "to", "copy.local"},
			wantSource: "site.local",
			wantTarget: "copy.local",
			wantErr:    false,
		},
		{
			name:    "missing to keyword",
			args:    []string{"site.local", "copy.local"},
			wantErr: true,
		},
		{
			name:    "wrong keyword",
			args:    []string{"site.local", "into", "copy.local"},
			wantErr: true,
		},
		{
			name:    "same source and target",
			args:    []string{"site.local", "to", "site.local"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ParseCloneArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCloneArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err == nil {
				if res.SourceSite != tt.wantSource {
					t.Errorf("Source = %v, want %v", res.SourceSite, tt.wantSource)
				}
				if res.TargetSite != tt.wantTarget {
					t.Errorf("Target = %v, want %v", res.TargetSite, tt.wantTarget)
				}
			}
		})
	}
}
