package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wtnb75/cternal/internal/api"
	"github.com/wtnb75/cternal/internal/runtime"
	"github.com/wtnb75/cternal/internal/session"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Default to "serve" when no subcommand is given (or only flags are passed).
	if len(os.Args) == 1 || (len(os.Args) > 1 && len(os.Args[1]) > 0 && os.Args[1][0] == '-') {
		os.Args = append([]string{os.Args[0], "serve"}, os.Args[1:]...)
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cternal",
	Short: "Web-based container terminal with session recording",
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Generate shell completion script",
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}
		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the cternal HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe(cmd)
	},
}

func init() {
	serveCmd.Flags().String("host", "0.0.0.0", "Host to listen on")
	serveCmd.Flags().Int("port", 3000, "Port to listen on")
	serveCmd.Flags().String("base-path", "", "Base path prefix for all routes")
	serveCmd.Flags().String("runtime", "docker", "Container runtime (docker, podman, k8s)")
	serveCmd.Flags().Int("max-sessions", 100, "Maximum number of concurrent sessions")
	serveCmd.Flags().Int("session-ttl", 3600, "Session TTL in seconds after last disconnect")
	serveCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	serveCmd.Flags().String("log-format", "text", "Log format (text, json)")
	serveCmd.Flags().Int("scrollback", 5000, "Terminal scrollback buffer lines")
	serveCmd.Flags().StringArray("webhook-url", nil, "Webhook URL(s) for session events (repeatable)")
	serveCmd.Flags().String("export-url", "", "HTTP PUT endpoint for auto-exporting .cast files on session end")
}

func runServe(cmd *cobra.Command) error {
	logLevel, _ := cmd.Flags().GetString("log-level")
	logFormat, _ := cmd.Flags().GetString("log-format")
	setupLogger(logLevel, logFormat)

	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetInt("port")
	basePath, _ := cmd.Flags().GetString("base-path")
	runtimeName, _ := cmd.Flags().GetString("runtime")
	maxSessions, _ := cmd.Flags().GetInt("max-sessions")
	sessionTTL, _ := cmd.Flags().GetInt("session-ttl")
	scrollback, _ := cmd.Flags().GetInt("scrollback")
	webhookURLs, _ := cmd.Flags().GetStringArray("webhook-url")
	exportURL, _ := cmd.Flags().GetString("export-url")

	// Also accept comma-separated webhook URLs from a single flag value.
	var flatWebhookURLs []string
	for _, raw := range webhookURLs {
		for _, u := range strings.Split(raw, ",") {
			if u = strings.TrimSpace(u); u != "" {
				flatWebhookURLs = append(flatWebhookURLs, u)
			}
		}
	}

	rt, err := newRuntime(runtimeName)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}

	store := session.NewStore(maxSessions)
	ttlDur := time.Duration(sessionTTL) * time.Second

	// srv is assigned after TTLManager is created; the closure captures it by
	// reference so EvictSession is available when the first TTL fires.
	var srv *api.Server
	ttlMgr := session.NewTTLManager(ttlDur, func(id string) {
		if srv != nil {
			srv.EvictSession(id)
		}
	})

	cfg := api.Config{
		Runtime:     runtimeName,
		MaxSessions: maxSessions,
		BasePath:    basePath,
		Version:     Version,
		Scrollback:  scrollback,
		WebhookURLs: flatWebhookURLs,
		ExportURL:   exportURL,
	}

	srv = api.NewServer(cfg, rt, store, ttlMgr)
	addr := fmt.Sprintf("%s:%d", host, port)
	slog.Info("cternal starting", "addr", addr, "runtime", runtimeName, "version", Version)

	return http.ListenAndServe(addr, srv.Handler())
}

func newRuntime(name string) (runtime.Runtime, error) {
	switch name {
	case "docker", "":
		return runtime.NewDockerRuntime()
	case "podman":
		return runtime.NewPodmanRuntime()
	case "k8s", "kubernetes":
		return runtime.NewK8sRuntime("")
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", name)
	}
}

func setupLogger(level, format string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cternal %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}
