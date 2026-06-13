package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/wtnb75/cternal/internal/runtime"
	"github.com/wtnb75/cternal/internal/session"
	"github.com/wtnb75/cternal/internal/webhook"
)

// Config holds the runtime configuration exposed via GET /api/v1/config.
type Config struct {
	Runtime     string `json:"runtime"`
	MaxSessions int    `json:"maxSessions"`
	BasePath    string `json:"basePath"`
	Version     string `json:"version"`
	Scrollback  int    `json:"scrollback,omitempty"`
	LogoutURL   string `json:"logoutUrl,omitempty"`

	// Not exposed via /api/v1/config.
	WebhookURLs []string `json:"-"`
	ExportURL   string   `json:"-"`
	UserHeader  string   `json:"-"`
}

// configResponse extends Config with the per-request username derived from
// UserHeader. It is only included in the JSON response (omitempty) when
// UserHeader is configured and present on the request.
type configResponse struct {
	Config
	Username string `json:"username,omitempty"`
}

type apiMetrics struct {
	activeSessions  metric.Int64UpDownCounter
	sessionDuration metric.Float64Histogram
	wsMessages      metric.Int64Counter
}

func initMetrics() apiMetrics {
	meter := otel.GetMeterProvider().Meter("cternal")
	active, _ := meter.Int64UpDownCounter("cternal.sessions.active",
		metric.WithDescription("Number of active sessions"))
	duration, _ := meter.Float64Histogram("cternal.sessions.duration_seconds",
		metric.WithDescription("Session duration in seconds"),
		metric.WithUnit("s"))
	msgs, _ := meter.Int64Counter("cternal.ws.messages_total",
		metric.WithDescription("Total WebSocket messages"))
	return apiMetrics{active, duration, msgs}
}

// Server wires together all HTTP handlers.
type Server struct {
	config    Config
	rt        runtime.Runtime
	store     *session.Store
	ttlMgr    *session.TTLManager
	mux       *http.ServeMux
	basePath  string
	notifier  *webhook.Notifier
	exportURL string
	metrics   apiMetrics
}

// NewServer creates a Server. basePath should be empty or start with "/".
func NewServer(cfg Config, rt runtime.Runtime, store *session.Store, ttlMgr *session.TTLManager) *Server {
	s := &Server{
		config:    cfg,
		rt:        rt,
		store:     store,
		ttlMgr:    ttlMgr,
		mux:       http.NewServeMux(),
		basePath:  cfg.BasePath,
		notifier:  webhook.New(cfg.WebhookURLs),
		exportURL: cfg.ExportURL,
	}
	s.metrics = initMetrics()
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	bp := s.basePath

	// Static frontend (SPA: falls back to index.html for unknown paths)
	s.mux.Handle(bp+"/", StaticHandler(bp))

	// REST API
	s.mux.HandleFunc(bp+"/api/v1/config", s.handleConfig)
	s.mux.HandleFunc(bp+"/api/v1/containers", s.handleContainers)
	s.mux.HandleFunc(bp+"/api/v1/sessions", s.handleSessions)
	s.mux.HandleFunc(bp+"/api/v1/sessions/", s.handleSession)

	// WebSocket
	s.mux.HandleFunc(bp+"/ws/{id}", s.handleWebSocket)
}

// Handler returns the http.Handler with access-log middleware applied.
func (s *Server) Handler() http.Handler {
	return accessLog(s.config.UserHeader, s.mux)
}

// Store returns the session store (used in tests).
func (s *Server) Store() *session.Store {
	return s.store
}

// EvictSession closes the session, fires the webhook, and runs auto-export.
// Called from the TTL manager callback when a session expires.
func (s *Server) EvictSession(id string) {
	sess, err := s.store.Get(id)
	if err != nil {
		return // already gone
	}
	s.evict(sess)
	slog.Info("session evicted by TTL", "id", id)
}

// evict performs the common teardown for both DELETE and TTL expiry.
func (s *Server) evict(sess *session.Session) {
	s.ttlMgr.Remove(sess.ID)
	if sess.Stream != nil {
		_ = sess.Stream.Close()
	}
	s.store.Delete(sess.ID)

	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String("mode", string(sess.Mode)),
		attribute.String("runtime", sess.Runtime),
	)
	s.metrics.activeSessions.Add(ctx, -1, attrs)
	if !sess.CreatedAt.IsZero() {
		s.metrics.sessionDuration.Record(ctx, time.Since(sess.CreatedAt).Seconds(), attrs)
	}

	s.notifier.Send(webhook.Payload{
		Event:     "session.end",
		SessionID: sess.ID,
		Container: sess.ContainerID,
		Mode:      string(sess.Mode),
	})
	s.autoExport(sess)
}

// autoExport marshals the session recording and PUTs it to ExportURL.
// Runs in a goroutine; failures are logged and never block the caller.
func (s *Server) autoExport(sess *session.Session) {
	if s.exportURL == "" {
		return
	}
	events := sess.Recorder.All()
	width, height := 80, 24
	for _, ev := range events {
		if ev.Type == "r" {
			var c, h int
			if _, err := fmt.Sscanf(ev.Data, "%dx%d", &c, &h); err == nil {
				width, height = c, h
			}
			break
		}
	}
	data, err := marshalCast(width, height, events)
	if err != nil {
		slog.Error("auto-export marshal", "id", sess.ID, "err", err)
		return
	}
	exportURL := s.exportURL
	go func() {
		spanCtx, span := otel.Tracer("cternal").Start(context.Background(), "recorder.export")
		defer span.End()
		span.SetAttributes(
			attribute.String("session.id", sess.ID),
			attribute.String("export.url", exportURL),
		)

		ctx, cancel := context.WithTimeout(spanCtx, 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, exportURL, bytes.NewReader(data))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.Error("auto-export request", "id", sess.ID, "err", err)
			return
		}
		req.Header.Set("Content-Type", "application/x-asciicast")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.Error("auto-export send", "id", sess.ID, "err", err)
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("status %d", resp.StatusCode))
			slog.Warn("auto-export response", "id", sess.ID, "status", resp.StatusCode)
		} else {
			slog.Info("auto-export success", "id", sess.ID, "url", exportURL)
		}
	}()
}

func accessLog(userHeader string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		args := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if userHeader != "" {
			args = append(args, "user", r.Header.Get(userHeader))
		}
		slog.Info("http", args...)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack forwards hijacking to the underlying ResponseWriter so that
// WebSocket upgrades work through the access-log middleware.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack: feature not supported")
	}
	return hj.Hijack()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resp := configResponse{Config: s.config}
	if s.config.UserHeader != "" {
		resp.Username = r.Header.Get(s.config.UserHeader)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	filter := runtime.Filter{
		Name:   q.Get("name"),
		Status: q.Get("status"),
	}
	// Support repeated ?label=key=val params.
	if lbls := q["label"]; len(lbls) > 0 {
		filter.Labels = make(map[string]string, len(lbls))
		for _, lbl := range lbls {
			if idx := strings.IndexByte(lbl, '='); idx >= 0 {
				filter.Labels[lbl[:idx]] = lbl[idx+1:]
			} else {
				filter.Labels[lbl] = ""
			}
		}
	}
	containers, err := s.rt.ListContainers(r.Context(), filter)
	if err != nil {
		slog.Error("list containers", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list containers")
		return
	}
	writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions := s.store.List()
		result := make([]sessionResponse, 0, len(sessions))
		for _, sess := range sessions {
			result = append(result, s.buildSessionResponse(sess, r))
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodPost:
		s.createSession(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type createSessionRequest struct {
	ContainerID   string       `json:"containerId"`
	ContainerName string       `json:"containerName"`
	Mode          session.Mode `json:"mode"`
	Shell         []string     `json:"shell"`
	Since         string       `json:"since"`
	Cols          uint16       `json:"cols"`
	Rows          uint16       `json:"rows"`
}

type sessionResponse struct {
	ID            string       `json:"id"`
	ContainerID   string       `json:"containerId"`
	ContainerName string       `json:"containerName,omitempty"`
	Runtime       string       `json:"runtime,omitempty"`
	Mode          session.Mode `json:"mode"`
	Status        string       `json:"status"`
	WSURL         string       `json:"wsUrl"`
	CreatedAt     string       `json:"createdAt,omitempty"`
	Cols          uint16       `json:"cols,omitempty"`
	Rows          uint16       `json:"rows,omitempty"`
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("cternal").Start(r.Context(), "session.create")
	defer span.End()

	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ContainerID == "" {
		writeError(w, http.StatusBadRequest, "containerId is required")
		return
	}
	if req.Mode == "" {
		req.Mode = session.ModeExec
	}
	span.SetAttributes(
		attribute.String("container.id", req.ContainerID),
		attribute.String("session.mode", string(req.Mode)),
	)

	// attach mode reuses an existing session for the same container
	if req.Mode == session.ModeAttach {
		if existing, err := s.store.GetByContainer(req.ContainerID, session.ModeAttach); err == nil {
			writeJSON(w, http.StatusOK, s.buildSessionResponse(existing, r))
			return
		}
	}

	var (
		strm runtime.Stream
		err  error
	)

	switch req.Mode {
	case session.ModeExec:
		_, rtSpan := otel.Tracer("cternal").Start(ctx, "runtime.exec")
		strm, err = s.rt.Exec(ctx, req.ContainerID, runtime.ExecOptions{
			Shell: req.Shell,
			Cols:  req.Cols,
			Rows:  req.Rows,
		})
		rtSpan.End()
	case session.ModeAttach:
		_, rtSpan := otel.Tracer("cternal").Start(ctx, "runtime.attach")
		strm, err = s.rt.Attach(ctx, req.ContainerID)
		rtSpan.End()
	case session.ModeLogs:
		// Logs mode uses a read-only stream; the WebSocket handler handles it separately.
		strm = nil
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown mode: %s", req.Mode))
		return
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.Error("create stream", "mode", req.Mode, "container", req.ContainerID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to connect: "+err.Error())
		return
	}

	id := generateID()
	user := ""
	if s.config.UserHeader != "" {
		user = r.Header.Get(s.config.UserHeader)
	}
	sess := session.NewSession(id, req.ContainerID, req.Mode, strm,
		session.WithContainerName(req.ContainerName),
		session.WithRuntime(s.config.Runtime),
		session.WithSize(req.Cols, req.Rows),
		session.WithUser(user),
	)

	if req.Mode == session.ModeLogs {
		sess.SetStatus(session.StatusDisconnected) // will be activated on WS connect
	}

	if err := s.store.Create(sess); err != nil {
		if strm != nil {
			_ = strm.Close()
		}
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	span.SetAttributes(attribute.String("session.id", id))
	s.metrics.activeSessions.Add(ctx, 1, metric.WithAttributes(
		attribute.String("mode", string(req.Mode)),
		attribute.String("runtime", s.config.Runtime),
	))

	logArgs := []any{"id", id, "container", req.ContainerID, "mode", req.Mode}
	if s.config.UserHeader != "" {
		logArgs = append(logArgs, "user", sess.User)
	}
	slog.Info("session created", logArgs...)
	s.notifier.Send(webhook.Payload{
		Event:     "session.start",
		SessionID: id,
		Container: req.ContainerID,
		Mode:      string(req.Mode),
	})
	writeJSON(w, http.StatusCreated, s.buildSessionResponse(sess, r))
}

func (s *Server) buildSessionResponse(sess *session.Session, r *http.Request) sessionResponse {
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s%s/ws/%s", scheme, r.Host, s.basePath, sess.ID)
	createdAt := ""
	if !sess.CreatedAt.IsZero() {
		createdAt = sess.CreatedAt.UTC().Format(time.RFC3339)
	}
	return sessionResponse{
		ID:            sess.ID,
		ContainerID:   sess.ContainerID,
		ContainerName: sess.ContainerName,
		Runtime:       sess.Runtime,
		Mode:          sess.Mode,
		Status:        string(sess.GetStatus()),
		WSURL:         wsURL,
		CreatedAt:     createdAt,
		Cols:          sess.Cols,
		Rows:          sess.Rows,
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Routes: /api/v1/sessions/{id} and /api/v1/sessions/{id}/cast and /api/v1/sessions/{id}/events
	path := r.URL.Path
	prefix := s.basePath + "/api/v1/sessions/"

	// strip prefix
	rest := path[len(prefix):]
	// rest is now either: "{id}", "{id}/cast", "{id}/events"

	var id, sub string
	for i, c := range rest {
		if c == '/' {
			id = rest[:i]
			sub = rest[i+1:]
			break
		}
	}
	if id == "" {
		id = rest
	}

	sess, err := s.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	switch sub {
	case "":
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, s.buildSessionResponse(sess, r))
		case http.MethodDelete:
			s.deleteSession(w, r, sess)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "cast":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleCast(w, r, sess)
	case "events":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, sess.Recorder.All())
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) deleteSession(w http.ResponseWriter, _ *http.Request, sess *session.Session) {
	s.evict(sess)
	slog.Info("session deleted", "id", sess.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCast(w http.ResponseWriter, _ *http.Request, sess *session.Session) {
	events := sess.Recorder.All()

	// Determine terminal size from resize events; default to 80x24.
	width, height := 80, 24
	for _, ev := range events {
		if ev.Type == "r" {
			// parse "COLSxROWS"
			var c, h int
			if _, err := fmt.Sscanf(ev.Data, "%dx%d", &c, &h); err == nil {
				width, height = c, h
			}
			break
		}
	}

	data, err := marshalCast(width, height, events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal cast")
		return
	}

	// Build a descriptive filename: <containerName>_<ISO8601>.cast
	name := sess.ContainerName
	if name == "" {
		if len(sess.ContainerID) >= 12 {
			name = sess.ContainerID[:12]
		} else {
			name = sess.ContainerID
		}
	}
	ts := ""
	if !sess.CreatedAt.IsZero() {
		ts = "_" + sess.CreatedAt.UTC().Format("20060102T150405Z")
	}
	filename := name + ts + ".cast"

	w.Header().Set("Content-Type", "application/x-asciicast")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(data)
}
