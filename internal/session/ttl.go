package session

import (
	"log/slog"
	"sync"
	"time"
)

// TTLManager fires a cleanup callback after a session has been disconnected
// longer than the configured duration.
type TTLManager struct {
	mu      sync.Mutex
	timers  map[string]*time.Timer
	ttl     time.Duration
	onEvict func(id string)
}

// NewTTLManager creates a TTLManager that calls onEvict(id) when a session expires.
func NewTTLManager(ttl time.Duration, onEvict func(id string)) *TTLManager {
	return &TTLManager{
		timers:  make(map[string]*time.Timer),
		ttl:     ttl,
		onEvict: onEvict,
	}
}

// StartTTL begins (or restarts) the TTL countdown for a session.
func (m *TTLManager) StartTTL(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.timers[id]; ok {
		t.Stop()
	}
	m.timers[id] = time.AfterFunc(m.ttl, func() {
		slog.Info("session TTL expired", "session_id", id)
		m.onEvict(id)
		m.mu.Lock()
		delete(m.timers, id)
		m.mu.Unlock()
	})
}

// CancelTTL stops the TTL countdown for a session (e.g. on reconnect).
// Returns true if the timer was stopped before it fired.
func (m *TTLManager) CancelTTL(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.timers[id]
	if !ok {
		return false
	}
	// Stop() returns false if the timer already fired.
	stopped := t.Stop()
	if stopped {
		delete(m.timers, id)
	}
	return stopped
}

// Remove unconditionally cancels and deletes any timer for the session.
func (m *TTLManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.timers[id]; ok {
		t.Stop()
		delete(m.timers, id)
	}
}
