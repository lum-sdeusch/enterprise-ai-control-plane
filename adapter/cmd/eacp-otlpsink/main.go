// eacp-otlpsink is a test helper: an OTLP/gRPC trace receiver that prints every span it is sent
// as one JSON object per line. The pathway test points the gateway and the adapter at it and
// checks that one tool call yields one trace, with the adapter's spans under the gateway's.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"sync"

	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
)

type sink struct {
	coltrace.UnimplementedTraceServiceServer
	mu  sync.Mutex
	enc *json.Encoder
}

func attrs(kvs []*commonpb.KeyValue) map[string]any {
	out := map[string]any{}
	for _, kv := range kvs {
		switch v := kv.GetValue().GetValue().(type) {
		case *commonpb.AnyValue_StringValue:
			out[kv.GetKey()] = v.StringValue
		case *commonpb.AnyValue_BoolValue:
			out[kv.GetKey()] = v.BoolValue
		case *commonpb.AnyValue_IntValue:
			out[kv.GetKey()] = v.IntValue
		case *commonpb.AnyValue_DoubleValue:
			out[kv.GetKey()] = v.DoubleValue
		default:
			out[kv.GetKey()] = kv.GetValue().String()
		}
	}
	return out
}

func (s *sink) Export(_ context.Context, req *coltrace.ExportTraceServiceRequest) (*coltrace.ExportTraceServiceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rs := range req.GetResourceSpans() {
		service, _ := attrs(rs.GetResource().GetAttributes())["service.name"].(string)
		for _, ss := range rs.GetScopeSpans() {
			for _, sp := range ss.GetSpans() {
				_ = s.enc.Encode(map[string]any{
					"service": service, "name": sp.GetName(),
					"trace_id": hex.EncodeToString(sp.GetTraceId()), "span_id": hex.EncodeToString(sp.GetSpanId()),
					"parent_span_id": hex.EncodeToString(sp.GetParentSpanId()), "attributes": attrs(sp.GetAttributes()),
				})
			}
		}
	}
	return &coltrace.ExportTraceServiceResponse{}, nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:4317", "listen address")
	flag.Parse()
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	coltrace.RegisterTraceServiceServer(srv, &sink{enc: json.NewEncoder(os.Stdout)})
	log.Printf("eacp-otlpsink listening on %s", *addr)
	log.Fatal(srv.Serve(lis))
}
