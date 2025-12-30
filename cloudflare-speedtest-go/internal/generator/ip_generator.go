package generator

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// IPGenerator generates IP addresses from subnets with enhanced features
type IPGenerator struct {
	mu           sync.RWMutex
	generatedIPs map[string]bool         // Track generated IPs to prevent duplicates
	subnetStats  map[string]*SubnetStats // Track subnet usage statistics
	maxRetries   int                     // Maximum retries for generating unique IPs
}

// SubnetStats tracks statistics for a subnet
type SubnetStats struct {
	TotalGenerated int       // Total IPs generated from this subnet
	MaxCapacity    int       // Maximum possible IPs in this subnet
	LastGenerated  time.Time // Last time an IP was generated
	Exhausted      bool      // Whether this subnet is exhausted
}

// GenerationResult represents the result of IP generation
type GenerationResult struct {
	IP       string
	Subnet   string
	Success  bool
	Error    error
	Attempts int
}

// New creates a new enhanced IP generator
func New() *IPGenerator {
	return &IPGenerator{
		generatedIPs: make(map[string]bool),
		subnetStats:  make(map[string]*SubnetStats),
		maxRetries:   100, // Maximum attempts to generate unique IP
	}
}

// SetMaxRetries sets the maximum number of retries for generating unique IPs
func (ig *IPGenerator) SetMaxRetries(retries int) {
	ig.mu.Lock()
	defer ig.mu.Unlock()
	ig.maxRetries = retries
}

// GenerateIP generates a random IP from a subnet with enhanced features
func (ig *IPGenerator) GenerateIP(subnet string, ipType string) *GenerationResult {
	ig.mu.Lock()
	defer ig.mu.Unlock()

	// Check if subnet is exhausted
	if stats, exists := ig.subnetStats[subnet]; exists && stats.Exhausted {
		return &GenerationResult{
			Subnet:  subnet,
			Success: false,
			Error:   fmt.Errorf("subnet %s is exhausted", subnet),
		}
	}

	var result *GenerationResult
	if ipType == "ipv4" {
		result = ig.generateIPv4Enhanced(subnet)
	} else {
		result = ig.generateIPv6Enhanced(subnet)
	}

	// Update subnet statistics
	ig.updateSubnetStats(subnet, result)

	return result
}

// generateIPv4Enhanced generates a random IPv4 address with enhanced features
func (ig *IPGenerator) generateIPv4Enhanced(subnet string) *GenerationResult {
	// Parse CIDR notation
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return &GenerationResult{
			Subnet:  subnet,
			Success: false,
			Error:   fmt.Errorf("invalid subnet format: %w", err),
		}
	}

	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return &GenerationResult{
			Subnet:  subnet,
			Success: false,
			Error:   fmt.Errorf("invalid IPv4 subnet"),
		}
	}

	hostBits := bits - ones
	if hostBits <= 1 {
		// Single host or network address only
		return &GenerationResult{
			IP:      ipnet.IP.String(),
			Subnet:  subnet,
			Success: true,
		}
	}

	maxHosts := (1 << uint(hostBits)) - 2 // Exclude network and broadcast addresses
	networkIP := ipToInt(ipnet.IP)

	for attempt := 0; attempt < ig.maxRetries; attempt++ {
		// Generate cryptographically secure random offset
		offset, err := ig.generateSecureRandom(maxHosts)
		if err != nil {
			continue
		}

		// Skip network address (offset 0) and broadcast address (maxHosts+1)
		if offset == 0 {
			offset = 1
		}

		resultIP := intToIP(networkIP + uint32(offset))
		ipStr := resultIP.String()

		// Check for duplicates
		if !ig.generatedIPs[ipStr] {
			ig.generatedIPs[ipStr] = true
			return &GenerationResult{
				IP:       ipStr,
				Subnet:   subnet,
				Success:  true,
				Attempts: attempt + 1,
			}
		}
	}

	// Mark subnet as exhausted if we can't generate unique IPs
	return &GenerationResult{
		Subnet:   subnet,
		Success:  false,
		Error:    fmt.Errorf("failed to generate unique IP after %d attempts", ig.maxRetries),
		Attempts: ig.maxRetries,
	}
}

// generateIPv6Enhanced generates a random IPv6 address with enhanced features
func (ig *IPGenerator) generateIPv6Enhanced(subnet string) *GenerationResult {
	// Parse CIDR notation
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return &GenerationResult{
			Subnet:  subnet,
			Success: false,
			Error:   fmt.Errorf("invalid IPv6 subnet format: %w", err),
		}
	}

	ones, bits := ipnet.Mask.Size()
	if bits != 128 {
		return &GenerationResult{
			Subnet:  subnet,
			Success: false,
			Error:   fmt.Errorf("invalid IPv6 subnet"),
		}
	}

	hostBits := bits - ones
	if hostBits <= 0 {
		// Single host address
		return &GenerationResult{
			IP:      ipnet.IP.String(),
			Subnet:  subnet,
			Success: true,
		}
	}

	networkIP := ipnet.IP.To16()
	if networkIP == nil {
		return &GenerationResult{
			Subnet:  subnet,
			Success: false,
			Error:   fmt.Errorf("invalid IPv6 address"),
		}
	}

	for attempt := 0; attempt < ig.maxRetries; attempt++ {
		// Generate random IPv6 suffix
		resultIP := make(net.IP, 16)
		copy(resultIP, networkIP)

		// Generate random bytes for the host portion
		hostBytes := hostBits / 8
		if hostBits%8 != 0 {
			hostBytes++
		}

		// Generate random bytes for host portion
		for i := 0; i < hostBytes && i < 8; i++ {
			byteIndex := 16 - hostBytes + i
			if byteIndex >= 0 && byteIndex < 16 {
				randomByte, err := ig.generateSecureRandomByte()
				if err != nil {
					continue
				}
				resultIP[byteIndex] = randomByte
			}
		}

		ipStr := resultIP.String()

		// Check for duplicates
		if !ig.generatedIPs[ipStr] {
			ig.generatedIPs[ipStr] = true
			return &GenerationResult{
				IP:       ipStr,
				Subnet:   subnet,
				Success:  true,
				Attempts: attempt + 1,
			}
		}
	}

	return &GenerationResult{
		Subnet:   subnet,
		Success:  false,
		Error:    fmt.Errorf("failed to generate unique IPv6 after %d attempts", ig.maxRetries),
		Attempts: ig.maxRetries,
	}
}

// generateSecureRandom generates a cryptographically secure random number
func (ig *IPGenerator) generateSecureRandom(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}

	return int(n.Int64()), nil
}

// generateSecureRandomByte generates a cryptographically secure random byte
func (ig *IPGenerator) generateSecureRandomByte() (byte, error) {
	bytes := make([]byte, 1)
	_, err := rand.Read(bytes)
	if err != nil {
		return 0, err
	}
	return bytes[0], nil
}

// updateSubnetStats updates statistics for a subnet
func (ig *IPGenerator) updateSubnetStats(subnet string, result *GenerationResult) {
	stats, exists := ig.subnetStats[subnet]
	if !exists {
		// Calculate maximum capacity for this subnet
		maxCapacity := ig.calculateSubnetCapacity(subnet)
		stats = &SubnetStats{
			MaxCapacity: maxCapacity,
		}
		ig.subnetStats[subnet] = stats
	}

	stats.LastGenerated = time.Now()

	if result.Success {
		stats.TotalGenerated++

		// Check if subnet is approaching exhaustion
		if stats.TotalGenerated >= stats.MaxCapacity*90/100 { // 90% threshold
			stats.Exhausted = true
		}
	} else if result.Attempts >= ig.maxRetries {
		// Mark as exhausted if we failed after max retries
		stats.Exhausted = true
	}
}

// calculateSubnetCapacity calculates the maximum number of host addresses in a subnet
func (ig *IPGenerator) calculateSubnetCapacity(subnet string) int {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return 0
	}

	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones

	if bits == 32 { // IPv4
		if hostBits <= 1 {
			return 1
		}
		return (1 << uint(hostBits)) - 2 // Exclude network and broadcast
	} else if bits == 128 { // IPv6
		if hostBits <= 0 {
			return 1
		}
		// For IPv6, we limit to a reasonable number to prevent overflow
		if hostBits > 32 {
			return 1 << 32 // Limit to 4 billion addresses
		}
		return 1 << uint(hostBits)
	}

	return 0
}

// GetSubnetStats returns statistics for all subnets
func (ig *IPGenerator) GetSubnetStats() map[string]*SubnetStats {
	ig.mu.RLock()
	defer ig.mu.RUnlock()

	result := make(map[string]*SubnetStats)
	for subnet, stats := range ig.subnetStats {
		result[subnet] = &SubnetStats{
			TotalGenerated: stats.TotalGenerated,
			MaxCapacity:    stats.MaxCapacity,
			LastGenerated:  stats.LastGenerated,
			Exhausted:      stats.Exhausted,
		}
	}

	return result
}

// GetGeneratedCount returns the total number of generated IPs
func (ig *IPGenerator) GetGeneratedCount() int {
	ig.mu.RLock()
	defer ig.mu.RUnlock()
	return len(ig.generatedIPs)
}

// HasIP checks if an IP has been generated before
func (ig *IPGenerator) HasIP(ip string) bool {
	ig.mu.RLock()
	defer ig.mu.RUnlock()
	return ig.generatedIPs[ip]
}

// ClearGenerated clears all generated IP records
func (ig *IPGenerator) ClearGenerated() {
	ig.mu.Lock()
	defer ig.mu.Unlock()

	ig.generatedIPs = make(map[string]bool)
	ig.subnetStats = make(map[string]*SubnetStats)
}

// ResetSubnet resets the exhausted status of a subnet
func (ig *IPGenerator) ResetSubnet(subnet string) {
	ig.mu.Lock()
	defer ig.mu.Unlock()

	if stats, exists := ig.subnetStats[subnet]; exists {
		stats.Exhausted = false
	}
}

// ipToInt converts an IPv4 address to a 32-bit integer
func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// intToIP converts a 32-bit integer to an IPv4 address
func intToIP(i uint32) net.IP {
	return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
}
