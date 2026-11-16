package runtime

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func createTestZip(t *testing.T, files map[string]string) string {
	tmpFile, err := os.CreateTemp("", "test_archive_*.zip")
	if err != nil {
		t.Fatalf("failed to create temp zip: %v", err)
	}
	defer tmpFile.Close()

	w := zip.NewWriter(tmpFile)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content: %v", err)
		}
	}
	_ = w.Close()
	return tmpFile.Name()
}

func createTestTarGz(t *testing.T, files map[string]string) string {
	tmpFile, err := os.CreateTemp("", "test_archive_*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create temp tar.gz: %v", err)
	}
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}
	_ = tw.Close()
	_ = gw.Close()
	return tmpFile.Name()
}

func TestExtractZipWithStripDirs(t *testing.T) {
	files := map[string]string{
		"wordpress/index.php":      "<?php // test index",
		"wordpress/wp-config.php":  "<?php // test config",
		"wordpress/wp-content/foo": "bar",
	}

	zipPath := createTestZip(t, files)
	defer os.Remove(zipPath)

	destDir, err := os.MkdirTemp("", "extract_dest_*")
	if err != nil {
		t.Fatalf("failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Strip 1 directory segment ("wordpress/")
	if err := ExtractZip(zipPath, destDir, 1); err != nil {
		t.Fatalf("ExtractZip failed: %v", err)
	}

	// Verify index.php is at destDir root, not destDir/wordpress/
	indexPath := filepath.Join(destDir, "index.php")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read extracted index.php: %v", err)
	}
	if !bytes.Equal(content, []byte("<?php // test index")) {
		t.Errorf("extracted content mismatch: %s", string(content))
	}

	// Verify nested file
	fooPath := filepath.Join(destDir, "wp-content", "foo")
	fooContent, err := os.ReadFile(fooPath)
	if err != nil {
		t.Fatalf("failed to read extracted nested file: %v", err)
	}
	if !bytes.Equal(fooContent, []byte("bar")) {
		t.Errorf("nested content mismatch: %s", string(fooContent))
	}
}

func TestExtractTarGzWithStripDirs(t *testing.T) {
	files := map[string]string{
		"wordpress/index.php": "<?php // tar index",
	}

	tarPath := createTestTarGz(t, files)
	defer os.Remove(tarPath)

	destDir, err := os.MkdirTemp("", "extract_tar_dest_*")
	if err != nil {
		t.Fatalf("failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	if err := ExtractTarGz(tarPath, destDir, 1); err != nil {
		t.Fatalf("ExtractTarGz failed: %v", err)
	}

	indexPath := filepath.Join(destDir, "index.php")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read extracted tar index.php: %v", err)
	}
	if !bytes.Equal(content, []byte("<?php // tar index")) {
		t.Errorf("extracted tar content mismatch: %s", string(content))
	}
}
