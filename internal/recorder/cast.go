package recorder

import (
	"encoding/json"
	"fmt"
)

// Header holds the asciicast v3 file header.
type Header struct {
	Version   int    `json:"version"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Title     string `json:"title,omitempty"`
	Env       *Env   `json:"env,omitempty"`
}

// Env stores shell environment metadata in the asciicast header.
type Env struct {
	Shell string `json:"SHELL,omitempty"`
	Term  string `json:"TERM,omitempty"`
}

// Marshal serialises events to asciicast v3 JSON-lines format.
// Only "o" (output) events are included in the exported cast file per spec.
func Marshal(hdr Header, events []Event) ([]byte, error) {
	hdr.Version = 3

	headerBytes, err := json.Marshal(hdr)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}

	result := append(headerBytes, '\n')

	for _, ev := range events {
		if ev.Type != EventOutput {
			continue
		}
		seconds := ev.Time.Seconds()
		line, err := json.Marshal([]any{seconds, string(ev.Type), ev.Data})
		if err != nil {
			return nil, fmt.Errorf("marshal event: %w", err)
		}
		result = append(result, line...)
		result = append(result, '\n')
	}

	return result, nil
}
