package server

import (
	"cloudflare-speedtest/internal/tester"
	"cloudflare-speedtest/pkg/models"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// runTest runs the two-phase speed test
func (s *Server) runTest() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Test panic: %v\n", r)
		}
		s.testMu.Lock()
		s.testing = false
		s.testMu.Unlock()

		fmt.Println("Test execution completed, testing flag set to false")
	}()

	fmt.Println("Starting runTest function")

	if !s.urlManager.HasURLs() {
		fmt.Println("Loading URLs...")
		if err := s.urlManager.LoadURLs(); err != nil {
			fmt.Printf("Failed to load URLs: %v\n", err)
			return
		}
	}

	if !s.coloManager.HasColos() {
		fmt.Println("Loading data centers...")
		if err := s.coloManager.LoadColos(); err != nil {
			fmt.Printf("Failed to load data centers: %v\n", err)
			return
		}
	}

	urls := s.urlManager.GetURLs()
	if len(urls) == 0 {
		fmt.Println("No URLs available for testing")
		return
	}

	url := urls[0]
	fmt.Printf("Using URL: %s\n", url)

	url, _ = strings.CutPrefix(url, "http://")
	url, _ = strings.CutPrefix(url, "https://")

	parts := strings.SplitN(url, "/", 2)
	domain := parts[0]
	filePath := ""
	if len(parts) > 1 {
		filePath = parts[1]
	}

	fmt.Printf("Domain: %s, FilePath: %s\n", domain, filePath)
	fmt.Printf("Data center filter mode: %s\n", s.coloManager.GetFilterMode())
	if s.coloManager.GetFilterMode() == "selected" {
		fmt.Printf("Selected data centers: %v\n", s.coloManager.GetSelectedDataCenters())
	}

	expectedServers := s.config.Test.ExpectedServers
	expectedBandwidth := s.config.Test.Bandwidth
	fmt.Printf("Target: %d servers with speed >= %.2f Mbps\n", expectedServers, expectedBandwidth)

	totalIPsTested := 0
	batchNumber := 0

	for {
		s.testMu.RLock()
		if !s.testing {
			s.testMu.RUnlock()
			fmt.Println("Testing stopped by user")
			return
		}
		s.testMu.RUnlock()

		batchNumber++
		fmt.Printf("\n=== Batch %d: Reading IPs ===\n", batchNumber)

		fmt.Printf("Reading IPs from %s...\n", s.config.Test.IPType)
		ips, err := s.ipReader.ReadIPs(s.config.Test.IPType, 100)
		if err != nil {
			fmt.Printf("Failed to read IPs: %v\n", err)
			break
		}

		if len(ips) == 0 {
			fmt.Println("No more IPs available for testing")
			break
		}

		totalIPsTested += len(ips)
		fmt.Printf("Batch %d: Read %d IPs (total tested so far: %d)\n", batchNumber, len(ips), totalIPsTested)

		s.resultManager.SetTotal(totalIPsTested)

		fmt.Printf("\n=== Batch %d - Phase 1: Concurrent Datacenter Detection ===\n", batchNumber)
		validIPs := s.runDataCenterPhase(ips, domain, filePath)

		if len(validIPs) == 0 {
			fmt.Printf("Batch %d: No valid IPs found after datacenter filtering\n", batchNumber)
			continue
		}

		fmt.Printf("Batch %d - Phase 1 completed: %d valid IPs found\n", batchNumber, len(validIPs))

		fmt.Printf("\n=== Batch %d - Phase 2: Serial Speed Testing ===\n", batchNumber)
		s.runSpeedTestPhase(validIPs, domain, filePath)

		qualifiedResults := s.resultManager.GetQualifiedResults()
		qualifiedCount := 0

		for _, r := range qualifiedResults {
			speedVal, err := strconv.ParseFloat(r.Speed, 64)
			if err == nil && speedVal >= expectedBandwidth {
				qualifiedCount++
			}
		}

		fmt.Printf("\nCurrent progress: %d servers with speed >= %.2f Mbps (need %d)\n",
			qualifiedCount, expectedBandwidth, expectedServers)

		if qualifiedCount >= expectedServers {
			fmt.Printf("\n✓ Found %d qualified servers (speed >= %.2f Mbps). Expected: %d. Test completed.\n",
				qualifiedCount, expectedBandwidth, expectedServers)

			stats := s.resultManager.GetStats()
			s.resultManager.SetTotal(stats.Completed)

			break
		}

		fmt.Printf("Batch %d completed. Need more qualified servers, reading next batch...\n", batchNumber)
	}

	fmt.Println("Two-phase speed test completed")
}

// runDataCenterPhase runs the concurrent datacenter detection phase
func (s *Server) runDataCenterPhase(ips []string, domain, filePath string) []string {
	fmt.Printf("Starting datacenter detection for %d IPs using %d workers\n", len(ips), s.config.Advanced.ConcurrentWorkers)

	enhancedTester := tester.NewEnhanced(s.config.Test.Timeout)
	enhancedTester.SetConfig(domain, filePath, float64(s.config.Test.DownloadTime))

	type DataCenterResult struct {
		IP         string
		DataCenter string
		Latency    float64
		Error      error
	}

	resultChan := make(chan DataCenterResult, len(ips))
	semaphore := make(chan struct{}, s.config.Advanced.ConcurrentWorkers)

	var wg sync.WaitGroup
	for _, ip := range ips {
		s.testMu.RLock()
		if !s.testing {
			s.testMu.RUnlock()
			break
		}
		s.testMu.RUnlock()

		wg.Add(1)
		go func(testIP string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Printf("Testing datacenter for IP: %s\n", testIP)
			datacenter, latency, err := enhancedTester.TestDataCenterOnly(testIP, s.config.Test.UseTLS, s.config.Test.Timeout)

			resultChan <- DataCenterResult{
				IP:         testIP,
				DataCenter: datacenter,
				Latency:    latency,
				Error:      err,
			}
		}(ip)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	validIPs := make([]string, 0)
	testedCount := 0
	filteredCount := 0

	for result := range resultChan {
		testedCount++

		if result.Error != nil {
			fmt.Printf("Datacenter test failed for %s: %v\n", result.IP, result.Error)
			continue
		}

		if result.DataCenter == "" {
			fmt.Printf("No datacenter info found for %s\n", result.IP)
			continue
		}

		if !s.coloManager.FilterByDataCenter(result.DataCenter) {
			fmt.Printf("IP %s filtered out (datacenter: %s not in selected list)\n", result.IP, result.DataCenter)
			filteredCount++
			continue
		}

		fmt.Printf("Valid IP found: %s (datacenter: %s, latency: %.2f ms)\n", result.IP, result.DataCenter, result.Latency)
		validIPs = append(validIPs, result.IP)
	}

	fmt.Printf("Datacenter phase summary: Tested=%d, Filtered=%d, Valid=%d\n", testedCount, filteredCount, len(validIPs))

	if len(validIPs) == 0 && filteredCount > 0 {
		fmt.Printf("WARNING: All %d IPs were filtered out due to datacenter selection. No IPs match the selected datacenters.\n", filteredCount)
	}

	return validIPs
}

// runSpeedTestPhase runs the serial speed testing phase
func (s *Server) runSpeedTestPhase(validIPs []string, domain, filePath string) {
	fmt.Printf("Starting serial speed testing for %d valid IPs\n", len(validIPs))

	enhancedTester := tester.NewEnhanced(s.config.Test.Timeout)
	enhancedTester.SetConfig(domain, filePath, float64(s.config.Test.DownloadTime))

	expectedBandwidth := s.config.Test.Bandwidth

	for i, ip := range validIPs {
		s.testMu.RLock()
		if !s.testing {
			s.testMu.RUnlock()
			fmt.Println("Speed testing stopped by user")
			break
		}
		s.testMu.RUnlock()

		fmt.Printf("Speed testing IP %d/%d: %s\n", i+1, len(validIPs), ip)

		s.resultManager.UpdateCurrentTest(ip, "")

		datacenter, latency, err := enhancedTester.TestDataCenterOnly(ip, s.config.Test.UseTLS, s.config.Test.Timeout)
		if err != nil {
			fmt.Printf("Failed to get datacenter info for %s: %v\n", ip, err)
			continue
		}

		speedResult, err := enhancedTester.TestSpeedOnly(ip, s.config.Test.UseTLS, s.config.Test.Timeout, float64(s.config.Test.DownloadTime))
		if err != nil {
			fmt.Printf("Speed test failed for %s: %v\n", ip, err)

			result := &models.SpeedTestResult{
				IP:         ip,
				Status:     "无效",
				Latency:    fmt.Sprintf("%.2f", latency),
				Speed:      "timeout",
				DataCenter: s.coloManager.GetFriendlyName(datacenter),
				PeakSpeed:  0,
			}

			s.storeResult(result)
			continue
		}

		speedVal, _ := strconv.ParseFloat(speedResult.Speed, 64)
		if expectedBandwidth > 0 && speedVal < expectedBandwidth {
			speedResult.Status = "低速"
		}

		result := &models.SpeedTestResult{
			IP:         ip,
			Status:     speedResult.Status,
			Latency:    fmt.Sprintf("%.2f", latency),
			Speed:      speedResult.Speed,
			DataCenter: s.coloManager.GetFriendlyName(datacenter),
			PeakSpeed:  speedResult.PeakSpeed,
		}

		s.storeResult(result)

		s.resultManager.UpdateCurrentTest(result.IP, result.Speed)

		fmt.Printf("Speed test completed for %s: Status=%s, Speed=%s Mbps, Latency=%s ms, DataCenter=%s\n",
			ip, result.Status, result.Speed, result.Latency, result.DataCenter)

		time.Sleep(100 * time.Millisecond)

		qualifiedResults := s.resultManager.GetQualifiedResults()
		qualifiedCount := 0

		for _, r := range qualifiedResults {
			speedVal, err := strconv.ParseFloat(r.Speed, 64)
			if err == nil && speedVal >= expectedBandwidth {
				qualifiedCount++
			}
		}

		if qualifiedCount >= s.config.Test.ExpectedServers {
			fmt.Printf("\nFound %d qualified servers (speed >= %.2f Mbps). Expected: %d. Stopping speed test phase.\n",
				qualifiedCount, expectedBandwidth, s.config.Test.ExpectedServers)
			break
		}
	}

	s.resultManager.UpdateCurrentTest("", "")
}

// storeResult stores a test result using ResultManager and updates metrics
func (s *Server) storeResult(result *models.SpeedTestResult) {
	s.resultManager.AddResultAllowDuplicate(result)

	if result.Status == "已完成" {
		s.resultManager.UpdateCurrentTest(result.IP, result.Speed)

		speed, _ := strconv.ParseFloat(result.Speed, 64)
		latency, _ := strconv.ParseFloat(result.Latency, 64)

		s.metrics.RecordSpeedSample(speed, 0, 0)
		s.metrics.RecordLatencySample(latency)
		s.metrics.RecordTestComplete(true)

		s.metrics.RecordCounter("tests.successful", 1, map[string]string{
			"datacenter": result.DataCenter,
		})
	} else {
		s.metrics.RecordTestComplete(false)
		s.metrics.RecordCounter("tests.failed", 1, map[string]string{
			"status": result.Status,
		})
	}
}
