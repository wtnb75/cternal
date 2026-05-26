package api_test

import (
	"bytes"
	"context"
	"encoding/json"
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

// anyCtx returns a testify argument matcher that accepts any context.
func anyCtx() interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })
}
