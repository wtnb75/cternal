package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/wtnb75/cternal/internal/runtime"
	"github.com/wtnb75/cternal/internal/session"
)

// Config holds the runtime configuration exposed via GET /api/v1/config.
type Config struct {
	Runtime     string `json:"runtime"`
	MaxSessions int    `json:"maxSessions"`
	BasePath    string `json:"basePath"`
	Version     string `json:"version"`
}

// Server wires together all HTTP handlers.
type Server struct {
	config   Config
	rt       runtime.Runtime
	store    *session.Store
	ttlMgr   *session.TTLManager
	mux      *http.ServeMux
	basePath string
}

// NewServer creates a Server. basePath should be empty or start with "/".
func NewServer(cfg Config, rt runtime.Runtime, store *session.Store, ttlMgr *session.TTLManager) *Server {
	s := &Server{
		config:   cfg,
		rt:       rt,
		store:    store,
		ttlMgr:   ttlMgr,
		mux:      http.NewServeMux(),
		basePath: cfg.BasePath,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	bp := s.basePath

	// Static frontend
	s.mux.Handle(bp+"/", StaticHandler())

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
	return accessLog(s.mux)
}

// Store returns the session store (used in tests).
func (s *Server) Store() *session.Store {
	return s.store
}

func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
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
	writeJSON(w, http.StatusOK, s.config)
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
		writeJSON(w, http.StatusOK, s.store.List())
	case http.MethodPost:
		s.createSession(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type createSessionRequest struct {
	ContainerID string          `json:"containerId"`
	Mode        session.Mode    `json:"mode"`
	Shell       []string        `json:"shell"`
	Since       string          `json:"since"`
	Cols        uint16          `json:"cols"`
	Rows        uint16          `json:"rows"`
}

type createSessionResponse struct {
	ID          string       `json:"id"`
	ContainerID string       `json:"containerId"`
	Mode        session.Mode `json:"mode"`
	Status      string       `json:"status"`
	WSURL       string       `json:"wsUrl"`
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
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

	// attach mode reuses an existing session for the same container
	if req.Mode == session.ModeAttach {
		if existing, err := s.store.GetByContainer(req.ContainerID, session.ModeAttach); err == nil {
			writeJSON(w, http.StatusOK, s.sessionResponse(existing, r))
			return
		}
	}

	var (
		strm runtime.Stream
		err  error
	)

	switch req.Mode {
	case session.ModeExec:
		strm, err = s.rt.Exec(r.Context(), req.ContainerID, runtime.ExecOptions{
			Shell: req.Shell,
			Cols:  req.Cols,
			Rows:  req.Rows,
		})
	case session.ModeAttach:
		strm, err = s.rt.Attach(r.Context(), req.ContainerID)
	case session.ModeLogs:
		// Logs mode uses a read-only stream; the WebSocket handler handles it separately.
		strm = nil
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown mode: %s", req.Mode))
		return
	}

	if err != nil {
		slog.Error("create stream", "mode", req.Mode, "container", req.ContainerID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to connect: "+err.Error())
		return
	}

	id := generateID()
	sess := session.NewSession(id, req.ContainerID, req.Mode, strm)

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

	slog.Info("session created", "id", id, "container", req.ContainerID, "mode", req.Mode)
	writeJSON(w, http.StatusCreated, s.sessionResponse(sess, r))
}

func (s *Server) sessionResponse(sess *session.Session, r *http.Request) createSessionResponse {
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s%s/ws/%s", scheme, r.Host, s.basePath, sess.ID)
	return createSessionResponse{
		ID:          sess.ID,
		ContainerID: sess.ContainerID,
		Mode:        sess.Mode,
		Status:      string(sess.GetStatus()),
		WSURL:       wsURL,
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
			writeJSON(w, http.StatusOK, s.sessionResponse(sess, r))
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
	s.ttlMgr.Remove(sess.ID)
	s.store.Delete(sess.ID)
	if sess.Stream != nil {
		_ = sess.Stream.Close()
	}
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
	w.Header().Set("Content-Type", "application/x-asciicast")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.cast"`, sess.ID))
	_, _ = w.Write(data)
}
