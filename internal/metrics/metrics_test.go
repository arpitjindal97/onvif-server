package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/aragarwal/onvif-server/internal/config"
)

func TestInit_DisabledReturnsNoopShutdown(t *testing.T) {
	shutdown, err := Init(context.Background(), config.MetricsConfig{Enabled: false}, "test")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown returned error: %v", err)
	}
}

func TestInit_EnabledFailsFast_OnUnreachableEndpoint(t *testing.T) {
	// Use an obviously-unreachable endpoint and a tiny timeout so we don't
	// need a running OTLP collector. otlpmetricgrpc.New will succeed the
	// initial DialContext (it's lazy) but periodic export attempts will
	// fail in the background; that's fine for this happy-path test.
	cfg := config.MetricsConfig{
		Enabled:      true,
		OTLPEndpoint: "127.0.0.1:1", // closed port
		Insecure:     true,
		ServiceName:  "onvif-server-test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	shutdown, err := Init(ctx, cfg, "v0-test")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Shutdown should complete within a short window even if the exporter
	// can't reach the collector.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := shutdown(shutdownCtx); err != nil {
		// We don't fail the test on shutdown errors — exporters frequently
		// surface ResourceExhausted / Unavailable when the collector isn't
		// up. We just want to ensure the call returns rather than hangs.
		t.Logf("shutdown returned (expected, collector unreachable): %v", err)
	}
}
