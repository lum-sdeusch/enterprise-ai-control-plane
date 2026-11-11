// eacp-hookprobe is a diagnostic stand-in for the adapter: an ExtMCP server that lets every call
// pass and prints exactly what agentgateway sent, one JSON object per line. Use it to see which
// headers, metadata and bodies a gateway configuration delivers on each phase before trusting
// the adapter with it (test/pathway/run.sh --probe).
//
// On the request phase it stashes the tool name under McpRequestResult.metadata, so the output
// also shows whether a metadata expression such as `mcpGuardrails.eacp_tool` brings it back on
// the response phase. Bearer tokens are reduced to their claims; nothing here verifies them.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	gen "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/gen/extmcp"
)

type probe struct {
	gen.UnimplementedExtMcpServer
	mu  sync.Mutex
	enc *json.Encoder
}

func (p *probe) emit(v map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.enc.Encode(v)
}

func grpcMeta(ctx context.Context) map[string]string {
	out := map[string]string{}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for k, v := range md {
			out[k] = strings.Join(v, ",")
		}
	}
	return out
}

func headers(hs []*gen.McpHeader) map[string]string {
	out := map[string]string{}
	for _, h := range hs {
		k := strings.ToLower(h.GetKey())
		v := string(h.GetValue())
		if k == "authorization" {
			v = "<bearer claims> " + claims(v)
		}
		if prev, ok := out[k]; ok {
			v = prev + "," + v
		}
		out[k] = v
	}
	return out
}

func claims(authorization string) string {
	parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(authorization, "Bearer "), "bearer ")), ".")
	if len(parts) != 3 {
		return "(not a JWT)"
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "(undecodable)"
	}
	return string(b)
}

func raw(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

func (p *probe) CheckRequest(ctx context.Context, req *gen.McpRequest) (*gen.McpRequestResult, error) {
	body := raw(req.GetMcpRequest())
	p.emit(map[string]any{
		"phase": "request", "method": req.GetMethod(), "service_names": req.GetServiceNames(),
		"headers": headers(req.GetHeaders()), "metadata_context": req.GetMetadataContext().AsMap(),
		"grpc_metadata": grpcMeta(ctx), "body": body,
	})
	stash := map[string]any{"probe": "request-phase-was-here"}
	if m, ok := body.(map[string]any); ok {
		if name, ok := m["name"].(string); ok {
			stash["eacp_tool"] = name
		}
	}
	s, _ := structpb.NewStruct(stash)
	return &gen.McpRequestResult{Result: &gen.McpRequestResult_Pass{Pass: &gen.Pass{}}, Metadata: s}, nil
}

func (p *probe) CheckResponse(ctx context.Context, req *gen.McpResponse) (*gen.McpResponseResult, error) {
	p.emit(map[string]any{
		"phase": "response", "method": req.GetMethod(), "service_names": req.GetServiceNames(),
		"metadata_context": req.GetMetadataContext().AsMap(), "grpc_metadata": grpcMeta(ctx),
		"body": raw(req.GetMcpResponse()),
	})
	return &gen.McpResponseResult{Result: &gen.McpResponseResult_Pass{Pass: &gen.Pass{}}}, nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9001", "listen address")
	flag.Parse()
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	gen.RegisterExtMcpServer(srv, &probe{enc: json.NewEncoder(os.Stdout)})
	log.Printf("eacp-hookprobe listening on %s", *addr)
	log.Fatal(srv.Serve(lis))
}
