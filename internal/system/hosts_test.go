package system

import (
	"strings"
	"testing"
)

const sampleHostsContent = `# Copyright (c) 1993-2009 Microsoft Corp.
#
# This is a sample HOSTS file used by Microsoft TCP/IP for Windows.
127.0.0.1       localhost
::1             localhost

# Custom entry from user
192.168.1.50    printer.office.local
`

func TestHostsBlockEditing(t *testing.T) {
	// 1. Initial parse of clean hosts file
	before, inside, after, hasBlock := ParseHostsFile(sampleHostsContent)
	if hasBlock {
		t.Errorf("expected no block in initial hosts file")
	}
	if len(inside) != 0 {
		t.Errorf("expected inside to be empty, got %d lines", len(inside))
	}

	// 2. Add domain "mysite.local"
	domains := ParseManagedDomains(inside)
	domains["mysite.local"] = "127.0.0.1"

	content1 := BuildHostsContent(before, after, domains)
	if !strings.Contains(content1, BlockBeginMarker) || !strings.Contains(content1, BlockEndMarker) {
		t.Errorf("expected block markers in output")
	}
	if !strings.Contains(content1, "127.0.0.1        mysite.local") && !strings.Contains(content1, "127.0.0.1       mysite.local") && !strings.Contains(content1, "mysite.local") {
		t.Errorf("expected mysite.local inside content")
	}
	if !strings.Contains(content1, "printer.office.local") {
		t.Errorf("existing user entry 'printer.office.local' was removed or corrupted")
	}

	// 3. Add second domain "demo.test"
	before2, inside2, after2, hasBlock2 := ParseHostsFile(content1)
	if !hasBlock2 {
		t.Errorf("expected hasBlock2 to be true")
	}
	domains2 := ParseManagedDomains(inside2)
	domains2["demo.test"] = "127.0.0.1"

	content2 := BuildHostsContent(before2, after2, domains2)
	if !strings.Contains(content2, "demo.test") || !strings.Contains(content2, "mysite.local") {
		t.Errorf("expected both domains to exist")
	}

	// 4. Remove "mysite.local"
	before3, inside3, after3, _ := ParseHostsFile(content2)
	domains3 := ParseManagedDomains(inside3)
	delete(domains3, "mysite.local")

	content3 := BuildHostsContent(before3, after3, domains3)
	if strings.Contains(content3, "mysite.local") {
		t.Errorf("expected mysite.local to be removed")
	}
	if !strings.Contains(content3, "demo.test") {
		t.Errorf("expected demo.test to remain")
	}

	// 5. Remove all managed domains -> block should be cleaned up
	delete(domains3, "demo.test")
	content4 := BuildHostsContent(before3, after3, domains3)
	if strings.Contains(content4, BlockBeginMarker) {
		t.Errorf("expected empty block to be removed completely")
	}
	if !strings.Contains(content4, "printer.office.local") {
		t.Errorf("custom user entry must be preserved")
	}
}
