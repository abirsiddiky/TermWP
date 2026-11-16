package runtime

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"termwp/internal/config"
	"termwp/internal/ui"
)

// Downloader manages portable archive downloads and extraction.
type Downloader struct {
	client  *http.Client
	tracker *ui.StepTracker
}

// NewDownloader creates an initialized Downloader.
func NewDownloader(tracker *ui.StepTracker) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Minute, // Large archives (MariaDB / PHP) may take time
		},
		tracker: tracker,
	}
}

// DownloadFile downloads a URL into destPath with resume capability and progress updates.
func (d *Downloader) DownloadFile(url, destPath string) error {
	partPath := destPath + ".part"

	var existingSize int64
	if stat, err := os.Stat(partPath); err == nil {
		existingSize = stat.Size()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) TermWP/1.0")

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("download connection failed: %w", err)
	}
	defer resp.Body.Close()

	var out *os.File
	var totalSize int64

	switch resp.StatusCode {
	case http.StatusOK:
		// Check Content-Type: if server returned text/html for a zip/tar archive, reject it as an error page!
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.HasPrefix(contentType, "text/html") && (strings.HasSuffix(destPath, ".zip") || strings.HasSuffix(destPath, ".tar.gz") || strings.HasSuffix(destPath, ".tgz")) {
			return fmt.Errorf("server returned HTML page instead of archive (resource likely not found or invalid URL: %s)", url)
		}
		// Server doesn't support range or fresh download
		existingSize = 0
		totalSize = resp.ContentLength
		out, err = os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	case http.StatusPartialContent:
		// Resuming download
		totalSize = existingSize + resp.ContentLength
		out, err = os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	default:
		// If windows.php.net returns 404, automatically try the archives directory
		if resp.StatusCode == http.StatusNotFound && strings.Contains(url, "windows.php.net/downloads/") && !strings.Contains(url, "/archives/") {
			var fallbackURL string
			if strings.Contains(url, "/releases/") {
				fallbackURL = strings.Replace(url, "/downloads/releases/", "/downloads/releases/archives/", 1)
			} else if strings.Contains(url, "/qa/") {
				fallbackURL = strings.Replace(url, "/downloads/qa/", "/downloads/qa/archives/", 1)
			}
			if fallbackURL != "" {
				return d.DownloadFile(fallbackURL, destPath)
			}
		}
		return fmt.Errorf("server responded with status %d: %s", resp.StatusCode, resp.Status)
	}

	if err != nil {
		return fmt.Errorf("failed to open destination file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	buffer := make([]byte, 64*1024)
	downloaded := existingSize
	lastUpdate := time.Now()

	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("failed writing downloaded data: %w", writeErr)
			}
			downloaded += int64(n)

			if time.Since(lastUpdate) > 500*time.Millisecond && totalSize > 0 {
				percent := float64(downloaded) / float64(totalSize) * 100.0
				if d.tracker != nil {
					_ = percent // progress updated
				}
				lastUpdate = time.Now()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("download stream error: %w", readErr)
		}
	}

	// Rename .part to final destination
	_ = out.Close()
	_ = os.Remove(destPath)
	if err := os.Rename(partPath, destPath); err != nil {
		return fmt.Errorf("failed to finalize downloaded file: %w", err)
	}

	return nil
}

// VerifyChecksum verifies that a file matches the expected SHA256 checksum.
func VerifyChecksum(filePath, expectedSha256 string) error {
	if expectedSha256 == "" {
		return nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	calculated := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(calculated, expectedSha256) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSha256, calculated)
	}
	return nil
}

// ExtractZip extracts a zip file to destDir, optionally stripping leading path segments.
func ExtractZip(zipPath, destDir string, stripDirs int) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	destClean := filepath.Clean(destDir)

	for _, file := range reader.File {
		parts := strings.Split(filepath.ToSlash(file.Name), "/")
		if len(parts) <= stripDirs {
			continue
		}
		relPath := filepath.Join(parts[stripDirs:]...)
		targetPath := filepath.Join(destClean, relPath)

		// Prevent Zip Slip vulnerability
		if !strings.HasPrefix(filepath.Clean(targetPath), destClean) {
			return fmt.Errorf("illegal file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(dstFile, rc)
		rc.Close()
		dstFile.Close()
		if copyErr != nil {
			return copyErr
		}
	}

	return nil
}

// ExtractTarGz extracts a tar.gz archive to destDir, stripping leading segments.
func ExtractTarGz(tarGzPath, destDir string, stripDirs int) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	destClean := filepath.Clean(destDir)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reading error: %w", err)
		}

		parts := strings.Split(filepath.ToSlash(header.Name), "/")
		if len(parts) <= stripDirs {
			continue
		}
		relPath := filepath.Join(parts[stripDirs:]...)
		if relPath == "" || relPath == "." {
			continue
		}

		targetPath := filepath.Join(destClean, relPath)
		if !strings.HasPrefix(filepath.Clean(targetPath), destClean) {
			return fmt.Errorf("illegal file path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			mode := header.FileInfo().Mode()
			if mode == 0 {
				mode = 0644
			}
			dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(dstFile, tarReader); err != nil {
				dstFile.Close()
				return err
			}
			dstFile.Close()
		}
	}

	return nil
}

// FetchAndInstall handles downloading into cache, verifying checksum, and extracting to destDir.
func (d *Downloader) FetchAndInstall(src *config.DownloadSource, destDir string) error {
	cachePath := filepath.Join(config.CacheDir(), src.Filename)

	if d.tracker != nil {
		d.tracker.Start(fmt.Sprintf("Downloading %s (%s)", src.Name, src.Version))
	}

	// If already in cache and checksum valid, skip re-download
	downloadNeeded := true
	if _, err := os.Stat(cachePath); err == nil {
		if err := VerifyChecksum(cachePath, src.SHA256); err == nil {
			downloadNeeded = false
		}
	}

	if downloadNeeded {
		if err := d.DownloadFile(src.URL, cachePath); err != nil {
			if d.tracker != nil {
				d.tracker.Fail(fmt.Sprintf("Download failed for %s", src.Name))
			}
			return err
		}
	}

	if err := VerifyChecksum(cachePath, src.SHA256); err != nil {
		_ = os.Remove(cachePath)
		if d.tracker != nil {
			d.tracker.Fail(fmt.Sprintf("Checksum verification failed for %s", src.Name))
		}
		return err
	}

	if d.tracker != nil {
		d.tracker.Start(fmt.Sprintf("Extracting %s", src.Name))
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	switch src.ArchiveType {
	case config.ArchiveZip:
		if err := ExtractZip(cachePath, destDir, src.StripDirs); err != nil {
			_ = os.Remove(cachePath) // Remove corrupted cache so retry doesn't reuse it
			if d.tracker != nil {
				d.tracker.Fail(fmt.Sprintf("Extraction failed for %s", src.Name))
			}
			return err
		}
	case config.ArchiveTarGz:
		if err := ExtractTarGz(cachePath, destDir, src.StripDirs); err != nil {
			_ = os.Remove(cachePath) // Remove corrupted cache so retry doesn't reuse it
			if d.tracker != nil {
				d.tracker.Fail(fmt.Sprintf("Extraction failed for %s", src.Name))
			}
			return err
		}
	case config.ArchiveBinary:
		targetBin := filepath.Join(destDir, src.Filename)
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetBin, data, 0755); err != nil {
			return err
		}
	}

	if d.tracker != nil {
		d.tracker.Success(fmt.Sprintf("Installed %s %s", src.Name, src.Version))
	}

	return nil
}
