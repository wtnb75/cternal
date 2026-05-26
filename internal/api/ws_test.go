package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/api"
	"github.com/wtnb75/cternal/internal/runtime"
)

func TestWebSocket_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/doesnotexist"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected connection to fail")
	}
	if resp != nil {
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
}

func TestWebSocket_execSession_output(t *testing.T) {
	srv, rt := newTestServer(t)

	ms := &runtime.MockStream{}
	ms.On("Read").Return([]byte("hello"), nil).Once()
	ms.On("Read").Return(nil, fmt.Errorf("EOF")).Maybe()
	ms.On("Close").Return(nil)

	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(string(body)))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/" + id
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(msg, &out))
	assert.Equal(t, "output", out["type"])
	assert.Equal(t, "hello", out["data"])

	rt.AssertExpectations(t)
}

func TestWebSocket_sendInput(t *testing.T) {
	srv, rt := newTestServer(t)

	inputReceived := make(chan string, 1)
	ms := &runtime.MockStream{}
	ms.On("Read").Return(nil, fmt.Errorf("EOF")).Maybe()
	ms.On("Write", mock.MatchedBy(func(b []byte) bool { return string(b) == "hello" })).
		Return(nil).
		Run(func(args mock.Arguments) {
			inputReceived <- string(args.Get(0).([]byte))
		}).Once()
	ms.On("Close").Return(nil)

	rt.On("Exec", anyCtx(), "ctr2", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr2",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(string(body)))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/" + id
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	inputMsg, _ := json.Marshal(api.InputMessage{Type: "input", Data: "hello"})
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, inputMsg))

	select {
	case got := <-inputReceived:
		assert.Equal(t, "hello", got)
	case <-time.After(3 * time.Second):
		t.Fatal("input not forwarded to stream")
	}
}

func TestWebSocket_attachSession(t *testing.T) {
	srv, rt := newTestServer(t)

	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Attach", anyCtx(), "ctr-attach").Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr-attach",
		"mode":        "attach",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(string(body)))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	// Get session to broadcast data to subscribers
	sessStore := srv.Store()
	require.NotNil(t, sessStore)
	sess, err := sessStore.Get(id)
	require.NoError(t, err)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/" + id
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Give WS time to subscribe, then broadcast data
	time.Sleep(50 * time.Millisecond)
	sess.Broadcast([]byte("broadcast-data"))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(msg, &out))
	assert.Equal(t, "output", out["type"])
	assert.Equal(t, "broadcast-data", out["data"])

	rt.AssertExpectations(t)
}

func TestWebSocket_logsSession(t *testing.T) {
	srv, rt := newTestServer(t)

	// Create a logs-mode session
	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr-logs",
		"mode":        "logs",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(string(body)))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	// Mock Logs to return "log output" then EOF
	rc := io.NopCloser(strings.NewReader("log output"))
	rt.On("Logs", anyCtx(), "ctr-logs",
		mock.MatchedBy(func(opts runtime.LogsOptions) bool { return opts.Follow })).
		Return(rc, nil)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/" + id
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(msg, &out))
	assert.Equal(t, "output", out["type"])
	assert.Contains(t, out["data"], "log output")

	rt.AssertExpectations(t)
}

func TestSessionResponse_subresourceNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/missing/unknown-sub", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestWebSocket_resizeMessage(t *testing.T) {
	srv, rt := newTestServer(t)

	ms := &runtime.MockStream{}
	ms.On("Read").Return(nil, fmt.Errorf("EOF")).Maybe()
	ms.On("Resize", uint16(120), uint16(30)).Return(nil).Once()
	ms.On("Close").Return(nil)

	rt.On("Exec", anyCtx(), "ctr3", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr3",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(string(body)))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/" + id
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	resizeMsg, _ := json.Marshal(api.ResizeMessage{Type: "resize", Cols: 120, Rows: 30})
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, resizeMsg))

	// Give some time for the message to be processed, then close
	time.Sleep(100 * time.Millisecond)
}
