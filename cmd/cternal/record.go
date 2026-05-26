package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/wtnb75/cternal/internal/recorder"
	"github.com/wtnb75/cternal/internal/runtime"
)

var recordCmd = &cobra.Command{
	Use:   "record <container-id>",
	Short: "Record a container session to an asciicast v3 file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runtimeName, _ := cmd.Flags().GetString("runtime")
		shell, _ := cmd.Flags().GetStringSlice("shell")
		output, _ := cmd.Flags().GetString("output")
		return runRecord(args[0], runtimeName, shell, output)
	},
}

func init() {
	recordCmd.Flags().String("runtime", "docker", "Container runtime (docker, podman, k8s)")
	recordCmd.Flags().StringSlice("shell", nil, "Shell command (default: /bin/sh)")
	recordCmd.Flags().String("output", "recording.cast", "Output file path")
	rootCmd.AddCommand(recordCmd)
}

func runRecord(containerID, runtimeName string, shell []string, outputPath string) error {
	rt, err := newRuntime(runtimeName)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	strm, err := rt.Exec(ctx, containerID, runtime.ExecOptions{
		Shell: shell,
		Cols:  80,
		Rows:  24,
	})
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	defer strm.Close()

	rec := recorder.New()
	fmt.Fprintf(os.Stderr, "Recording %s → %s  (Ctrl+C or Ctrl+D to stop)\n", containerID, outputPath)

	// Forward stdin → container
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				rec.Add(recorder.EventInput, string(buf[:n]))
				_ = strm.Write(buf[:n])
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Forward container → stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := strm.Read()
			if len(n) > 0 {
				rec.Add(recorder.EventOutput, string(n))
				_, _ = io.Writer(os.Stdout).Write(n)
			}
			if err != nil {
				cancel()
				return
			}
			_ = buf
		}
	}()

	<-ctx.Done()

	// Write cast file
	events := rec.All()
	data, err := recorder.Marshal(recorder.Header{Width: 80, Height: 24}, events)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("write cast: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\nSaved %d events to %s\n", len(events), outputPath)
	return nil
}
