package tester

import (
	"cloudflare-speedtest/pkg/models"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// SpeedTester handles speed testing logic
type SpeedTester struct {
	timeout      int
	domain       string
	filePath     string
	downloadTime float64
}

// New creates a new speed tester
func New(timeout int) *SpeedTester {
	return &SpeedTester{
		timeout:      timeout,
		domain:       "example.com",
		filePath:     "",
		downloadTime: 10.0,
	}
}

// SetConfig sets the test configuration
func (st *SpeedTester) SetConfig(domain, filePath string, downloadTime float64) {
	st.domain = domain
	st.filePath = filePath
	st.downloadTime = downloadTime
}

// TestSpeed tests the speed of an IP
func (st *SpeedTester) TestSpeed(ip string, useTLS bool, timeout int, downloadTime float64) *models.SpeedTestResult {
	result := &models.SpeedTestResult{
		IP:         ip,
		Status:     "测试中",
		Latency:    "-",
		Speed:      "-",
		DataCenter: "",
		PeakSpeed:  0,
	}

	// First, test data center
	fmt.Printf("Step 1: Testing datacenter for IP %s\n", ip)
	dc := st.testDataCenter(ip, useTLS, timeout)
	result.DataCenter = dc

	// If no datacenter info, mark as invalid
	if dc == "" {
		fmt.Printf("No datacenter info found for %s, marking as invalid\n", ip)
		result.Status = "无效"
		result.Latency = "-"
		return result
	}

	// Test latency
	fmt.Printf("Step 2: Testing latency for IP %s\n", ip)
	latency := st.testLatency(ip, useTLS, timeout)
	if latency < 0 {
		fmt.Printf("Latency test failed for %s\n", ip)
		result.Status = "无效"
		result.Latency = "timeout"
		return result
	}
	result.Latency = fmt.Sprintf("%.2f", latency)

	// Test speed
	fmt.Printf("Step 3: Testing speed for IP %s\n", ip)
	speed, peakSpeed := st.testDownloadSpeed(ip, useTLS, timeout, downloadTime)
	if speed < 0 {
		fmt.Printf("Speed test failed for %s\n", ip)
		result.Status = "无效"
		result.Speed = "timeout"
		return result
	}

	result.Status = "已完成"
	result.Speed = fmt.Sprintf("%.2f", speed)
	result.PeakSpeed = peakSpeed

	fmt.Printf("Test completed for %s: DC=%s, Latency=%s, Speed=%s\n", ip, dc, result.Latency, result.Speed)
	return result
}

// createHTTPClient creates an HTTP client with proper headers
func (st *SpeedTester) createHTTPClient(useTLS bool, timeout int) *http.Client {
	return &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         st.domain, // Add SNI
			},
			DialContext: (&net.Dialer{
				Timeout: time.Duration(timeout) * time.Second,
			}).DialContext,
		},
	}
}

// testLatency tests the latency to an IP
func (st *SpeedTester) testLatency(ip string, useTLS bool, timeout int) float64 {
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
		fmt.Printf("Failed to create request for %s: %v\n", url, err)
		return -1
	}

	req.Header.Set("Host", st.domain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := st.createHTTPClient(useTLS, timeout)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to test latency for %s: %v\n", url, err)
		return -1
	}
	defer resp.Body.Close()

	latency := time.Since(start).Seconds() * 1000 // Convert to milliseconds
	return latency
}

// testDataCenter tests the data center information
func (st *SpeedTester) testDataCenter(ip string, useTLS bool, timeout int) string {
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
	fmt.Printf("Testing datacenter for %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Failed to create datacenter request: %v\n", err)
		return ""
	}

	req.Header.Set("Host", st.domain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	client := st.createHTTPClient(useTLS, timeout)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to get datacenter info from %s: %v\n", url, err)
		return ""
	}
	defer resp.Body.Close()

	fmt.Printf("Datacenter response status: %d\n", resp.StatusCode)

	if resp.StatusCode != 200 {
		// Try to read the response body for debugging
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Response body: %s\n", string(body))
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read datacenter response: %v\n", err)
		return ""
	}

	fmt.Printf("Datacenter response body:\n%s\n", string(body))

	// Parse response to find colo
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "colo=") {
			// Extract the datacenter code after "colo="
			parts := strings.SplitN(line, "=", 2)
			if len(parts) > 1 {
				datacenter := strings.TrimSpace(parts[1])
				fmt.Printf("Found datacenter: %s\n", datacenter)
				return datacenter
			}
		}
	}

	fmt.Printf("No datacenter info found in response\n")
	return ""
}

// testDownloadSpeed tests the download speed
func (st *SpeedTester) testDownloadSpeed(ip string, useTLS bool, timeout int, downloadTime float64) (float64, float64) {
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

	url := fmt.Sprintf("%s://%s:%s/%s", protocol, ip, port, st.filePath)
	fmt.Printf("Downloading from: %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Failed to create download request: %v\n", err)
		return -1, 0
	}

	req.Header.Set("Host", st.domain)

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         st.domain, // Add SNI
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to download from %s: %v\n", url, err)
		return -1, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Unexpected status code: %d\n", resp.StatusCode)
		return -1, 0
	}

	// Download data for specified duration
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(downloadTime) * time.Second)
	totalSize := int64(0)
	peakSpeed := 0.0
	speedSamples := make([]float64, 0)
	windowSize := 5
	chunkSize := 65536
	lastUpdateTime := startTime

	for {
		currentTime := time.Now()
		if currentTime.After(endTime) {
			break
		}

		chunk := make([]byte, chunkSize)
		n, err := resp.Body.Read(chunk)
		if err != nil && err != io.EOF {
			fmt.Printf("Error reading response body: %v\n", err)
			break
		}

		if n > 0 {
			totalSize += int64(n)
		}

		if err == io.EOF {
			break
		}

		// Update every 0.5 seconds
		if currentTime.Sub(lastUpdateTime) >= 500*time.Millisecond {
			elapsed := currentTime.Sub(startTime).Seconds()
			if elapsed > 0 {
				// Calculate speed in Mbps
				currentSpeed := (float64(totalSize) / 1024 / elapsed) / 128
				speedSamples = append(speedSamples, currentSpeed)

				if len(speedSamples) > windowSize {
					speedSamples = speedSamples[1:]
				}

				// Calculate average speed in window
				avgSpeed := 0.0
				for _, s := range speedSamples {
					avgSpeed += s
				}
				avgSpeed /= float64(len(speedSamples))

				if avgSpeed > peakSpeed {
					peakSpeed = avgSpeed
				}
			}
			lastUpdateTime = currentTime
		}
	}

	// Calculate final speed
	totalDuration := time.Since(startTime).Seconds()
	finalSpeed := 0.0
	if totalDuration > 0 && totalSize > 0 {
		finalSpeed = (float64(totalSize) / 1024 / totalDuration) / 128
	}

	fmt.Printf("Downloaded %d bytes in %.2f seconds, speed: %.2f Mbps, peak: %.2f Mbps\n",
		totalSize, totalDuration, finalSpeed, peakSpeed)

	return finalSpeed, peakSpeed
}
