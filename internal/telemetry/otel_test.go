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
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	p, err := telemetry.Init(context.Background(), "test")
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.NotNil(t, p.Metrics.ActiveSessions)

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	p.Shutdown(ctx) // should not panic
}

func TestInit_customServiceName(t *testing.T) {
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	os.Setenv("OTEL_SERVICE_NAME", "my-service")
	defer os.Unsetenv("OTEL_SERVICE_NAME")

	p, err := telemetry.Init(context.Background(), "1.0.0")
	require.NoError(t, err)
	assert.NotNil(t, p)
	p.Shutdown(context.Background())
}
