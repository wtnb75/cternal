package runtime_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/runtime"
)

func TestMockRuntime_ListContainers_empty(t *testing.T) {
	m := &runtime.MockRuntime{}
	m.On("ListContainers", context.Background(), runtime.Filter{}).
		Return([]runtime.Container{}, nil)

	got, err := m.ListContainers(context.Background(), runtime.Filter{})
	require.NoError(t, err)
	assert.Empty(t, got)
	m.AssertExpectations(t)
}

func TestMockRuntime_ListContainers_oneContainer(t *testing.T) {
	m := &runtime.MockRuntime{}
	want := []runtime.Container{{ID: "abc123", Name: "mycontainer", Running: true}}
	m.On("ListContainers", context.Background(), runtime.Filter{Status: "running"}).
		Return(want, nil)

	got, err := m.ListContainers(context.Background(), runtime.Filter{Status: "running"})
	require.NoError(t, err)
	assert.Equal(t, want, got)
	m.AssertExpectations(t)
}

func TestMockRuntime_ListContainers_filterNoMatch(t *testing.T) {
	m := &runtime.MockRuntime{}
	m.On("ListContainers", context.Background(), runtime.Filter{Name: "nonexistent"}).
		Return([]runtime.Container(nil), nil)

	got, err := m.ListContainers(context.Background(), runtime.Filter{Name: "nonexistent"})
	require.NoError(t, err)
	assert.Nil(t, got)
	m.AssertExpectations(t)
}

func TestMockRuntime_Exec_success(t *testing.T) {
	m := &runtime.MockRuntime{}
	ms := &runtime.MockStream{}
	ms.On("Close").Return(nil)

	opts := runtime.ExecOptions{Shell: []string{"/bin/bash"}}
	m.On("Exec", context.Background(), "ctr1", opts).Return(ms, nil)

	stream, err := m.Exec(context.Background(), "ctr1", opts)
	require.NoError(t, err)
	assert.NotNil(t, stream)
	assert.NoError(t, stream.Close())
	m.AssertExpectations(t)
	ms.AssertExpectations(t)
}

func TestMockRuntime_Exec_containerNotRunning(t *testing.T) {
	m := &runtime.MockRuntime{}
	m.On("Exec", context.Background(), "stopped", runtime.ExecOptions{}).
		Return(nil, errors.New("container is not running"))

	_, err := m.Exec(context.Background(), "stopped", runtime.ExecOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
	m.AssertExpectations(t)
}

func TestMockRuntime_Exec_contextCancelled(t *testing.T) {
	m := &runtime.MockRuntime{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m.On("Exec", ctx, "ctr1", runtime.ExecOptions{}).
		Return(nil, context.Canceled)

	_, err := m.Exec(ctx, "ctr1", runtime.ExecOptions{})
	assert.ErrorIs(t, err, context.Canceled)
	m.AssertExpectations(t)
}

func TestMockRuntime_Logs(t *testing.T) {
	m := &runtime.MockRuntime{}
	content := "log line 1\nlog line 2\n"
	rc := io.NopCloser(strings.NewReader(content))
	m.On("Logs", context.Background(), "ctr1", runtime.LogsOptions{}).
		Return(rc, nil)

	got, err := m.Logs(context.Background(), "ctr1", runtime.LogsOptions{})
	require.NoError(t, err)
	data, _ := io.ReadAll(got)
	assert.Equal(t, content, string(data))
	m.AssertExpectations(t)
}

func TestMockStream_ReadWrite(t *testing.T) {
	ms := &runtime.MockStream{}
	ms.On("Write", []byte("hello")).Return(nil)
	ms.On("Read").Return([]byte("world"), nil)
	ms.On("Resize", uint16(80), uint16(24)).Return(nil)
	ms.On("Close").Return(nil)

	assert.NoError(t, ms.Write([]byte("hello")))
	data, err := ms.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte("world"), data)
	assert.NoError(t, ms.Resize(80, 24))
	assert.NoError(t, ms.Close())
	ms.AssertExpectations(t)
}
