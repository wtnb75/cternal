package telemetry_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/telemetry"
)

func TestInit_noEndpoint(t *testing.T) {
	// Ensure OTEL endpoint is not set → uses no-op providers.
	require.NoError(t, os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	p, err := telemetry.Init(context.Background(), "test")
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.NotNil(t, p.Metrics.ActiveSessions)

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	p.Shutdown(ctx) // should not panic
}

func TestInit_customServiceName(t *testing.T) {
	require.NoError(t, os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	require.NoError(t, os.Setenv("OTEL_SERVICE_NAME", "my-service"))
	defer func() { require.NoError(t, os.Unsetenv("OTEL_SERVICE_NAME")) }()

	p, err := telemetry.Init(context.Background(), "1.0.0")
	require.NoError(t, err)
	assert.NotNil(t, p)
	p.Shutdown(context.Background())
}

func TestProviders_tracerAndMeter(t *testing.T) {
	require.NoError(t, os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	p, err := telemetry.Init(context.Background(), "test")
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	tracer := p.Tracer("test-tracer")
	assert.NotNil(t, tracer)

	meter := p.Meter("test-meter")
	assert.NotNil(t, meter)
}

func TestShutdown_cancelledContext(t *testing.T) {
	require.NoError(t, os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	p, err := telemetry.Init(context.Background(), "test")
	require.NoError(t, err)

	// Shutdown with already-cancelled context — must not panic.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Shutdown(ctx)
}
