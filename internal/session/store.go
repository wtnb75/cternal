package session

import (
	"fmt"
	"sync"
)

// ErrSessionNotFound is returned when a session ID does not exist.
var ErrSessionNotFound = fmt.Errorf("session not found")

// ErrMaxSessions is returned when the session limit has been reached.
var ErrMaxSessions = fmt.Errorf("maximum number of sessions reached")

// Store is a thread-safe registry of active sessions.
type Store struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	maxSessions int
}

// NewStore creates a Store with the given session cap (0 = unlimited).
func NewStore(maxSessions int) *Store {
	return &Store{
		sessions:    make(map[string]*Session),
		maxSessions: maxSessions,
	}
}

// Create adds a session to the store.
func (s *Store) Create(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxSessions > 0 && len(s.sessions) >= s.maxSessions {
		return ErrMaxSessions
	}
	s.sessions[sess.ID] = sess
	return nil
}

// Get retrieves a session by ID.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

// Delete removes a session from the store.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// List returns all sessions.
func (s *Store) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, sess)
	}
	return result
}

// Len returns the number of active sessions.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// GetByContainer returns the first session matching the given container ID and mode.
// Returns ErrSessionNotFound if no match exists.
func (s *Store) GetByContainer(containerID string, mode Mode) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.sessions {
		if sess.ContainerID == containerID && sess.Mode == mode {
			return sess, nil
		}
	}
	return nil, ErrSessionNotFound
}
