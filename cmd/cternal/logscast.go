package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/wtnb75/cternal/internal/recorder"
	"github.com/wtnb75/cternal/internal/runtime"
)

func init() {
	f := logscastCmd.Flags()
	f.String("runtime", "docker", "Container runtime (docker, podman, k8s)")
	f.String("podman-host", "", "Podman socket URL")
	f.String("kubeconfig", "", "Path to kubeconfig file")
	f.String("output", "logs.cast", "Output file path")
	f.String("since", "", "Show logs since timestamp or duration (e.g. 1h, 2006-01-02T15:04:05Z)")
	f.Bool("follow", false, "Follow log output (stop with Ctrl+C)")
	f.Float64("idle-time-limit", 1.0, "Cap idle gaps in seconds (0 = no limit)")
	rootCmd.AddCommand(logscastCmd)
}

var logscastCmd = &cobra.Command{
	Use:   "logscast <container-id>",
	Short: "Convert container logs to an asciicast file",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogscast,
}

func runLogscast(cmd *cobra.Command, args []string) error {
	runtimeName, _ := cmd.Flags().GetString("runtime")
	podmanHost, _ := cmd.Flags().GetString("podman-host")
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	outputPath, _ := cmd.Flags().GetString("output")
	since, _ := cmd.Flags().GetString("since")
	follow, _ := cmd.Flags().GetBool("follow")
	idleLimit, _ := cmd.Flags().GetFloat64("idle-time-limit")

	rt, err := newRuntime(runtimeName, podmanHost, kubeconfig)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}

	ctx, cancel := context.Background(), func() {}
	if follow {
		ctx, cancel = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	}
	defer cancel()

	logsRC, err := rt.Logs(ctx, args[0], runtime.LogsOptions{
		Since:      since,
		Follow:     follow,
		Timestamps: true,
	})
	if err != nil {
		return fmt.Errorf("logs: %w", err)
	}
	defer func() { _ = logsRC.Close() }()

	rec := recorder.New()
	var epoch time.Time

	sc := bufio.NewScanner(logsRC)
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	for sc.Scan() {
		line := sc.Text()

		display := line
		var elapsed time.Duration
		if idx := strings.IndexByte(line, ' '); idx > 0 {
			if ts, parseErr := time.Parse(time.RFC3339Nano, line[:idx]); parseErr == nil {
				if epoch.IsZero() {
					epoch = ts
				}
				elapsed = ts.Sub(epoch)
				display = line[idx+1:]
			}
		}

		// Restore the newline (scanner stripped it) and convert bare LF → CRLF.
		data := string(normalizeLFBytes([]byte(display + "\n")))

		if !epoch.IsZero() {
			rec.AddAt(recorder.EventOutput, data, elapsed)
		} else {
			rec.Add(recorder.EventOutput, data)
		}
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("read logs: %w", err)
	}

	events := rec.All()
	hdr := recorder.Header{
		Width:         220,
		Height:        50,
		IdleTimeLimit: idleLimit,
	}
	castData, err := recorder.Marshal(hdr, events)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(outputPath, castData, 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved %d events to %s\n", len(events), outputPath)
	return nil
}

func normalizeLFBytes(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}
