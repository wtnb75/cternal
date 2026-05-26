package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wtnb75/cternal/internal/recorder"
)

var playCmd = &cobra.Command{
	Use:   "play <file.cast>",
	Short: "Play back a recorded asciicast v3 file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		speed, _ := cmd.Flags().GetFloat64("speed")
		loop, _ := cmd.Flags().GetBool("loop")
		return runPlay(args[0], speed, loop)
	},
}

func init() {
	playCmd.Flags().Float64("speed", 1.0, "Playback speed multiplier")
	playCmd.Flags().Bool("loop", false, "Loop the playback")
	rootCmd.AddCommand(playCmd)
}

func runPlay(path string, speed float64, loop bool) error {
	events, err := loadCast(path)
	if err != nil {
		return fmt.Errorf("load cast: %w", err)
	}

	done := make(chan struct{})

	for {
		p := recorder.NewPlayer(events, speed)
		if err := p.Play(os.Stdout, done); err != nil {
			return err
		}
		if !loop {
			break
		}
	}
	return nil
}

func loadCast(path string) ([]recorder.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var events []recorder.Event
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			// Skip the header line
			first = false
			continue
		}
		// Parse event: [timestamp, type, data]
		var row []json.RawMessage
		if err := json.Unmarshal([]byte(line), &row); err != nil || len(row) != 3 {
			continue
		}
		var ts float64
		var typ, data string
		if err := json.Unmarshal(row[0], &ts); err != nil {
			continue
		}
		if err := json.Unmarshal(row[1], &typ); err != nil {
			continue
		}
		if err := json.Unmarshal(row[2], &data); err != nil {
			continue
		}
		events = append(events, recorder.Event{
			Time: time.Duration(ts * float64(time.Second)),
			Type: recorder.EventType(typ),
			Data: data,
		})
	}
	return events, scanner.Err()
}
