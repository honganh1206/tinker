package storage

import "sync"

// Metrics tracks token usage across model calls.
type Metrics struct {
	mu    sync.Mutex
	total int
}

func (m *Metrics) Add(n int) {
	m.mu.Lock()
	m.total += n
	m.mu.Unlock()
}

func (m *Metrics) Total() int {
	m.mu.Lock()
	n := m.total
	m.mu.Unlock()
	return n
}
