package recorder

import (
	"sync"
	"time"
)

// Recorder accumulates terminal events during a live session.
type Recorder struct {
	mu     sync.RWMutex
	events []Event
	start  time.Time
}

// New creates a new Recorder with the clock started now.
func New() *Recorder {
	return &Recorder{start: time.Now()}
}

// Add appends an event with the elapsed time since recording start.
func (r *Recorder) Add(typ EventType, data string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, Event{
		Time: time.Since(r.start),
		Type: typ,
		Data: data,
	})
}

// EventsSince returns all events at index >= offset.
// Callers use this to fetch deltas during live streaming.
func (r *Recorder) EventsSince(offset int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if offset >= len(r.events) {
		return nil
	}
	// Return a copy to avoid races after unlock.
	result := make([]Event, len(r.events)-offset)
	copy(result, r.events[offset:])
	return result
}

// Len returns the current number of recorded events.
func (r *Recorder) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}

// All returns a copy of all recorded events.
func (r *Recorder) All() []Event {
	return r.EventsSince(0)
}

// AddAt records an event with an explicit elapsed duration instead of
// computing it from the wall clock.  Use this for externally timestamped
// sources such as "docker logs --timestamps" so that playback reflects
// the real inter-event gaps rather than the ingestion time.
func (r *Recorder) AddAt(typ EventType, data string, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, Event{
		Time: elapsed,
		Type: typ,
		Data: data,
	})
}
