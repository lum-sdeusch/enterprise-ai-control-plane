package telemetry

import (
	"context"
	"testing"
)

// Setup once failed at start-up with "conflicting Schema URL" and took the adapter down with it.
func TestSetupStartsWithoutAnExporter(t *testing.T) {
	shutdown, err := Setup(context.Background(), "", "eacp-adapter-test", "test")
	if err != nil {
		t.Fatalf("telemetry setup must not fail: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx := Extract(context.Background(), map[string]string{"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"})
	ctx, span := Tracer().Start(ctx, "t")
	defer span.End()
	if traceID, _ := IDs(ctx); traceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("the incoming trace must be continued, got %q", traceID)
	}
}
