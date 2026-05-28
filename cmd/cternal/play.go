package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/wtnb75/cternal/internal/recorder"
	"golang.org/x/term"
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
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		close(done)
	}()

	// Raw mode suppresses terminal echo of control-sequence responses (e.g. CPR
	// replies to ESC[6n that are embedded in recorded output).
	stdinFd := int(os.Stdin.Fd())
	if term.IsTerminal(stdinFd) {
		oldState, err := term.MakeRaw(stdinFd)
		if err == nil {
			defer func() { _ = term.Restore(stdinFd, oldState) }()
		}
		// Drain any bytes the terminal sends back (e.g. CPR responses) so they
		// don't accumulate in the input buffer and appear after playback ends.
		go func() {
			buf := make([]byte, 256)
			for {
				select {
				case <-done:
					return
				default:
					_, _ = os.Stdin.Read(buf)
				}
			}
		}()
	}

	for {
		p := recorder.NewPlayer(events, speed)
		if err := p.Play(os.Stdout, done); err != nil {
			return err
		}
		select {
		case <-done:
			return nil
		default:
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
