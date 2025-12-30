package tester

import (
	"cloudflare-speedtest/pkg/models"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SpeedSample represents a speed measurement sample
type SpeedSample struct {
	Timestamp time.Time `json:"timestamp"`
	Speed     float64   `json:"speed"`    // Speed in Mbps
	Bytes     int64     `json:"bytes"`    // Bytes downloaded at this point
	Duration  float64   `json:"duration"` // Duration in seconds
}

// EnhancedSpeedTester provides advanced speed testing with sliding window algorithm
type EnhancedSpeedTester struct {
	client     *http.Client
	domain     string
	filePath   string
	timeout    time.Duration
	sampleRate time.Duration // How often to take samples
	windowSize int           // Number of samples in sliding window
	mu         sync.Mutex    // Protect concurrent access
}

// NewEnhanced creates a new enhanced speed tester
func NewEnhanced(timeout int) *EnhancedSpeedTester {
	return &EnhancedSpeedTester{
		timeout:    time.Duration(timeout) * time.Second,
		sampleRate: 500 * time.Millisecond, // Sample every 500ms
		windowSize: 10,                     // Keep last 10 samples
	}
}

// SetConfig sets the test configuration
func (est *EnhancedSpeedTester) SetConfig(domain, filePath string, downloadTime float64) {
	est.mu.Lock()
	defer est.mu.Unlock()

	est.domain = domain
	est.filePath = filePath
}

// TestDataCenterOnly tests only the data center information (for concurrent phase)
func (est *EnhancedSpeedTester) TestDataCenterOnly(ip string, useTLS bool, timeout int) (string, float64, error) {
	protocol := "http"
	port := "80"
	if useTLS {
		protocol = "https"
		port = "443"
	}

	// Handle IPv6 addresses
	if strings.Contains(ip, ":") {
		ip = "[" + ip + "]"
	}

	url := fmt.Sprintf("%s://%s:%s/cdn-cgi/trace", protocol, ip, port)

	start := time.Now()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", -1, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Host", est.domain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")

	client := est.createHTTPClient(useTLS, timeout)
	resp, err := client.Do(req)
	if err != nil {
		return "", -1, fmt.Errorf("failed to get datacenter info: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(start).Seconds() * 1000 // Convert to milliseconds

	if resp.StatusCode != 200 {
		return "", latency, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", latency, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response to find colo
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "colo=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) > 1 {
				datacenter := strings.TrimSpace(parts[1])
				return datacenter, latency, nil
			}
		}
	}

	return "", latency, fmt.Errorf("no datacenter info found")
}

// TestSpeedOnly tests only the download speed (for serial phase)
func (est *EnhancedSpeedTester) TestSpeedOnly(ip string, useTLS bool, timeout int, downloadTime float64) (*models.SpeedTestResult, error) {
	protocol := "http"
	port := "80"
	if useTLS {
		protocol = "https"
		port = "443"
	}

	// Handle IPv6 addresses
	if strings.Contains(ip, ":") {
		ip = "[" + ip + "]"
	}

	url := fmt.Sprintf("%s://%s:%s/%s", protocol, ip, port, est.filePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Host", est.domain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := est.createHTTPClient(useTLS, timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Perform speed test with sliding window algorithm
	result, err := est.performSpeedTest(resp.Body, downloadTime)
	if err != nil {
		return nil, fmt.Errorf("speed test failed: %w", err)
	}

	return result, nil
}

// performSpeedTest performs the actual speed test with sliding window algorithm
func (est *EnhancedSpeedTester) performSpeedTest(reader io.Reader, downloadTime float64) (*models.SpeedTestResult, error) {
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(downloadTime) * time.Second)

	totalBytes := int64(0)
	samples := make([]SpeedSample, 0)
	peakSpeed := 0.0
	chunkSize := 65536 // 64KB chunks
	lastSampleTime := startTime

	for {
		currentTime := time.Now()
		if currentTime.After(endTime) {
			break
		}

		// Read chunk
		chunk := make([]byte, chunkSize)
		n, err := reader.Read(chunk)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading data: %w", err)
		}

		if n > 0 {
			totalBytes += int64(n)
		}

		if err == io.EOF {
			break
		}

		// Take sample if enough time has passed
		if currentTime.Sub(lastSampleTime) >= est.sampleRate {
			elapsed := currentTime.Sub(startTime).Seconds()
			if elapsed > 0 {
				// Calculate current speed in Mbps
				currentSpeed := (float64(totalBytes) * 8) / (elapsed * 1000000)

				sample := SpeedSample{
					Timestamp: currentTime,
					Speed:     currentSpeed,
					Bytes:     totalBytes,
					Duration:  elapsed,
				}

				samples = append(samples, sample)

				// Maintain sliding window
				if len(samples) > est.windowSize {
					samples = samples[1:]
				}

				// Calculate windowed average speed
				windowedSpeed := est.calculateWindowedSpeed(samples)
				if windowedSpeed > peakSpeed {
					peakSpeed = windowedSpeed
				}

				lastSampleTime = currentTime
			}
		}
	}

	// Calculate final statistics
	totalDuration := time.Since(startTime).Seconds()
	finalSpeed := 0.0
	if totalDuration > 0 && totalBytes > 0 {
		finalSpeed = (float64(totalBytes) * 8) / (totalDuration * 1000000)
	}

	result := &models.SpeedTestResult{
		Status:    "已完成",
		Speed:     fmt.Sprintf("%.2f", finalSpeed),
		PeakSpeed: peakSpeed,
	}

	return result, nil
}

// calculateWindowedSpeed calculates the average speed using sliding window
func (est *EnhancedSpeedTester) calculateWindowedSpeed(samples []SpeedSample) float64 {
	if len(samples) == 0 {
		return 0
	}

	if len(samples) == 1 {
		return samples[0].Speed
	}

	// Calculate speed based on the difference between first and last sample in window
	first := samples[0]
	last := samples[len(samples)-1]

	bytesDiff := last.Bytes - first.Bytes
	timeDiff := last.Duration - first.Duration

	if timeDiff > 0 && bytesDiff > 0 {
		return (float64(bytesDiff) * 8) / (timeDiff * 1000000)
	}

	// Fallback to simple average
	total := 0.0
	for _, sample := range samples {
		total += sample.Speed
	}
	return total / float64(len(samples))
}

// createHTTPClient creates an HTTP client with proper configuration
func (est *EnhancedSpeedTester) createHTTPClient(useTLS bool, timeout int) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Duration(timeout) * time.Second,
		}).DialContext,
		DisableKeepAlives: false, // Enable keep-alives for better performance
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
	}

	// Configure TLS if needed
	if useTLS {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         est.domain,
		}
	}

	return &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}
}

// TestSpeedWithSamples tests speed and returns detailed samples (for analysis)
func (est *EnhancedSpeedTester) TestSpeedWithSamples(ip string, useTLS bool, timeout int, downloadTime float64) (*models.SpeedTestResult, []SpeedSample, error) {
	protocol := "http"
	port := "80"
	if useTLS {
		protocol = "https"
		port = "443"
	}

	// Handle IPv6 addresses
	if strings.Contains(ip, ":") {
		ip = "[" + ip + "]"
	}

	url := fmt.Sprintf("%s://%s:%s/%s", protocol, ip, port, est.filePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Host", est.domain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := est.createHTTPClient(useTLS, timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Perform speed test and collect samples
	result, samples, err := est.performSpeedTestWithSamples(resp.Body, downloadTime)
	if err != nil {
		return nil, nil, fmt.Errorf("speed test failed: %w", err)
	}

	return result, samples, nil
}

// performSpeedTestWithSamples performs speed test and returns all samples
func (est *EnhancedSpeedTester) performSpeedTestWithSamples(reader io.Reader, downloadTime float64) (*models.SpeedTestResult, []SpeedSample, error) {
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(downloadTime) * time.Second)

	totalBytes := int64(0)
	allSamples := make([]SpeedSample, 0)
	windowSamples := make([]SpeedSample, 0)
	peakSpeed := 0.0
	chunkSize := 65536
	lastSampleTime := startTime

	for {
		currentTime := time.Now()
		if currentTime.After(endTime) {
			break
		}

		chunk := make([]byte, chunkSize)
		n, err := reader.Read(chunk)
		if err != nil && err != io.EOF {
			return nil, nil, fmt.Errorf("error reading data: %w", err)
		}

		if n > 0 {
			totalBytes += int64(n)
		}

		if err == io.EOF {
			break
		}

		if currentTime.Sub(lastSampleTime) >= est.sampleRate {
			elapsed := currentTime.Sub(startTime).Seconds()
			if elapsed > 0 {
				currentSpeed := (float64(totalBytes) * 8) / (elapsed * 1000000)

				sample := SpeedSample{
					Timestamp: currentTime,
					Speed:     currentSpeed,
					Bytes:     totalBytes,
					Duration:  elapsed,
				}

				allSamples = append(allSamples, sample)
				windowSamples = append(windowSamples, sample)

				if len(windowSamples) > est.windowSize {
					windowSamples = windowSamples[1:]
				}

				windowedSpeed := est.calculateWindowedSpeed(windowSamples)
				if windowedSpeed > peakSpeed {
					peakSpeed = windowedSpeed
				}

				lastSampleTime = currentTime
			}
		}
	}

	totalDuration := time.Since(startTime).Seconds()
	finalSpeed := 0.0
	if totalDuration > 0 && totalBytes > 0 {
		finalSpeed = (float64(totalBytes) * 8) / (totalDuration * 1000000)
	}

	result := &models.SpeedTestResult{
		Status:    "已完成",
		Speed:     fmt.Sprintf("%.2f", finalSpeed),
		PeakSpeed: peakSpeed,
	}

	return result, allSamples, nil
}
