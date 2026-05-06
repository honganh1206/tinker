package storage

import "sync"

// Metrics tracks token usage across model calls.
type Metrics struct {
	mu    sync.Mutex
	total int
}

func (m *Metrics) Add(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.total += n
}

func (m *Metrics) Total() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.total
	return n
}
