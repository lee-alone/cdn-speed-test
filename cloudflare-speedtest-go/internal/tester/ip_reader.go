package tester

import (
	"bufio"
	"cloudflare-speedtest/internal/generator"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

// IPReader reads IP addresses from files
type IPReader struct {
	dataDir string
	ipGen   *generator.IPGenerator
}

// NewIPReader creates a new IP reader
func NewIPReader(dataDir string) *IPReader {
	return &IPReader{
		dataDir: dataDir,
		ipGen:   generator.New(),
	}
}

// ReadIPs reads IP addresses from file based on IP type
// Returns up to batchSize IPs randomly selected from the file
func (ir *IPReader) ReadIPs(ipType string, batchSize int) ([]string, error) {
	var filename string
	switch ipType {
	case "ipv4":
		filename = "ips-v4.txt"
	case "ipv6":
		filename = "ips-v6.txt"
	default:
		return nil, fmt.Errorf("invalid IP type: %s", ipType)
	}

	filePath := filepath.Join(ir.dataDir, filename)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer file.Close()

	// First pass: read all subnets
	var subnets []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			subnets = append(subnets, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filename, err)
	}

	if len(subnets) == 0 {
		return nil, fmt.Errorf("no subnets found in %s", filename)
	}

	// Clear previous generation history for fresh start
	ir.ipGen.ClearGenerated()

	// Randomly select subnets and generate IPs
	selectedIndices := make(map[int]bool)
	var ips []string
	maxAttempts := batchSize * 3 // Allow more attempts to find unique IPs

	for len(ips) < batchSize && len(selectedIndices) < len(subnets) && maxAttempts > 0 {
		maxAttempts--

		// Generate cryptographically secure random index
		idx, err := ir.generateSecureRandomInt(len(subnets))
		if err != nil {
			// Fallback to simple selection if crypto random fails
			idx = len(selectedIndices) % len(subnets)
		}

		// Skip if already selected
		if selectedIndices[idx] {
			continue
		}

		selectedIndices[idx] = true
		subnet := subnets[idx]

		// Generate a random IP from the CIDR subnet
		result := ir.ipGen.GenerateIP(subnet, ipType)
		if result.Success {
			ips = append(ips, result.IP)
		} else {
			fmt.Printf("Failed to generate IP from subnet %s: %v\n", subnet, result.Error)
		}
	}

	return ips, nil
}

// generateSecureRandomInt generates a cryptographically secure random integer
func (ir *IPReader) generateSecureRandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}

	return int(n.Int64()), nil
}

// GetGeneratorStats returns statistics about IP generation
func (ir *IPReader) GetGeneratorStats() map[string]*generator.SubnetStats {
	return ir.ipGen.GetSubnetStats()
}

// GetGeneratedCount returns the total number of generated IPs
func (ir *IPReader) GetGeneratedCount() int {
	return ir.ipGen.GetGeneratedCount()
}
