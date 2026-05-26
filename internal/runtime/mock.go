package runtime

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

// MockRuntime is a testify mock for the Runtime interface.
type MockRuntime struct {
	mock.Mock
}

func (m *MockRuntime) ListContainers(ctx context.Context, f Filter) ([]Container, error) {
	args := m.Called(ctx, f)
	if v := args.Get(0); v != nil {
		return v.([]Container), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRuntime) Exec(ctx context.Context, id string, opts ExecOptions) (Stream, error) {
	args := m.Called(ctx, id, opts)
	if v := args.Get(0); v != nil {
		return v.(Stream), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRuntime) Attach(ctx context.Context, id string) (Stream, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(Stream), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRuntime) Logs(ctx context.Context, id string, opts LogsOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, id, opts)
	if v := args.Get(0); v != nil {
		return v.(io.ReadCloser), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockStream is a testify mock for the Stream interface.
type MockStream struct {
	mock.Mock
}

func (m *MockStream) Read() ([]byte, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStream) Write(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockStream) Resize(cols, rows uint16) error {
	args := m.Called(cols, rows)
	return args.Error(0)
}

func (m *MockStream) Close() error {
	args := m.Called()
	return args.Error(0)
}
