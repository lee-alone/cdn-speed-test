package tester

import (
	"cloudflare-speedtest/internal/generator"
	"fmt"
	"net"
	"sync"
	"time"
)

// SubnetMetrics tracks performance metrics for a subnet
type SubnetMetrics struct {
	Subnet         string
	TotalAttempts  int
	SuccessCount   int
	FailureCount   int
	LastUsed       time.Time
	SuccessRate    float64
	AverageRetries int
	Capacity       int
	IsExhausted    bool
	Priority       int // Higher priority = prefer this subnet
}

// IPPoolManager manages IP generation with intelligent pooling and retry strategies
type IPPoolManager struct {
	mu                  sync.RWMutex
	ipGen               *generator.IPGenerator
	subnetMetrics       map[string]*SubnetMetrics
	generatedIPs        map[string]string // ip -> subnet mapping
	ipType              string
	maxRetries          int
	exhaustionThreshold float64 // Percentage threshold for marking subnet as exhausted
	fallbackMode        bool    // Allow fallback to other datacenters
}

// NewIPPoolManager creates a new IP pool manager
func NewIPPoolManager(ipType string) *IPPoolManager {
	return &IPPoolManager{
		ipGen:               generator.New(),
		subnetMetrics:       make(map[string]*SubnetMetrics),
		generatedIPs:        make(map[string]string),
		ipType:              ipType,
		maxRetries:          200,  // Increased from 100
		exhaustionThreshold: 0.85, // 85% instead of 90%
		fallbackMode:        false,
	}
}

// SetFallbackMode enables/disables fallback to other datacenters
func (ipm *IPPoolManager) SetFallbackMode(enabled bool) {
	ipm.mu.Lock()
	defer ipm.mu.Unlock()
	ipm.fallbackMode = enabled
}

// RegisterSubnets registers subnets for a datacenter
func (ipm *IPPoolManager) RegisterSubnets(subnets []string) {
	ipm.mu.Lock()
	defer ipm.mu.Unlock()

	for _, subnet := range subnets {
		if _, exists := ipm.subnetMetrics[subnet]; !exists {
			capacity := ipm.calculateSubnetCapacity(subnet)
			ipm.subnetMetrics[subnet] = &SubnetMetrics{
				Subnet:      subnet,
				Capacity:    capacity,
				Priority:    100, // Default priority
				IsExhausted: false,
			}
		}
	}
}

// GenerateIPWithRetry generates an IP with intelligent retry strategy
func (ipm *IPPoolManager) GenerateIPWithRetry(subnets []string) (string, string, error) {
	ipm.mu.Lock()
	defer ipm.mu.Unlock()

	if len(subnets) == 0 {
		return "", "", fmt.Errorf("no subnets provided")
	}

	// Sort subnets by priority and success rate
	sortedSubnets := ipm.sortSubnetsByPriority(subnets)

	// Try each subnet with adaptive retry count
	for _, subnet := range sortedSubnets {
		metrics := ipm.subnetMetrics[subnet]
		if metrics == nil {
			metrics = &SubnetMetrics{
				Subnet:   subnet,
				Capacity: ipm.calculateSubnetCapacity(subnet),
				Priority: 100,
			}
			ipm.subnetMetrics[subnet] = metrics
		}

		// Skip if subnet is exhausted
		if metrics.IsExhausted {
			continue
		}

		// Calculate adaptive retry count based on subnet capacity
		adaptiveRetries := ipm.calculateAdaptiveRetries(metrics)

		// Try to generate IP from this subnet
		for attempt := 0; attempt < adaptiveRetries; attempt++ {
			result := ipm.ipGen.GenerateIP(subnet, ipm.ipType)

			if result.Success {
				// Update metrics
				metrics.SuccessCount++
				metrics.TotalAttempts++
				metrics.LastUsed = time.Now()
				metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.TotalAttempts)
				metrics.AverageRetries = (metrics.AverageRetries + result.Attempts) / 2

				// Track generated IP
				ipm.generatedIPs[result.IP] = subnet

				return result.IP, subnet, nil
			}

			metrics.FailureCount++
			metrics.TotalAttempts++
		}

		// Check if subnet should be marked as exhausted
		if metrics.TotalAttempts > 0 {
			exhaustionRate := float64(metrics.SuccessCount) / float64(metrics.TotalAttempts)
			if exhaustionRate < (1 - ipm.exhaustionThreshold) {
				metrics.IsExhausted = true
			}
		}
	}

	return "", "", fmt.Errorf("failed to generate IP from any subnet after adaptive retries")
}

// calculateAdaptiveRetries calculates retry count based on subnet metrics
func (ipm *IPPoolManager) calculateAdaptiveRetries(metrics *SubnetMetrics) int {
	// Base retry count
	baseRetries := ipm.maxRetries

	// Adjust based on subnet capacity
	if metrics.Capacity < 100 {
		// Small subnet: allow more retries
		baseRetries = ipm.maxRetries * 2
	} else if metrics.Capacity < 500 {
		// Medium subnet: normal retries
		baseRetries = ipm.maxRetries
	} else {
		// Large subnet: fewer retries needed
		baseRetries = ipm.maxRetries / 2
	}

	// Adjust based on success rate
	if metrics.SuccessRate > 0.8 {
		// High success rate: reduce retries
		baseRetries = baseRetries / 2
	} else if metrics.SuccessRate < 0.3 && metrics.TotalAttempts > 10 {
		// Low success rate: increase retries
		baseRetries = baseRetries * 2
	}

	return baseRetries
}

// sortSubnetsByPriority sorts subnets by priority, success rate, and capacity
func (ipm *IPPoolManager) sortSubnetsByPriority(subnets []string) []string {
	sorted := make([]string, len(subnets))
	copy(sorted, subnets)

	// Simple bubble sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			metricsI := ipm.subnetMetrics[sorted[i]]
			metricsJ := ipm.subnetMetrics[sorted[j]]

			if metricsI == nil {
				metricsI = &SubnetMetrics{Subnet: sorted[i], Priority: 100}
			}
			if metricsJ == nil {
				metricsJ = &SubnetMetrics{Subnet: sorted[j], Priority: 100}
			}

			// Compare: priority > success rate > capacity
			if metricsJ.Priority > metricsI.Priority ||
				(metricsJ.Priority == metricsI.Priority && metricsJ.SuccessRate > metricsI.SuccessRate) ||
				(metricsJ.Priority == metricsI.Priority && metricsJ.SuccessRate == metricsI.SuccessRate && metricsJ.Capacity > metricsI.Capacity) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// calculateSubnetCapacity calculates the maximum number of host addresses in a subnet
func (ipm *IPPoolManager) calculateSubnetCapacity(subnet string) int {
	// Parse CIDR notation directly
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return 0
	}

	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones

	switch bits {
	case 32: // IPv4
		if hostBits <= 1 {
			return 1
		}
		return (1 << uint(hostBits)) - 2
	case 128: // IPv6
		if hostBits <= 0 {
			return 1
		}
		if hostBits > 32 {
			return 1 << 32
		}
		return 1 << uint(hostBits)
	default:
		return 0
	}
}

// GetMetrics returns metrics for all subnets
func (ipm *IPPoolManager) GetMetrics() map[string]*SubnetMetrics {
	ipm.mu.RLock()
	defer ipm.mu.RUnlock()

	result := make(map[string]*SubnetMetrics)
	for subnet, metrics := range ipm.subnetMetrics {
		result[subnet] = &SubnetMetrics{
			Subnet:         metrics.Subnet,
			TotalAttempts:  metrics.TotalAttempts,
			SuccessCount:   metrics.SuccessCount,
			FailureCount:   metrics.FailureCount,
			LastUsed:       metrics.LastUsed,
			SuccessRate:    metrics.SuccessRate,
			AverageRetries: metrics.AverageRetries,
			Capacity:       metrics.Capacity,
			IsExhausted:    metrics.IsExhausted,
			Priority:       metrics.Priority,
		}
	}
	return result
}

// ResetSubnet resets metrics for a subnet
func (ipm *IPPoolManager) ResetSubnet(subnet string) {
	ipm.mu.Lock()
	defer ipm.mu.Unlock()

	if metrics, exists := ipm.subnetMetrics[subnet]; exists {
		metrics.IsExhausted = false
		metrics.TotalAttempts = 0
		metrics.SuccessCount = 0
		metrics.FailureCount = 0
		metrics.SuccessRate = 0
	}
}

// ResetAll resets all metrics
func (ipm *IPPoolManager) ResetAll() {
	ipm.mu.Lock()
	defer ipm.mu.Unlock()

	for _, metrics := range ipm.subnetMetrics {
		metrics.IsExhausted = false
		metrics.TotalAttempts = 0
		metrics.SuccessCount = 0
		metrics.FailureCount = 0
		metrics.SuccessRate = 0
	}
	ipm.generatedIPs = make(map[string]string)
	ipm.ipGen.ClearGenerated()
}

// GetSubnetForIP returns the subnet that generated a specific IP
func (ipm *IPPoolManager) GetSubnetForIP(ip string) string {
	ipm.mu.RLock()
	defer ipm.mu.RUnlock()

	return ipm.generatedIPs[ip]
}

// GetHealthStatus returns overall health status
func (ipm *IPPoolManager) GetHealthStatus() map[string]interface{} {
	ipm.mu.RLock()
	defer ipm.mu.RUnlock()

	totalSubnets := len(ipm.subnetMetrics)
	exhaustedSubnets := 0
	totalAttempts := 0
	totalSuccess := 0

	for _, metrics := range ipm.subnetMetrics {
		if metrics.IsExhausted {
			exhaustedSubnets++
		}
		totalAttempts += metrics.TotalAttempts
		totalSuccess += metrics.SuccessCount
	}

	overallSuccessRate := 0.0
	if totalAttempts > 0 {
		overallSuccessRate = float64(totalSuccess) / float64(totalAttempts)
	}

	return map[string]interface{}{
		"total_subnets":        totalSubnets,
		"exhausted_subnets":    exhaustedSubnets,
		"total_attempts":       totalAttempts,
		"total_success":        totalSuccess,
		"overall_success_rate": overallSuccessRate,
		"generated_ips":        len(ipm.generatedIPs),
	}
}
