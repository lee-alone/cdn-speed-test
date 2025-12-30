package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader handles file downloads
type Downloader struct {
	timeout time.Duration
}

// FileInfo contains information about a file to download
type FileInfo struct {
	Name string
	URL  string
}

// New creates a new downloader
func New() *Downloader {
	return &Downloader{
		timeout: 30 * time.Second,
	}
}

// Download downloads a file from URL
func (d *Downloader) Download(url, filePath string) error {
	client := &http.Client{
		Timeout: d.timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create file directly (no subdirectory)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy content
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// DownloadFiles downloads multiple files
func (d *Downloader) DownloadFiles(files []FileInfo, outputDir string) error {
	for _, file := range files {
		filePath := filepath.Join(outputDir, file.Name)
		fmt.Printf("Downloading %s to %s...\n", file.Name, filePath)
		if err := d.Download(file.URL, filePath); err != nil {
			return fmt.Errorf("failed to download %s: %w", file.Name, err)
		}
		fmt.Printf("Downloaded %s successfully\n", file.Name)
	}
	return nil
}

// GetDefaultFiles returns the default files to download
func GetDefaultFiles() []FileInfo {
	return []FileInfo{
		{
			Name: "ips-v4.txt",
			URL:  "https://www.baipiao.eu.org/cloudflare/ips-v4",
		},
		{
			Name: "ips-v6.txt",
			URL:  "https://www.baipiao.eu.org/cloudflare/ips-v6",
		},
		{
			Name: "colo.txt",
			URL:  "https://www.baipiao.eu.org/cloudflare/colo",
		},
		{
			Name: "url.txt",
			URL:  "https://www.baipiao.eu.org/cloudflare/url",
		},
	}
}

// GetFilesFromConfig returns files to download from configuration
func GetFilesFromConfig(urls map[string]string) []FileInfo {
	var files []FileInfo
	for name, url := range urls {
		files = append(files, FileInfo{
			Name: name,
			URL:  url,
		})
	}
	return files
}

// FileExists checks if a file exists
func FileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

// GetMissingFiles returns list of missing files
func GetMissingFiles(outputDir string) []FileInfo {
	allFiles := GetDefaultFiles()
	var missing []FileInfo

	for _, file := range allFiles {
		filePath := filepath.Join(outputDir, file.Name)
		if !FileExists(filePath) {
			missing = append(missing, file)
		}
	}

	return missing
}
