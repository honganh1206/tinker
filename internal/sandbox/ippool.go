package sandbox

import (
	"fmt"
	"net"
	"sync"
)

type IPPool struct {
	// An IP network
	network *net.IPNet
	// Allocated IPs
	allocated map[string]bool
	// Available IP addresses
	available []net.IP
	mu        sync.Mutex
}

func NewIPPool(network *net.IPNet) (*IPPool, error) {
	pool := &IPPool{
		network:   network,
		allocated: make(map[string]bool),
		available: make([]net.IP, 0),
	}

	// Generate all usuable IPs in the network
	// and skip network and boradcast addresses
	for ip := network.IP.Mask(network.Mask); network.Contains(ip); inc(ip) {
		if !ip.Equal(network.IP) && !isBroadcast(ip, network) {
			pool.available = append(pool.available, copyIP(ip))
		}
	}

	if len(pool.available) == 0 {
		return nil, fmt.Errorf("no available IP addresses in network %s", network.String())
	}

	return pool, nil
}

// Available returns the number of available IP addresses
func (p *IPPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.available) - len(p.allocated)
}

// Allocate allocates an IP address from the pool
func (p *IPPool) Allocate() (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, ip := range p.available {
		ipStr := ip.String()
		if !p.allocated[ipStr] {
			p.allocated[ipStr] = true
			return ip, nil
		}

		// No IP available
		if i == len(p.available)-1 {
			break
		}
	}
	return nil, fmt.Errorf("no available IP addresses")
}

// Release releases an IP address back to the pool
func (p *IPPool) Release(ip net.IP) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ipStr := ip.String()
	delete(p.allocated, ipStr)
}

// IsAllocated checks if an IP address is allocated
func (p *IPPool) IsAllocated(ip net.IP) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.allocated[ip.String()]
}

// isBroadcast checks if an IP is the broadcast address of the network
func isBroadcast(ip net.IP, network *net.IPNet) bool {
	// IP address is a slice of bytes
	broadcast := make(net.IP, len(network.IP))
	copy(broadcast, network.IP)

	// Set all host bit to 1
	for i := 0; i < len(broadcast); i++ {
		broadcast[i] |= ^network.Mask[i]
	}

	return ip.Equal(broadcast)
}

// copyIP creates a copy of an IP address
func copyIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

// inc increments an IP address (the last byte)
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
