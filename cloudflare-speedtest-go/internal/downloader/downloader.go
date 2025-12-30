package downloader

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Downloader handles file downloads with enhanced features
type Downloader struct {
	timeout      time.Duration
	maxRetries   int
	retryDelay   time.Duration
	cacheDir     string
	progressChan chan ProgressInfo
	mu           sync.RWMutex
}

// FileInfo contains information about a file to download
type FileInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Size     int64  `json:"size,omitempty"`
	Checksum string `json:"checksum,omitempty"`
	HashType string `json:"hash_type,omitempty"` // "md5", "sha256"
}

// ProgressInfo represents download progress information
type ProgressInfo struct {
	FileName        string        `json:"file_name"`
	TotalBytes      int64         `json:"total_bytes"`
	DownloadedBytes int64         `json:"downloaded_bytes"`
	Percentage      float64       `json:"percentage"`
	Speed           float64       `json:"speed_mbps"`
	ETA             time.Duration `json:"eta"`
	Status          string        `json:"status"` // "downloading", "completed", "failed", "verifying"
	Error           error         `json:"error,omitempty"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time,omitempty"`
}

// DownloadResult represents the result of a download operation
type DownloadResult struct {
	FileInfo     FileInfo      `json:"file_info"`
	Success      bool          `json:"success"`
	Error        error         `json:"error,omitempty"`
	Attempts     int           `json:"attempts"`
	Duration     time.Duration `json:"duration"`
	BytesWritten int64         `json:"bytes_written"`
	Verified     bool          `json:"verified"`
}

// CacheInfo represents cached file information
type CacheInfo struct {
	FilePath     string    `json:"file_path"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	HashType     string    `json:"hash_type"`
}

// New creates a new enhanced downloader
func New() *Downloader {
	return &Downloader{
		timeout:      30 * time.Second,
		maxRetries:   3,
		retryDelay:   2 * time.Second,
		progressChan: make(chan ProgressInfo, 100),
	}
}

// SetCacheDir sets the cache directory for downloaded files
func (d *Downloader) SetCacheDir(cacheDir string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	d.cacheDir = cacheDir
	return nil
}

// SetRetryPolicy sets the retry policy
func (d *Downloader) SetRetryPolicy(maxRetries int, retryDelay time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.maxRetries = maxRetries
	d.retryDelay = retryDelay
}

// GetProgressChannel returns the progress channel
func (d *Downloader) GetProgressChannel() <-chan ProgressInfo {
	return d.progressChan
}

// Download downloads a file with enhanced features
func (d *Downloader) Download(url, filePath string) error {
	fileInfo := FileInfo{
		Name: filepath.Base(filePath),
		URL:  url,
	}

	result := d.DownloadWithProgress(fileInfo, filePath)
	return result.Error
}

// DownloadWithProgress downloads a file with progress tracking
func (d *Downloader) DownloadWithProgress(fileInfo FileInfo, outputPath string) *DownloadResult {
	result := &DownloadResult{
		FileInfo: fileInfo,
		Success:  false,
	}

	startTime := time.Now()

	// Check cache first
	if d.cacheDir != "" {
		if cached, err := d.checkCache(fileInfo); err == nil && cached {
			cachedPath := filepath.Join(d.cacheDir, fileInfo.Name)
			if err := d.copyFile(cachedPath, outputPath); err == nil {
				result.Success = true
				result.Duration = time.Since(startTime)
				result.BytesWritten = fileInfo.Size
				result.Verified = true

				d.sendProgress(ProgressInfo{
					FileName:   fileInfo.Name,
					Status:     "completed",
					Percentage: 100,
					StartTime:  startTime,
					EndTime:    time.Now(),
				})

				return result
			}
		}
	}

	// Attempt download with retries
	for attempt := 1; attempt <= d.maxRetries; attempt++ {
		result.Attempts = attempt

		d.sendProgress(ProgressInfo{
			FileName:  fileInfo.Name,
			Status:    "downloading",
			StartTime: startTime,
		})

		err := d.downloadWithProgressTracking(fileInfo, outputPath)
		if err == nil {
			// Verify file integrity if checksum is provided
			if fileInfo.Checksum != "" {
				d.sendProgress(ProgressInfo{
					FileName: fileInfo.Name,
					Status:   "verifying",
				})

				if verified, verifyErr := d.verifyFileIntegrity(outputPath, fileInfo); verifyErr == nil && verified {
					result.Verified = true
				} else {
					err = fmt.Errorf("file verification failed: %w", verifyErr)
				}
			}

			if err == nil {
				// Cache the file if cache directory is set
				if d.cacheDir != "" {
					d.cacheFile(fileInfo, outputPath)
				}

				result.Success = true
				result.Duration = time.Since(startTime)

				// Get file size
				if stat, statErr := os.Stat(outputPath); statErr == nil {
					result.BytesWritten = stat.Size()
				}

				d.sendProgress(ProgressInfo{
					FileName:   fileInfo.Name,
					Status:     "completed",
					Percentage: 100,
					StartTime:  startTime,
					EndTime:    time.Now(),
				})

				return result
			}
		}

		result.Error = err

		d.sendProgress(ProgressInfo{
			FileName:  fileInfo.Name,
			Status:    "failed",
			Error:     err,
			StartTime: startTime,
			EndTime:   time.Now(),
		})

		// Wait before retry (except for last attempt)
		if attempt < d.maxRetries {
			time.Sleep(d.retryDelay)
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// downloadWithProgressTracking downloads a file with real-time progress tracking
func (d *Downloader) downloadWithProgressTracking(fileInfo FileInfo, outputPath string) error {
	client := &http.Client{
		Timeout: d.timeout,
	}

	resp, err := client.Get(fileInfo.URL)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get content length
	contentLength := resp.ContentLength
	if contentLength <= 0 && fileInfo.Size > 0 {
		contentLength = fileInfo.Size
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Track progress
	var downloaded int64
	startTime := time.Now()
	lastProgressTime := startTime

	// Create a progress reader
	progressReader := &progressReader{
		reader: resp.Body,
		onProgress: func(bytesRead int64) {
			downloaded += bytesRead

			now := time.Now()
			if now.Sub(lastProgressTime) >= 500*time.Millisecond || downloaded == contentLength {
				elapsed := now.Sub(startTime).Seconds()

				var percentage float64
				var eta time.Duration
				var speed float64

				if contentLength > 0 {
					percentage = float64(downloaded) / float64(contentLength) * 100
					if elapsed > 0 {
						speed = float64(downloaded) / elapsed / 1024 / 1024 * 8 // Mbps
						if downloaded > 0 {
							remainingBytes := contentLength - downloaded
							eta = time.Duration(float64(remainingBytes)/(float64(downloaded)/elapsed)) * time.Second
						}
					}
				}

				d.sendProgress(ProgressInfo{
					FileName:        fileInfo.Name,
					TotalBytes:      contentLength,
					DownloadedBytes: downloaded,
					Percentage:      percentage,
					Speed:           speed,
					ETA:             eta,
					Status:          "downloading",
					StartTime:       startTime,
				})

				lastProgressTime = now
			}
		},
	}

	// Copy with progress tracking
	_, err = io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// verifyFileIntegrity verifies file integrity using checksum
func (d *Downloader) verifyFileIntegrity(filePath string, fileInfo FileInfo) (bool, error) {
	if fileInfo.Checksum == "" {
		return true, nil // No checksum to verify
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file for verification: %w", err)
	}
	defer file.Close()

	var hasher hash.Hash
	switch fileInfo.HashType {
	case "sha256":
		hasher = sha256.New()
	case "md5":
		hasher = md5.New()
	default:
		hasher = sha256.New() // Default to SHA256
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return false, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	calculatedChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	return calculatedChecksum == fileInfo.Checksum, nil
}

// checkCache checks if a file exists in cache and is valid
func (d *Downloader) checkCache(fileInfo FileInfo) (bool, error) {
	if d.cacheDir == "" {
		return false, fmt.Errorf("cache directory not set")
	}

	cachedPath := filepath.Join(d.cacheDir, fileInfo.Name)

	// Check if cached file exists
	stat, err := os.Stat(cachedPath)
	if err != nil {
		return false, err
	}

	// Check size if provided
	if fileInfo.Size > 0 && stat.Size() != fileInfo.Size {
		return false, fmt.Errorf("cached file size mismatch")
	}

	// Verify checksum if provided
	if fileInfo.Checksum != "" {
		verified, err := d.verifyFileIntegrity(cachedPath, fileInfo)
		if err != nil || !verified {
			return false, fmt.Errorf("cached file verification failed")
		}
	}

	return true, nil
}

// cacheFile copies a file to cache directory
func (d *Downloader) cacheFile(fileInfo FileInfo, sourcePath string) error {
	if d.cacheDir == "" {
		return nil // No cache directory set
	}

	cachedPath := filepath.Join(d.cacheDir, fileInfo.Name)
	return d.copyFile(sourcePath, cachedPath)
}

// copyFile copies a file from source to destination
func (d *Downloader) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// sendProgress sends progress information to the channel
func (d *Downloader) sendProgress(progress ProgressInfo) {
	select {
	case d.progressChan <- progress:
	default:
		// Channel is full, skip this progress update
	}
}

// DownloadFiles downloads multiple files with progress tracking
func (d *Downloader) DownloadFiles(files []FileInfo, outputDir string) error {
	var results []*DownloadResult

	for _, file := range files {
		filePath := filepath.Join(outputDir, file.Name)
		fmt.Printf("Downloading %s to %s...\n", file.Name, filePath)

		result := d.DownloadWithProgress(file, filePath)
		results = append(results, result)

		if result.Success {
			fmt.Printf("Downloaded %s successfully (attempts: %d, duration: %v, verified: %t)\n",
				file.Name, result.Attempts, result.Duration, result.Verified)
		} else {
			fmt.Printf("Failed to download %s after %d attempts: %v\n",
				file.Name, result.Attempts, result.Error)
			return result.Error
		}
	}

	return nil
}

// progressReader wraps an io.Reader to track progress
type progressReader struct {
	reader     io.Reader
	onProgress func(int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.onProgress != nil {
		pr.onProgress(int64(n))
	}
	return n, err
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
