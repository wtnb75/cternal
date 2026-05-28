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
	"golang.org/x/term"
)

var recordCmd = &cobra.Command{
	Use:   "record <container-id>",
	Short: "Record a container session to an asciicast v3 file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runtimeName, _ := cmd.Flags().GetString("runtime")
		shell, _ := cmd.Flags().GetStringSlice("shell")
		output, _ := cmd.Flags().GetString("output")
		podmanHost, _ := cmd.Flags().GetString("podman-host")
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		idleLimit, _ := cmd.Flags().GetFloat64("idle-time-limit")
		return runRecord(args[0], runtimeName, podmanHost, kubeconfig, shell, output, idleLimit)
	},
}

func init() {
	recordCmd.Flags().String("runtime", "docker", "Container runtime (docker, podman, k8s)")
	recordCmd.Flags().StringSlice("shell", nil, "Shell command (default: /bin/sh)")
	recordCmd.Flags().String("output", "recording.cast", "Output file path")
	recordCmd.Flags().String("podman-host", "", "Podman socket URL")
	recordCmd.Flags().String("kubeconfig", "", "Path to kubeconfig file")
	recordCmd.Flags().Float64("idle-time-limit", 1.0, "Cap idle gaps in seconds (0 = no limit)")
	rootCmd.AddCommand(recordCmd)
}

func runRecord(containerID, runtimeName, podmanHost, kubeconfig string, shell []string, outputPath string, idleLimit float64) error {
	rt, err := newRuntime(runtimeName, podmanHost, kubeconfig)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Detect terminal size; fall back to 80×24.
	cols, rows := 80, 24
	if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && w > 0 {
		cols, rows = w, h
	}

	strm, err := rt.Exec(ctx, containerID, runtime.ExecOptions{
		Shell: shell,
		Cols:  uint16(cols),
		Rows:  uint16(rows),
	})
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	defer func() { _ = strm.Close() }()

	// Put stdin into raw mode so keystrokes are forwarded immediately.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
	}

	rec := recorder.New()
	fmt.Fprintf(os.Stderr, "Recording %s → %s  (Ctrl+C or Ctrl+D to stop)\r\n", containerID, outputPath)

	// Forward SIGWINCH (terminal resize) → container.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-winch:
				if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && w > 0 {
					rec.Add(recorder.EventResize, fmt.Sprintf("%dx%d", w, h))
					_ = strm.Resize(uint16(w), uint16(h))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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
		for {
			data, err := strm.Read()
			if len(data) > 0 {
				rec.Add(recorder.EventOutput, string(data))
				_, _ = io.Writer(os.Stdout).Write(data)
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	<-ctx.Done()

	// Restore terminal before writing the final message.
	if oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// Write cast file
	events := rec.All()
	data, err := recorder.Marshal(recorder.Header{Width: cols, Height: rows, IdleTimeLimit: idleLimit}, events)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("write cast: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\nSaved %d events to %s\n", len(events), outputPath)
	return nil
}
