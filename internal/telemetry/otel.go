package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultServiceName = "cternal"
)

// Metrics holds pre-registered OTel instruments.
type Metrics struct {
	ActiveSessions   metric.Int64UpDownCounter
	SessionDuration  metric.Float64Histogram
	WSMessages       metric.Int64Counter
	APILatency       metric.Float64Histogram
	WebhookErrors    metric.Int64Counter
}

// Provider wraps OTel SDK providers and exposes metrics.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	Metrics        Metrics
}

// Tracer returns a Tracer for the given instrumentation scope.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.tracerProvider.Tracer(name)
}

// Meter returns a Meter for the given instrumentation scope.
func (p *Provider) Meter(name string) metric.Meter {
	return p.meterProvider.Meter(name)
}

// Shutdown flushes and shuts down the OTel providers.
func (p *Provider) Shutdown(ctx context.Context) {
	if err := p.tracerProvider.Shutdown(ctx); err != nil {
		slog.Error("tracer shutdown", "err", err)
	}
	if err := p.meterProvider.Shutdown(ctx); err != nil {
		slog.Error("meter shutdown", "err", err)
	}
}

// Init initialises OpenTelemetry.
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset, a no-op provider is used.
func Init(ctx context.Context, version string) (*Provider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		res = sdkresource.Default()
	}

	var (
		tp *sdktrace.TracerProvider
		mp *sdkmetric.MeterProvider
	)

	if endpoint == "" {
		// No-op: use SDK with no exporters (spans are dropped).
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
		mp = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	} else {
		traceExp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("trace exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(traceExp),
		)

		metricExp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("metric exporter: %w", err)
		}
		mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
				sdkmetric.WithInterval(30*time.Second),
			)),
		)
	}

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	meter := mp.Meter("cternal")
	metrics, err := registerMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}

	return &Provider{tracerProvider: tp, meterProvider: mp, Metrics: metrics}, nil
}

func registerMetrics(meter metric.Meter) (Metrics, error) {
	activeSessions, err := meter.Int64UpDownCounter("cternal.sessions.active",
		metric.WithDescription("Number of active sessions"))
	if err != nil {
		return Metrics{}, err
	}

	sessionDuration, err := meter.Float64Histogram("cternal.sessions.duration_seconds",
		metric.WithDescription("Session duration in seconds"))
	if err != nil {
		return Metrics{}, err
	}

	wsMessages, err := meter.Int64Counter("cternal.ws.messages_total",
		metric.WithDescription("Total WebSocket messages processed"))
	if err != nil {
		return Metrics{}, err
	}

	apiLatency, err := meter.Float64Histogram("cternal.api.latency_seconds",
		metric.WithDescription("HTTP API request latency"))
	if err != nil {
		return Metrics{}, err
	}

	webhookErrors, err := meter.Int64Counter("cternal.webhook.errors_total",
		metric.WithDescription("Total webhook delivery failures"))
	if err != nil {
		return Metrics{}, err
	}

	return Metrics{
		ActiveSessions:  activeSessions,
		SessionDuration: sessionDuration,
		WSMessages:      wsMessages,
		APILatency:      apiLatency,
		WebhookErrors:   webhookErrors,
	}, nil
}
