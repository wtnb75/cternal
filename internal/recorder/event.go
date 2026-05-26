package recorder

import "time"

// EventType represents the type of a recorded event (asciicast v3).
type EventType string

const (
	EventOutput EventType = "o" // terminal output
	EventInput  EventType = "i" // user input
	EventResize EventType = "r" // terminal resize
)

// Event is a single recorded terminal event.
type Event struct {
	Time      time.Duration // time since recording start
	Type      EventType
	Data      string
}
