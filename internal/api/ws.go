package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wtnb75/cternal/internal/recorder"
	"github.com/wtnb75/cternal/internal/runtime"
	"github.com/wtnb75/cternal/internal/session"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	sess, err := s.store.Get(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade", "err", err)
		return
	}
	defer conn.Close()

	// Cancel any pending TTL countdown (reconnect wins over expiry).
	s.ttlMgr.CancelTTL(id)
	sess.SetStatus(session.StatusActive)

	slog.Info("ws connected", "session_id", id, "mode", sess.Mode)

	// Send buffered events so the client catches up after reconnect.
	s.flushEvents(conn, sess)

	switch sess.Mode {
	case session.ModeExec:
		s.runExecWS(conn, sess)
	case session.ModeAttach:
		s.runAttachWS(conn, sess)
	case session.ModeLogs:
		s.runLogsWS(conn, r, sess)
	}

	sess.SetStatus(session.StatusDisconnected)
	s.ttlMgr.StartTTL(id)
	slog.Info("ws disconnected", "session_id", id)
}

// flushEvents sends all recorded output events to the new WS client.
func (s *Server) flushEvents(conn *websocket.Conn, sess *session.Session) {
	for _, ev := range sess.Recorder.All() {
		if ev.Type != recorder.EventOutput {
			continue
		}
		msg := OutputMessage{Type: "output", Data: ev.Data}
		if err := writeWS(conn, msg); err != nil {
			return
		}
	}
}

// runExecWS drives an exec-mode session: bidirectional PTY I/O.
func (s *Server) runExecWS(conn *websocket.Conn, sess *session.Session) {
	writeCh := make(chan []byte, 64)
	done := make(chan struct{})

	// Write goroutine: serialise all writes to the WebSocket.
	go func() {
		defer close(done)
		for data := range writeCh {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	// Stream reader: forward container output → WS.
	go func() {
		for {
			data, err := sess.Stream.Read()
			if err != nil {
				if err != io.EOF {
					slog.Debug("stream read", "err", err)
				}
				break
			}
			sess.Recorder.Add(recorder.EventOutput, string(data))
			sess.Broadcast(data)
			msg, _ := json.Marshal(OutputMessage{Type: "output", Data: string(data)})
			select {
			case writeCh <- msg:
			case <-done:
				return
			}
		}
		close(writeCh)
	}()

	// Main loop: read WS messages → container.
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s.dispatchMessage(raw, sess)
	}

	<-done
}

// runAttachWS drives an attach-mode session: subscribe to broadcast output.
func (s *Server) runAttachWS(conn *websocket.Conn, sess *session.Session) {
	sub := sess.Subscribe()
	defer sess.Unsubscribe(sub)

	writeCh := make(chan []byte, 64)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for data := range writeCh {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	go func() {
		defer close(writeCh)
		for {
			select {
			case data := <-sub.Ch:
				msg, _ := json.Marshal(OutputMessage{Type: "output", Data: string(data)})
				select {
				case writeCh <- msg:
				case <-done:
					return
				}
			case <-sub.Done:
				return
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s.dispatchMessage(raw, sess)
	}

	<-done
}

// runLogsWS drives a logs-mode session: stream container logs read-only.
func (s *Server) runLogsWS(conn *websocket.Conn, r *http.Request, sess *session.Session) {
	// Retrieve the since parameter from the session's recorded events, or from creation context.
	// For logs mode we open a new log stream at WS connect time.
	logsRC, err := s.rt.Logs(r.Context(), sess.ContainerID, runtime.LogsOptions{Follow: true})
	if err != nil {
		_ = writeWS(conn, ErrorMessage{Type: "error", Message: "failed to open logs: " + err.Error()})
		return
	}
	defer logsRC.Close()

	writeCh := make(chan []byte, 64)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for data := range writeCh {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	go func() {
		defer close(writeCh)
		buf := make([]byte, 4096)
		for {
			n, err := logsRC.Read(buf)
			if n > 0 {
				sess.Recorder.Add(recorder.EventOutput, string(buf[:n]))
				msg, _ := json.Marshal(OutputMessage{Type: "output", Data: string(buf[:n])})
				select {
				case writeCh <- msg:
				case <-done:
					return
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Logs mode is read-only; still drain WS messages (e.g. ping).
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s.dispatchMessage(raw, sess)
	}

	<-done
}

func (s *Server) dispatchMessage(raw []byte, sess *session.Session) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return
	}
	switch base.Type {
	case "input":
		var msg InputMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		sess.Recorder.Add(recorder.EventInput, msg.Data)
		if sess.Stream != nil {
			_ = sess.Stream.Write([]byte(msg.Data))
		}
	case "resize":
		var msg ResizeMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		data := fmt.Sprintf("%dx%d", msg.Cols, msg.Rows)
		sess.Recorder.Add(recorder.EventResize, data)
		if sess.Stream != nil {
			_ = sess.Stream.Resize(msg.Cols, msg.Rows)
		}
	}
}

func writeWS(conn *websocket.Conn, v any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}
