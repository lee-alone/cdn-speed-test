package examples

import (
	"cloudflare-speedtest/internal/tester"
	"fmt"
	"log"
	"time"
)

// Example1_BasicUsage demonstrates basic IP pool manager usage
func Example1_BasicUsage() {
	fmt.Println("=== Example 1: Basic Usage ===")

	// Create IP pool manager
	poolManager := tester.NewIPPoolManager("ipv4")

	// Register subnets
	subnets := []string{
		"1.1.1.0/24",
		"1.1.2.0/24",
		"1.1.3.0/24",
	}
	poolManager.RegisterSubnets(subnets)

	// Generate IPs
	fmt.Println("Generating 10 IPs...")
	for i := 0; i < 10; i++ {
		ip, subnet, err := poolManager.GenerateIPWithRetry(subnets)
		if err != nil {
			log.Printf("Failed to generate IP: %v", err)
			continue
		}
		fmt.Printf("IP %d: %s (from %s)\n", i+1, ip, subnet)
	}

	fmt.Println()
}

// Example2_PerformanceMonitoring demonstrates performance monitoring
func Example2_PerformanceMonitoring() {
	fmt.Println("=== Example 2: Performance Monitoring ===")

	poolManager := tester.NewIPPoolManager("ipv4")

	subnets := []string{
		"1.1.1.0/24",
		"1.1.2.0/24",
		"1.1.3.0/24",
	}
	poolManager.RegisterSubnets(subnets)

	// Generate some IPs
	for i := 0; i < 20; i++ {
		poolManager.GenerateIPWithRetry(subnets)
	}

	// Display metrics
	fmt.Println("Subnet Metrics:")
	fmt.Println("Subnet\t\tCapacity\tSuccess Rate\tTotal Attempts\tAvg Retries")
	fmt.Println("------\t\t--------\t------------\t--------------\t-----------")

	metrics := poolManager.GetMetrics()
	for subnet, m := range metrics {
		fmt.Printf("%s\t%d\t\t%.2f%%\t\t%d\t\t%d\n",
			subnet, m.Capacity, m.SuccessRate*100, m.TotalAttempts, m.AverageRetries)
	}

	// Display health status
	fmt.Println("\nOverall Health Status:")
	health := poolManager.GetHealthStatus()
	fmt.Printf("Total Subnets: %d\n", health["total_subnets"])
	fmt.Printf("Exhausted Subnets: %d\n", health["exhausted_subnets"])
	fmt.Printf("Total Attempts: %d\n", health["total_attempts"])
	fmt.Printf("Total Success: %d\n", health["total_success"])
	fmt.Printf("Overall Success Rate: %.2f%%\n", health["overall_success_rate"].(float64)*100)
	fmt.Printf("Generated IPs: %d\n", health["generated_ips"])

	fmt.Println()
}

// Example3_SmallSubnetHandling demonstrates handling of small subnets
func Example3_SmallSubnetHandling() {
	fmt.Println("=== Example 3: Small Subnet Handling ===")

	poolManager := tester.NewIPPoolManager("ipv4")

	// Small subnets with limited IPs
	smallSubnets := []string{
		"192.168.1.0/30", // Only 2 usable IPs
		"192.168.2.0/30", // Only 2 usable IPs
		"192.168.3.0/30", // Only 2 usable IPs
	}
	poolManager.RegisterSubnets(smallSubnets)

	fmt.Println("Attempting to generate 10 IPs from small subnets...")
	successCount := 0
	for i := 0; i < 10; i++ {
		ip, subnet, err := poolManager.GenerateIPWithRetry(smallSubnets)
		if err != nil {
			fmt.Printf("IP %d: FAILED - %v\n", i+1, err)
		} else {
			fmt.Printf("IP %d: %s (from %s)\n", i+1, ip, subnet)
			successCount++
		}
	}

	fmt.Printf("\nSuccess Rate: %d/10 (%.0f%%)\n", successCount, float64(successCount)*10)

	// Show metrics
	fmt.Println("\nSubnet Metrics:")
	metrics := poolManager.GetMetrics()
	for subnet, m := range metrics {
		fmt.Printf("%s: Success Rate=%.2f%%, Capacity=%d, Exhausted=%v\n",
			subnet, m.SuccessRate*100, m.Capacity, m.IsExhausted)
	}

	fmt.Println()
}

// Example4_FallbackMode demonstrates fallback mode
func Example4_FallbackMode() {
	fmt.Println("=== Example 4: Fallback Mode ===")

	poolManager := tester.NewIPPoolManager("ipv4")

	primarySubnets := []string{
		"1.1.1.0/24",
		"1.1.2.0/24",
	}
	poolManager.RegisterSubnets(primarySubnets)

	fmt.Println("Generating IPs with fallback mode disabled...")
	poolManager.SetFallbackMode(false)

	for i := 0; i < 5; i++ {
		ip, subnet, err := poolManager.GenerateIPWithRetry(primarySubnets)
		if err != nil {
			fmt.Printf("IP %d: FAILED - %v\n", i+1, err)
		} else {
			fmt.Printf("IP %d: %s (from %s)\n", i+1, ip, subnet)
		}
	}

	fmt.Println("\nEnabling fallback mode...")
	poolManager.SetFallbackMode(true)

	backupSubnets := []string{
		"2.2.2.0/24",
		"2.2.3.0/24",
	}
	poolManager.RegisterSubnets(backupSubnets)

	for i := 0; i < 5; i++ {
		allSubnets := append(primarySubnets, backupSubnets...)
		ip, subnet, err := poolManager.GenerateIPWithRetry(allSubnets)
		if err != nil {
			fmt.Printf("IP %d: FAILED - %v\n", i+1, err)
		} else {
			fmt.Printf("IP %d: %s (from %s)\n", i+1, ip, subnet)
		}
	}

	fmt.Println()
}

// Example5_ResetAndRecovery demonstrates reset and recovery
func Example5_ResetAndRecovery() {
	fmt.Println("=== Example 5: Reset and Recovery ===")

	poolManager := tester.NewIPPoolManager("ipv4")

	subnets := []string{
		"1.1.1.0/24",
		"1.1.2.0/24",
	}
	poolManager.RegisterSubnets(subnets)

	// Generate some IPs
	fmt.Println("Generating initial IPs...")
	for i := 0; i < 5; i++ {
		poolManager.GenerateIPWithRetry(subnets)
	}

	// Show metrics before reset
	fmt.Println("\nMetrics before reset:")
	health := poolManager.GetHealthStatus()
	fmt.Printf("Generated IPs: %d\n", health["generated_ips"])
	fmt.Printf("Total Attempts: %d\n", health["total_attempts"])

	// Reset all
	fmt.Println("\nResetting all metrics...")
	poolManager.ResetAll()

	// Show metrics after reset
	fmt.Println("\nMetrics after reset:")
	health = poolManager.GetHealthStatus()
	fmt.Printf("Generated IPs: %d\n", health["generated_ips"])
	fmt.Printf("Total Attempts: %d\n", health["total_attempts"])

	// Generate IPs again
	fmt.Println("\nGenerating IPs after reset...")
	for i := 0; i < 3; i++ {
		ip, subnet, err := poolManager.GenerateIPWithRetry(subnets)
		if err != nil {
			fmt.Printf("IP %d: FAILED - %v\n", i+1, err)
		} else {
			fmt.Printf("IP %d: %s (from %s)\n", i+1, ip, subnet)
		}
	}

	fmt.Println()
}

// Example6_ConcurrentGeneration demonstrates concurrent IP generation
func Example6_ConcurrentGeneration() {
	fmt.Println("=== Example 6: Concurrent Generation ===")

	poolManager := tester.NewIPPoolManager("ipv4")

	subnets := []string{
		"1.1.1.0/24",
		"1.1.2.0/24",
		"1.1.3.0/24",
	}
	poolManager.RegisterSubnets(subnets)

	// Generate IPs concurrently
	const numWorkers = 4
	const ipsPerWorker = 5

	ipChan := make(chan string, numWorkers*ipsPerWorker)
	errChan := make(chan error, numWorkers)

	fmt.Printf("Generating %d IPs using %d workers...\n", numWorkers*ipsPerWorker, numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			for i := 0; i < ipsPerWorker; i++ {
				ip, _, err := poolManager.GenerateIPWithRetry(subnets)
				if err != nil {
					errChan <- err
					return
				}
				ipChan <- ip
				fmt.Printf("Worker %d generated: %s\n", workerID, ip)
			}
		}(w)
	}

	// Collect results
	var ips []string
	for i := 0; i < numWorkers*ipsPerWorker; i++ {
		select {
		case ip := <-ipChan:
			ips = append(ips, ip)
		case err := <-errChan:
			fmt.Printf("Error: %v\n", err)
		}
	}

	fmt.Printf("\nTotal IPs generated: %d\n", len(ips))

	fmt.Println()
}

// Example7_PerformanceComparison compares performance with different subnet sizes
func Example7_PerformanceComparison() {
	fmt.Println("=== Example 7: Performance Comparison ===")

	testCases := []struct {
		name    string
		subnets []string
	}{
		{
			name: "Small Subnets (/30)",
			subnets: []string{
				"192.168.1.0/30",
				"192.168.2.0/30",
				"192.168.3.0/30",
			},
		},
		{
			name: "Medium Subnets (/24)",
			subnets: []string{
				"10.0.1.0/24",
				"10.0.2.0/24",
				"10.0.3.0/24",
			},
		},
		{
			name: "Large Subnets (/22)",
			subnets: []string{
				"172.16.0.0/22",
				"172.16.4.0/22",
				"172.16.8.0/22",
			},
		},
	}

	for _, tc := range testCases {
		fmt.Printf("Testing: %s\n", tc.name)

		poolManager := tester.NewIPPoolManager("ipv4")
		poolManager.RegisterSubnets(tc.subnets)

		start := time.Now()
		successCount := 0

		for i := 0; i < 20; i++ {
			_, _, err := poolManager.GenerateIPWithRetry(tc.subnets)
			if err == nil {
				successCount++
			}
		}

		duration := time.Since(start)

		health := poolManager.GetHealthStatus()
		fmt.Printf("  Success Rate: %d/20 (%.0f%%)\n", successCount, float64(successCount)*5)
		fmt.Printf("  Total Attempts: %d\n", health["total_attempts"])
		fmt.Printf("  Duration: %v\n", duration)
		fmt.Printf("  Avg Time per IP: %v\n", duration/20)

		fmt.Println()
	}
}

// Example8_ErrorHandling demonstrates error handling strategies
func Example8_ErrorHandling() {
	fmt.Println("=== Example 8: Error Handling ===")

	poolManager := tester.NewIPPoolManager("ipv4")

	subnets := []string{
		"1.1.1.0/24",
		"1.1.2.0/24",
	}
	poolManager.RegisterSubnets(subnets)

	fmt.Println("Attempting to generate IP with retry logic...")

	const maxRetries = 3
	var ip string
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		ip, _, err = poolManager.GenerateIPWithRetry(subnets)
		if err == nil {
			fmt.Printf("Success on attempt %d: %s\n", attempt+1, ip)
			break
		}

		fmt.Printf("Attempt %d failed: %v\n", attempt+1, err)

		if attempt < maxRetries-1 {
			fmt.Println("Resetting and retrying...")
			poolManager.ResetAll()
			time.Sleep(time.Second)
		}
	}

	if err != nil {
		fmt.Printf("Failed after %d attempts: %v\n", maxRetries, err)
	}

	fmt.Println()
}

// RunAllExamples runs all examples
func RunAllExamples() {
	examples := []func(){
		Example1_BasicUsage,
		Example2_PerformanceMonitoring,
		Example3_SmallSubnetHandling,
		Example4_FallbackMode,
		Example5_ResetAndRecovery,
		Example6_ConcurrentGeneration,
		Example7_PerformanceComparison,
		Example8_ErrorHandling,
	}

	for _, example := range examples {
		example()
		time.Sleep(500 * time.Millisecond)
	}
}
