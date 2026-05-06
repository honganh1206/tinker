package sandbox

import (
	"net"
	"testing"
)

func TestNewIPPool(t *testing.T) {
	// First 24 bits are for network
	_, network, err := net.ParseCIDR("192.168.100.0/24")
	if err != nil {
		t.Fatalf("Failed to parse CIDR: %v", err)
	}

	// Create a pool of hosts under the same network
	pool, err := NewIPPool(network)
	if err != nil {
		t.Fatalf("Failed to create IP pool: %v", err)
	}

	// Should have 254 available IPs, 2 for network and broadcast
	if pool.Available() != 254 {
		t.Errorf("Expected 254 available IPs, got %d", pool.Available())
	}
}

func TestIPPoolAllocation(t *testing.T) {
	_, network, err := net.ParseCIDR("192.168.100.0/28")
	if err != nil {
		t.Fatalf("Failed to parse CIDR: %v", err)
	}

	pool, err := NewIPPool(network)
	if err != nil {
		t.Fatalf("Failed to create IP pool: %v", err)
	}

	// With /28 we should have 14 available IPs (minus 2 for network and broadcast)
	expectedAvailable := 14
	if pool.Available() != expectedAvailable {
		t.Errorf("Expected %d available IPs got %d", expectedAvailable, pool.Available())
	}

	// Allocate an IP
	ip1, err := pool.Allocate()
	if err != nil {
		t.Fatalf("Failed to allocate IP: %v", err)
	}

	if !network.Contains(ip1) {
		t.Errorf("Allocated IP %s is not in network %s", ip1, network)
	}

	if pool.Available() != expectedAvailable-1 {
		t.Errorf("Expected %d available IPs after alllcation, got %d", expectedAvailable-1, pool.Available())
	}

	// Check if IP is marked allocated
	if !pool.IsAllocated(ip1) {
		t.Errorf("IP %s should be marked as allocated", ip1)
	}

	// Allocate another IP
	ip2, err := pool.Allocate()
	if err != nil {
		t.Fatalf("Failed to allocate second IP: %v", err)
	}

	if ip1.Equal(ip2) {
		t.Errorf("Allocated the same IP twice: %s", ip1)
	}

	// Release the first IP
	pool.Release(ip1)

	if pool.IsAllocated(ip1) {
		t.Errorf("IP %s should not be marked as allocated after release", ip1)
	}

	if pool.Available() != expectedAvailable-1 {
		t.Errorf("Expected %d available IPs after release, got %d", expectedAvailable-1, pool.Available())
	}
}
