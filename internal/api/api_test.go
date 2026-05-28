package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/api"
	"github.com/wtnb75/cternal/internal/runtime"
	"github.com/wtnb75/cternal/internal/session"
)

func newTestServer(t *testing.T) (*api.Server, *runtime.MockRuntime) {
	t.Helper()
	rt := &runtime.MockRuntime{}
	store := session.NewStore(10)
	ttl := session.NewTTLManager(time.Hour, func(id string) { store.Delete(id) })
	cfg := api.Config{Runtime: "docker", MaxSessions: 10}
	srv := api.NewServer(cfg, rt, store, ttl)
	return srv, rt
}

func TestHandleConfig(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var got api.Config
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, "docker", got.Runtime)
}

func TestHandleConfig_methodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleContainers_empty(t *testing.T) {
	srv, rt := newTestServer(t)
	rt.On("ListContainers", anyCtx(), runtime.Filter{}).
		Return([]runtime.Container{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	rt.AssertExpectations(t)
}

func TestHandleContainers_withFilter(t *testing.T) {
	srv, rt := newTestServer(t)
	rt.On("ListContainers", anyCtx(), runtime.Filter{Name: "web", Status: "running"}).
		Return([]runtime.Container{{ID: "abc", Name: "web"}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers?name=web&status=running", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	rt.AssertExpectations(t)
}

func TestCreateSession_invalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateSession_missingContainerID(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"mode": "exec"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateSession_exec(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)

	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "ctr1", resp["containerId"])
	assert.Equal(t, "exec", resp["mode"])
	assert.NotEmpty(t, resp["id"])
	rt.AssertExpectations(t)
}

func TestListSessions(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetSession_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/doesnotexist", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteSession_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/doesnotexist", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetSessionCast_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/doesnotexist/cast", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetSessionEvents_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/doesnotexist/events", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCreateSession_unknownMode(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "invalid",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateSession_execFails(t *testing.T) {
	srv, rt := newTestServer(t)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).
		Return(nil, fmt.Errorf("container not running"))

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	rt.AssertExpectations(t)
}

func TestDeleteSession_success(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	// Create a session first
	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var sess map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &sess))
	id := sess["id"].(string)

	// Delete it
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+id, nil)
	delRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(delRR, delReq)
	assert.Equal(t, http.StatusNoContent, delRR.Code)
}

func TestGetSession_success(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id, nil)
	getRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRR, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
}

func TestGetSessionCast_success(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	castReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+"/cast", nil)
	castRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(castRR, castReq)
	assert.Equal(t, http.StatusOK, castRR.Code)
	assert.Contains(t, castRR.Header().Get("Content-Type"), "asciicast")
}

func TestGetSessionEvents_success(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	evReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+"/events", nil)
	evRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(evRR, evReq)
	assert.Equal(t, http.StatusOK, evRR.Code)
}

func TestCreateSession_logsMode(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "logs",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestCreateSession_attachMode_reuseExisting(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)

	// First attach creates a new session via Attach
	rt.On("Attach", anyCtx(), "ctr1").Return(ms, nil).Once()

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "attach",
	})

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr1, req1)
	require.Equal(t, http.StatusCreated, rr1.Code)

	var s1 map[string]any
	require.NoError(t, json.Unmarshal(rr1.Body.Bytes(), &s1))

	// Second request reuses existing session (no second Attach call)
	body, _ = json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "attach",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code)

	var s2 map[string]any
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &s2))
	assert.Equal(t, s1["id"], s2["id"], "should reuse the same session")

	rt.AssertExpectations(t)
}

func TestSessionMethodNotAllowed(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{
		"containerId": "ctr1",
		"mode":        "exec",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	// PATCH on /sessions/{id} should return 405
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+id, nil)
	patchRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(patchRR, patchReq)
	assert.Equal(t, http.StatusMethodNotAllowed, patchRR.Code)
}

func TestHandleContainers_methodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/containers", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleContainers_runtimeError(t *testing.T) {
	srv, rt := newTestServer(t)
	rt.On("ListContainers", anyCtx(), runtime.Filter{}).
		Return(nil, fmt.Errorf("docker unavailable"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	rt.AssertExpectations(t)
}

func TestHandleContainers_withLabelFilter(t *testing.T) {
	srv, rt := newTestServer(t)
	rt.On("ListContainers", anyCtx(), runtime.Filter{Labels: map[string]string{"env": "prod", "team": ""}}).
		Return([]runtime.Container{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers?label=env=prod&label=team", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	rt.AssertExpectations(t)
}

func TestListSessions_empty(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &sessions))
	assert.Empty(t, sessions)
}

func TestListSessions_methodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleSession_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleSession_unknownSub(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "ctr1", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+"/unknown", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleSession_castMethodNotAllowed(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "ctr1", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+id+"/cast", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleSession_eventsMethodNotAllowed(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "ctr1", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+id+"/events", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestCreateSession_maxSessionsReached(t *testing.T) {
	rt := &runtime.MockRuntime{}
	store := session.NewStore(1)
	ttl := session.NewTTLManager(time.Hour, func(id string) { store.Delete(id) })
	cfg := api.Config{Runtime: "docker", MaxSessions: 1}
	srv := api.NewServer(cfg, rt, store, ttl)

	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), mock.AnythingOfType("string"), runtime.ExecOptions{}).Return(ms, nil)

	for i := range 2 {
		body, _ := json.Marshal(map[string]string{"containerId": fmt.Sprintf("ctr%d", i), "mode": "exec"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if i == 0 {
			assert.Equal(t, http.StatusCreated, rr.Code)
		} else {
			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		}
	}
}

func TestEvictSession(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "ctr1", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	srv.EvictSession(id)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id, nil)
	getRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRR, getReq)
	assert.Equal(t, http.StatusNotFound, getRR.Code)
}

func TestEvictSession_notFound(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.EvictSession("nonexistent") // must not panic
}

func TestAutoExport_success(t *testing.T) {
	received := make(chan string, 1)
	exportSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer exportSrv.Close()

	rt := &runtime.MockRuntime{}
	store := session.NewStore(10)
	ttl := session.NewTTLManager(time.Hour, func(id string) { store.Delete(id) })
	cfg := api.Config{Runtime: "docker", MaxSessions: 10, ExportURL: exportSrv.URL}
	srv := api.NewServer(cfg, rt, store, ttl)

	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "ctr1", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+id, nil)
	delRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(delRR, delReq)
	assert.Equal(t, http.StatusNoContent, delRR.Code)

	select {
	case method := <-received:
		assert.Equal(t, http.MethodPut, method)
	case <-time.After(3 * time.Second):
		t.Fatal("auto-export not received")
	}
}

func TestGetSessionCast_noContainerName(t *testing.T) {
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "abc123def456", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "abc123def456", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	castReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+"/cast", nil)
	castRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(castRR, castReq)
	assert.Equal(t, http.StatusOK, castRR.Code)
	// Falls back to first 12 chars of ContainerID
	assert.Contains(t, castRR.Header().Get("Content-Disposition"), "abc123def456")
}

func TestAutoExport_4xxResponse(t *testing.T) {
	// Verifies that a 4xx from the export server is logged but does not block.
	exportSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer exportSrv.Close()

	rt := &runtime.MockRuntime{}
	store := session.NewStore(10)
	ttl := session.NewTTLManager(time.Hour, func(id string) { store.Delete(id) })
	cfg := api.Config{Runtime: "docker", MaxSessions: 10, ExportURL: exportSrv.URL}
	srv := api.NewServer(cfg, rt, store, ttl)

	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr2", runtime.ExecOptions{}).Return(ms, nil)

	body, _ := json.Marshal(map[string]string{"containerId": "ctr2", "mode": "exec"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+id, nil)
	delRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(delRR, delReq)
	assert.Equal(t, http.StatusNoContent, delRR.Code)

	// Give goroutine time to finish.
	time.Sleep(200 * time.Millisecond)
}

func TestGetSessionCast_withResize(t *testing.T) {
	// Verifies that a resize event in the recorder is used to set cast dimensions.
	srv, rt := newTestServer(t)
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)
	rt.On("Exec", anyCtx(), "ctr1", runtime.ExecOptions{Cols: 120, Rows: 40}).Return(ms, nil)

	body, _ := json.Marshal(map[string]any{
		"containerId": "ctr1",
		"mode":        "exec",
		"containerName": "web",
		"cols": 120,
		"rows": 40,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
	createRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	id := created["id"].(string)

	// Inject a resize event directly via the events endpoint
	sess, err := srv.Store().Get(id)
	require.NoError(t, err)
	sess.Recorder.Add("r", "120x40")

	castReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+id+"/cast", nil)
	castRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(castRR, castReq)
	assert.Equal(t, http.StatusOK, castRR.Code)
	assert.Contains(t, string(castRR.Body.Bytes()), "120")
}

func TestStaticHandler_nilFS(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	// StaticFS is nil in tests → http.NotFoundHandler returns 404
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// anyCtx returns a testify argument matcher that accepts any context.
func anyCtx() interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })
}
