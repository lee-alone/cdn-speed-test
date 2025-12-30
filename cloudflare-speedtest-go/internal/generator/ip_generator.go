package generator

import (
	"math/rand"
	"net"
	"strconv"
	"strings"
)

// IPGenerator generates IP addresses from subnets
type IPGenerator struct {
}

// New creates a new IP generator
func New() *IPGenerator {
	return &IPGenerator{}
}

// GenerateIP generates a random IP from a subnet
func (ig *IPGenerator) GenerateIP(subnet string, ipType string) string {
	if ipType == "ipv4" {
		return ig.generateIPv4(subnet)
	}
	return ig.generateIPv6(subnet)
}

// generateIPv4 generates a random IPv4 address from a subnet
func (ig *IPGenerator) generateIPv4(subnet string) string {
	// Parse CIDR notation
	ip, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return ""
	}

	// Generate random IP within the subnet
	ones, bits := ipnet.Mask.Size()
	if bits-ones <= 0 {
		return ip.String()
	}

	// Convert IP to integer
	ipInt := ipToInt(ip)

	// Generate random offset
	maxOffset := (1 << uint(bits-ones)) - 1
	offset := rand.Intn(maxOffset)

	// Add offset to IP
	resultInt := ipInt + uint32(offset)

	// Convert back to IP
	return intToIP(resultInt).String()
}

// generateIPv6 generates a random IPv6 address from a subnet
func (ig *IPGenerator) generateIPv6(subnet string) string {
	// Parse CIDR notation
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return ""
	}

	// For IPv6, we'll just return the network address with a random suffix
	// This is a simplified implementation
	parts := strings.Split(ip.String(), ":")
	if len(parts) > 0 {
		// Generate random suffix
		suffix := rand.Intn(65535)
		return ip.String() + ":" + strconv.Itoa(suffix)
	}

	return ip.String()
}

// ipToInt converts an IP address to a 32-bit integer
func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// intToIP converts a 32-bit integer to an IP address
func intToIP(i uint32) net.IP {
	return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
}
