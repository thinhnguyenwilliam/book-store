// Package telemetry initializes the process-wide OpenTelemetry providers.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config controls OTLP export for traces and metrics.
type Config struct {
	Enabled              bool    `mapstructure:"enabled"`
	OTLPEndpoint         string  `mapstructure:"otlp_endpoint"`
	Insecure             bool    `mapstructure:"insecure"`
	ServiceNamespace     string  `mapstructure:"service_namespace"`
	Environment          string  `mapstructure:"environment"`
	TraceSampleRatio     float64 `mapstructure:"trace_sample_ratio"`
	MetricExportInterval string  `mapstructure:"metric_export_interval"`
	ShutdownTimeout      string  `mapstructure:"shutdown_timeout"`
}

// Manager owns the providers created for one process.
type Manager struct {
	enabled         bool
	shutdownTimeout time.Duration
	tracerProvider  *sdktrace.TracerProvider
	meterProvider   *sdkmetric.MeterProvider
}

// New configures W3C propagation and exports telemetry to an OTLP collector.
// Exporters connect asynchronously, so a temporarily unavailable collector
// never prevents an application service from starting.
func New(ctx context.Context, serviceName string, cfg Config) (*Manager, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	manager := &Manager{enabled: cfg.Enabled}
	if !cfg.Enabled {
		return manager, nil
	}

	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, errors.New("telemetry service name is required")
	}
	metricInterval, err := time.ParseDuration(cfg.MetricExportInterval)
	if err != nil || metricInterval <= 0 {
		return nil, fmt.Errorf("parse telemetry metric export interval %q", cfg.MetricExportInterval)
	}
	manager.shutdownTimeout, err = time.ParseDuration(cfg.ShutdownTimeout)
	if err != nil || manager.shutdownTimeout <= 0 {
		return nil, fmt.Errorf("parse telemetry shutdown timeout %q", cfg.ShutdownTimeout)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("service.namespace", cfg.ServiceNamespace),
		attribute.String("deployment.environment.name", cfg.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	traceOptions := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint)}
	metricOptions := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint)}
	if cfg.Insecure {
		traceOptions = append(traceOptions, otlptracegrpc.WithInsecure())
		metricOptions = append(metricOptions, otlpmetricgrpc.WithInsecure())
	}

	traceExporter, err := otlptracegrpc.New(ctx, traceOptions...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	manager.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx, metricOptions...)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, manager.shutdownTimeout)
		defer cancel()
		_ = manager.tracerProvider.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	manager.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(metricInterval))),
	)

	otel.SetTracerProvider(manager.tracerProvider)
	otel.SetMeterProvider(manager.meterProvider)
	return manager, nil
}

// Close flushes pending spans and metrics within the configured timeout.
func (m *Manager) Close() error {
	if m == nil || !m.enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()
	return errors.Join(
		m.meterProvider.Shutdown(ctx),
		m.tracerProvider.Shutdown(ctx),
	)
}
