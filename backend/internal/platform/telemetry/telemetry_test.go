package telemetry

import (
	"context"
	"testing"
)

func TestNewDisabled(t *testing.T) {
	manager, err := New(context.Background(), "test-service", Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNewRejectsEmptyServiceName(t *testing.T) {
	_, err := New(context.Background(), "", Config{
		Enabled:              true,
		OTLPEndpoint:         "localhost:4317",
		Insecure:             true,
		ServiceNamespace:     "bookstore",
		Environment:          "test",
		TraceSampleRatio:     1,
		MetricExportInterval: "10s",
		ShutdownTimeout:      "1s",
	})
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}
}
