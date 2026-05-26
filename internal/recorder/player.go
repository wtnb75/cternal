package recorder

import (
	"fmt"
	"io"
	"time"
)

// Player replays recorded events to an io.Writer with speed control.
type Player struct {
	events []Event
	speed  float64
}

// NewPlayer creates a Player from the given events.
// speed 1.0 is real-time; 2.0 is double speed; 0 defaults to 1.0.
func NewPlayer(events []Event, speed float64) *Player {
	if speed <= 0 {
		speed = 1.0
	}
	cp := make([]Event, len(events))
	copy(cp, events)
	return &Player{events: cp, speed: speed}
}

// Play writes all events to w at the configured playback speed.
// ctx cancellation is checked between events.
func (p *Player) Play(w io.Writer, done <-chan struct{}) error {
	var prev time.Duration
	for _, ev := range p.events {
		if ev.Type != EventOutput {
			prev = ev.Time
			continue
		}
		delay := time.Duration(float64(ev.Time-prev) / p.speed)
		if delay > 0 {
			select {
			case <-done:
				return nil
			case <-time.After(delay):
			}
		}
		if _, err := fmt.Fprint(w, ev.Data); err != nil {
			return err
		}
		prev = ev.Time
	}
	return nil
}

// PlayFrom returns a Player that starts from the event at seekIndex.
func NewPlayerFrom(events []Event, speed float64, seekIndex int) *Player {
	if seekIndex < 0 {
		seekIndex = 0
	}
	if seekIndex >= len(events) {
		seekIndex = len(events)
	}
	return NewPlayer(events[seekIndex:], speed)
}
