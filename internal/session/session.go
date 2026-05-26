package session

import (
	"sync"

	"github.com/wtnb75/cternal/internal/recorder"
	"github.com/wtnb75/cternal/internal/runtime"
)

// Mode indicates how the session connects to the container.
type Mode string

const (
	ModeExec   Mode = "exec"
	ModeAttach Mode = "attach"
	ModeLogs   Mode = "logs"
)

// Status indicates the lifecycle state of a session.
type Status string

const (
	StatusActive      Status = "active"
	StatusDisconnected Status = "disconnected"
)

// Subscription is returned by Subscribe and allows a caller to receive
// broadcast output and detect when the subscription has been cancelled.
type Subscription struct {
	// Ch delivers broadcast data to the subscriber.
	Ch chan []byte
	// Done is closed by Unsubscribe to signal the subscriber to stop.
	Done chan struct{}
}

// Session represents a single terminal connection to a container.
type Session struct {
	ID          string
	ContainerID string
	Mode        Mode
	Recorder    *recorder.Recorder
	Stream      runtime.Stream

	mu          sync.Mutex
	status      Status
	subscribers []*Subscription
}

// NewSession creates a session in the active state.
func NewSession(id, containerID string, mode Mode, stream runtime.Stream) *Session {
	return &Session{
		ID:          id,
		ContainerID: containerID,
		Mode:        mode,
		Recorder:    recorder.New(),
		Stream:      stream,
		status:      StatusActive,
	}
}

// GetStatus returns the current lifecycle status.
func (s *Session) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// SetStatus updates the lifecycle status.
func (s *Session) SetStatus(st Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = st
}

// Subscribe registers a new broadcast subscription.
// Call Unsubscribe when done to free resources.
func (s *Session) Subscribe() *Subscription {
	sub := &Subscription{
		Ch:   make(chan []byte, 64),
		Done: make(chan struct{}),
	}
	s.mu.Lock()
	s.subscribers = append(s.subscribers, sub)
	s.mu.Unlock()
	return sub
}

// Unsubscribe removes a subscription and closes its Done channel to signal the subscriber.
// The data channel (sub.Ch) is NOT closed here; callers must stop reading it after Done fires.
func (s *Session) Unsubscribe(sub *Subscription) {
	s.mu.Lock()
	for i, v := range s.subscribers {
		if v == sub {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	close(sub.Done)
}

// Broadcast sends data to all current subscriber channels, dropping if full.
func (s *Session) Broadcast(data []byte) {
	s.mu.Lock()
	subs := make([]*Subscription, len(s.subscribers))
	copy(subs, s.subscribers)
	s.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub.Ch <- data:
		default:
			// drop rather than block; slow consumers miss data
		}
	}
}

// SubscriberCount returns the current number of active subscribers.
func (s *Session) SubscriberCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subscribers)
}
