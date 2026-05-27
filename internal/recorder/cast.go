package recorder

import (
	"encoding/json"
	"fmt"
	"time"
)

// Header holds the asciicast v2 file header.
type Header struct {
	Version       int     `json:"version"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Timestamp     int64   `json:"timestamp,omitempty"`
	Title         string  `json:"title,omitempty"`
	IdleTimeLimit float64 `json:"idle_time_limit,omitempty"`
	Env           *Env    `json:"env,omitempty"`
}

// Env stores shell environment metadata in the asciicast header.
type Env struct {
	Shell string `json:"SHELL,omitempty"`
	Term  string `json:"TERM,omitempty"`
}

// Marshal serialises events to asciicast v2 JSON-lines format.
// Only "o" (output) events are included in the exported cast file per spec.
//
// When hdr.IdleTimeLimit > 0, any gap between consecutive events that exceeds
// that duration is clamped to it, shortening idle pauses during playback.
// The same limit applies to the initial gap before the first event.
func Marshal(hdr Header, events []Event) ([]byte, error) {
	hdr.Version = 2

	headerBytes, err := json.Marshal(hdr)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}

	result := append(headerBytes, '\n')

	var limit time.Duration
	if hdr.IdleTimeLimit > 0 {
		limit = time.Duration(hdr.IdleTimeLimit * float64(time.Second))
	}

	var cursor time.Duration  // adjusted elapsed time written so far
	var prevRaw time.Duration // raw timestamp of the previous output event

	for _, ev := range events {
		if ev.Type != EventOutput {
			continue
		}

		gap := ev.Time - prevRaw
		if limit > 0 && gap > limit {
			gap = limit
		}
		cursor += gap
		prevRaw = ev.Time

		line, err := json.Marshal([]any{cursor.Seconds(), string(ev.Type), ev.Data})
		if err != nil {
			return nil, fmt.Errorf("marshal event: %w", err)
		}
		result = append(result, line...)
		result = append(result, '\n')
	}

	return result, nil
}
