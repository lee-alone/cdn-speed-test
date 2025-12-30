package tester

import (
	"bufio"
	"cloudflare-speedtest/internal/generator"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
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

	// Randomly select subnets and generate IPs
	rand.Seed(time.Now().UnixNano())
	selectedIndices := make(map[int]bool)
	var ips []string

	for len(ips) < batchSize && len(selectedIndices) < len(subnets) {
		// Generate random index
		idx := rand.Intn(len(subnets))

		// Skip if already selected
		if selectedIndices[idx] {
			continue
		}

		selectedIndices[idx] = true
		subnet := subnets[idx]

		// Generate a random IP from the CIDR subnet
		ip := ir.ipGen.GenerateIP(subnet, ipType)
		if ip != "" {
			ips = append(ips, ip)
		}
	}

	return ips, nil
}
