package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wtnb75/cternal/internal/api"
	"github.com/wtnb75/cternal/internal/runtime"
	"github.com/wtnb75/cternal/internal/session"
	"github.com/wtnb75/cternal/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	// CTERNAL_* env vars override defaults for every serve flag.
	viper.SetEnvPrefix("CTERNAL")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	f := serveCmd.Flags()
	f.String("addr", ":3000", "Address to listen on (host:port)")
	f.String("base-path", "", "Base path prefix for all routes")
	f.String("runtime", "docker", "Container runtime (docker, podman, k8s)")
	f.Int("max-sessions", 100, "Maximum number of concurrent sessions")
	f.Int("session-ttl", 3600, "Session TTL in seconds after last disconnect")
	f.String("log-level", "info", "Log level (debug, info, warn, error)")
	f.String("log-format", "text", "Log format (text, json)")
	f.Int("scrollback", 5000, "Terminal scrollback buffer lines")
	f.StringArray("webhook-url", nil, "Webhook URL(s) for session events (repeatable)")
	f.String("export-url", "", "HTTP PUT endpoint for auto-exporting .cast files on session end")
	f.String("podman-host", "", "Podman socket URL (e.g. unix:///run/user/1000/podman/podman.sock)")
	f.String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config)")
	f.String("user-header", "", "HTTP header name to use as the login username (e.g. X-Remote-User); empty disables the feature")
	f.String("logout-url", "", "URL for the logout link shown in the UI; empty hides the link")

	// Bind simple flags so viper resolves: explicit flag > CTERNAL_* env var > default.
	for _, name := range []string{
		"addr", "base-path", "runtime", "max-sessions", "session-ttl",
		"log-level", "log-format", "scrollback", "export-url",
		"podman-host", "kubeconfig", "user-header", "logout-url",
	} {
		_ = viper.BindPFlag(name, f.Lookup(name))
	}
	// webhook-url (StringArray) is merged with CTERNAL_WEBHOOK_URL manually in runServe.
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

func runServe(cmd *cobra.Command) error {
	setupLogger(viper.GetString("log-level"), viper.GetString("log-format"))

	ctx := context.Background()
	prov, err := telemetry.Init(ctx, Version)
	if err != nil {
		slog.Warn("telemetry init failed, continuing without OTel", "err", err)
	} else {
		defer prov.Shutdown(ctx)
	}

	addr := viper.GetString("addr")
	basePath := viper.GetString("base-path")
	runtimeName := viper.GetString("runtime")
	maxSessions := viper.GetInt("max-sessions")
	sessionTTL := viper.GetInt("session-ttl")
	scrollback := viper.GetInt("scrollback")
	exportURL := viper.GetString("export-url")
	userHeader := viper.GetString("user-header")
	logoutURL := viper.GetString("logout-url")

	// Merge --webhook-url flag values with CTERNAL_WEBHOOK_URL env var.
	// Both support comma-separated lists.
	flagURLs, _ := cmd.Flags().GetStringArray("webhook-url")
	rawURLs := append(flagURLs, strings.Split(os.Getenv("CTERNAL_WEBHOOK_URL"), ",")...)
	var webhookURLs []string
	for _, raw := range rawURLs {
		for _, u := range strings.Split(raw, ",") {
			if u = strings.TrimSpace(u); u != "" {
				webhookURLs = append(webhookURLs, u)
			}
		}
	}

	podmanHost := viper.GetString("podman-host")
	kubeconfig := viper.GetString("kubeconfig")
	rt, err := newRuntime(runtimeName, podmanHost, kubeconfig)
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
		WebhookURLs: webhookURLs,
		ExportURL:   exportURL,
		UserHeader:  userHeader,
		LogoutURL:   logoutURL,
	}

	srv = api.NewServer(cfg, rt, store, ttlMgr)
	slog.Info("cternal starting", "addr", addr, "runtime", runtimeName, "version", Version)

	handler := otelhttp.NewHandler(srv.Handler(), "cternal",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)
	return http.ListenAndServe(addr, handler)
}

func newRuntime(name, podmanHost, kubeconfig string) (runtime.Runtime, error) {
	switch name {
	case "docker", "":
		return runtime.NewDockerRuntime()
	case "podman":
		return runtime.NewPodmanRuntime(podmanHost)
	case "k8s", "kubernetes":
		return runtime.NewK8sRuntime(kubeconfig, "")
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
