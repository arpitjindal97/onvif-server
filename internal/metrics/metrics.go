// Package metrics initializes the OpenTelemetry SDK to export metrics over
// OTLP gRPC. When metrics are disabled in config, Init returns a no-op
// Shutdown so callers can use it unconditionally.
package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/aragarwal/onvif-server/internal/config"
	"github.com/aragarwal/onvif-server/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// ShutdownFunc flushes pending metrics and releases the OTLP connection.
// It is always safe to call (it is a no-op when metrics are disabled).
type ShutdownFunc func(context.Context) error

// noop is returned when metrics are disabled.
func noop(context.Context) error { return nil }

// Init configures a global MeterProvider that exports via OTLP gRPC. If
// cfg.Enabled is false it sets up a no-op provider and returns a no-op
// Shutdown.
func Init(ctx context.Context, cfg config.MetricsConfig, version string) (ShutdownFunc, error) {
	if !cfg.Enabled {
		logger.Info("Metrics: disabled")
		return noop, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(version),
		),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		return noop, fmt.Errorf("metrics: build resource: %w", err)
	}

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return noop, fmt.Errorf("metrics: create OTLP exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(15*time.Second),
		)),
	)
	otel.SetMeterProvider(provider)

	logger.Info("Metrics: OTLP gRPC -> %s (insecure=%v, service=%s)",
		cfg.OTLPEndpoint, cfg.Insecure, cfg.ServiceName)

	return provider.Shutdown, nil
}
