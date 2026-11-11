// Package extmcp implements agentgateway's ExtMCP gRPC hook on top of the pipeline.
//
// # The hook contract
//
// Everything the adapter knows about a call arrives in McpRequest.metadata_context and
// McpResponse.metadata_context, a struct the gateway fills from CEL expressions configured on
// the guardrail processor (CRD field remote.metadata; standalone field metadata). Verified
// against agentgateway 1.7.0 by test/pathway. The processor must be configured with:
//
//	jwt:                  jwt                                        required, both phases
//	traceparent:          request.headers["traceparent"]             optional
//	baggage:              request.headers["baggage"]                 optional
//	conversation_id:      request.headers["x-eacp-conversation-id"]  optional
//	prefix:               "always" on the aggregate, "none" on a per-server route
//	eacp_tool:            mcpGuardrails.eacp_tool                    response phase of tools/call
//	eacp_server:          mcpGuardrails.eacp_server
//	eacp_conversation_id: mcpGuardrails.eacp_conversation_id
//
// Three facts shape this file:
//
//   - `jwt` holds the claims of the token the gateway verified. It is the only identity source.
//     The Authorization header never reaches the hook (the gateway consumes the validated token),
//     and reading a bearer token from a header would also mean trusting a token nobody verified
//     on any route that lacks a JWT policy. No verified claims, no call.
//   - What the request phase returns in McpRequestResult.metadata is kept by the gateway for the
//     rest of the request and readable in CEL as mcpGuardrails.<key>. That is how the tool name
//     and the resolved conversation id reach the response phase, which otherwise sees only the
//     result. An expression whose variable is not set yet (the mcpGuardrails ones on the request
//     phase) is skipped by the gateway, not an error.
//   - On tools/call the gateway has already removed the aggregate's "<server>_" prefix from the
//     tool name and names the server in service_names. Listings are the opposite: one call for
//     the merged listing, names still prefixed, every backing server in service_names.
package extmcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	gen "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/gen/extmcp"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/identity"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/mcpjson"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/pipeline"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/telemetry"
)

// Keys of metadata_context (what the gateway sends).
const (
	MetaJWT            = "jwt"
	MetaTraceparent    = "traceparent"
	MetaBaggage        = "baggage"
	MetaConversation   = "conversation_id"
	MetaPrefix         = "prefix"
	MetaTool           = "eacp_tool"
	MetaServer         = "eacp_server"
	MetaConversationID = "eacp_conversation_id"
)

// Key the request phase stashes besides the three eacp_* keys above; the gateway's access log
// reads it as mcpGuardrails.decision.
const StashDecision = "decision"

const (
	PrefixAlways = "always"
	PrefixNone   = "none"
)

type Server struct {
	gen.UnimplementedExtMcpServer
	P   *pipeline.Pipeline
	Log *slog.Logger
}

func (s *Server) CheckRequest(ctx context.Context, req *gen.McpRequest) (*gen.McpRequestResult, error) {
	headers := headerMap(req.GetHeaders())
	meta := structMap(req.GetMetadataContext())
	s.debug(ctx, "request", req.GetMethod(), req.GetServiceNames(), headers, meta)
	ctx = telemetry.Extract(ctx, carrier(ctx, headers, meta))

	subject, ok := subjectFrom(meta)
	if !ok {
		s.Log.Warn("hook call without verified claims; denying", "method", req.GetMethod(), "phase", "request")
		return denyRequest(gen.AuthorizationError_PERMISSION_DENIED, "unauthenticated", "the gateway sent no verified token claims to the policy hook", nil, req.GetMethod() == "tools/call"), nil
	}
	caller := s.P.Resolve(ctx, subject, conversationID(meta, headers))

	if req.GetMethod() != "tools/call" {
		// Listings are judged on the response phase. Anything else configured for the request
		// phase passes; the stash still carries the conversation id forward.
		return &gen.McpRequestResult{Result: &gen.McpRequestResult_Pass{Pass: &gen.Pass{}}, Metadata: stash(caller, "", "", "pass")}, nil
	}

	params, err := mcpjson.ParseToolCall(req.GetMcpRequest())
	if err != nil {
		return denyRequest(gen.AuthorizationError_INVALID, "invalid_params", err.Error(), nil, true), nil
	}
	server, tool := mcpjson.SplitName(params.Name, req.GetServiceNames())
	if server == "" {
		return denyRequest(gen.AuthorizationError_INVALID, "unknown_server", "the gateway named no backing server for this call", nil, true), nil
	}
	out := s.P.ToolCallRequest(ctx, caller, server, tool, params)
	st := stash(caller, server, tool, out.Action.String())
	switch out.Action {
	case pipeline.Pass:
		return &gen.McpRequestResult{Result: &gen.McpRequestResult_Pass{Pass: &gen.Pass{}}, Metadata: st}, nil
	case pipeline.Mutate:
		return &gen.McpRequestResult{Result: &gen.McpRequestResult_Mutated{Mutated: out.Mutated}, Metadata: st}, nil
	case pipeline.ApprovalRequired:
		data := map[string]any{
			"approval_id": out.Approval.ID, "approval_url": s.P.ApprovalURL(out.Approval),
			"approver_group": out.Approval.ApproverGroup, "expires_at": out.Approval.ExpiresAt,
			"conversation_id": caller.ConversationID,
		}
		return denyRequest(gen.AuthorizationError_PERMISSION_DENIED, out.Code, out.Reason, data, true), nil
	default:
		return denyRequest(gen.AuthorizationError_PERMISSION_DENIED, out.Code, out.Reason, denialData(caller, out), true), nil
	}
}

// denialData is what a client learns about a denial besides the sentence: the conversation, and
// every reason code when policy gave more than one.
func denialData(c pipeline.Caller, out pipeline.Outcome) map[string]any {
	data := map[string]any{"conversation_id": c.ConversationID}
	if out.Decision != nil && len(out.Decision.Reasons) > 0 {
		data["eacp_codes"] = out.Decision.ReasonCodes()
	}
	return data
}

func (s *Server) CheckResponse(ctx context.Context, req *gen.McpResponse) (*gen.McpResponseResult, error) {
	meta := structMap(req.GetMetadataContext())
	s.debug(ctx, "response", req.GetMethod(), req.GetServiceNames(), nil, meta)
	ctx = telemetry.Extract(ctx, carrier(ctx, nil, meta))

	subject, ok := subjectFrom(meta)
	if !ok {
		s.Log.Warn("hook call without verified claims; denying", "method", req.GetMethod(), "phase", "response")
		return denyResponse("unauthenticated", "the gateway sent no verified token claims to the policy hook", nil, req.GetMethod() == "tools/call"), nil
	}
	caller := s.P.Resolve(ctx, subject, conversationID(meta, nil))
	data := map[string]any{"conversation_id": caller.ConversationID}

	switch req.GetMethod() {
	case "tools/call":
		server, tool := str(meta[MetaServer]), str(meta[MetaTool])
		if server == "" || tool == "" {
			// The request phase did not run, or the processor is missing the mcpGuardrails.*
			// metadata expressions. Without the tool name the result cannot be judged.
			s.Log.Error("response phase without the stashed tool name; denying", "services", req.GetServiceNames())
			return denyResponse("response_context_unavailable", "the request phase's tool name did not reach the response phase; check the processor's metadata expressions", data, true), nil
		}
		result, err := mcpjson.ParseToolResult(req.GetMcpResponse())
		if err != nil {
			return denyResponse("invalid_result", err.Error(), data, true), nil
		}
		out := s.P.ToolCallResponse(ctx, caller, server, tool, result)
		switch out.Action {
		case pipeline.Pass:
			return &gen.McpResponseResult{Result: &gen.McpResponseResult_Pass{Pass: &gen.Pass{}}}, nil
		case pipeline.Mutate:
			return &gen.McpResponseResult{Result: &gen.McpResponseResult_Mutated{Mutated: out.Mutated}}, nil
		default:
			return denyResponse(out.Code, out.Reason, denialData(caller, out), true), nil
		}

	case "tools/list", "prompts/list", "resources/list":
		kind := strings.TrimSuffix(req.GetMethod(), "/list")
		items, rest, err := mcpjson.ParseList(req.GetMcpResponse(), kind)
		if err != nil {
			return denyResponse("invalid_result", err.Error(), data, false), nil
		}
		servers := req.GetServiceNames()
		prefixed := str(meta[MetaPrefix]) == PrefixAlways || len(servers) > 1
		kept := []map[string]any{}
		for _, part := range partition(items, servers, prefixed) {
			filtered, out := s.P.Listing(ctx, caller, part.server, kind, part.items, prefixed)
			if out.Action == pipeline.Deny {
				return denyResponse(out.Code, out.Reason, data, false), nil
			}
			kept = append(kept, filtered...)
		}
		b, err := mcpjson.MarshalList(kind, kept, rest)
		if err != nil {
			return denyResponse("internal", err.Error(), data, false), nil
		}
		return &gen.McpResponseResult{Result: &gen.McpResponseResult_Mutated{Mutated: b}}, nil

	default:
		return &gen.McpResponseResult{Result: &gen.McpResponseResult_Pass{Pass: &gen.Pass{}}}, nil
	}
}

// --- helpers --------------------------------------------------------------------------------------

type listPart struct {
	server string
	items  []map[string]any
}

// partition attributes the items of a merged listing to the servers behind it. With prefixed
// names an item belongs to the server whose "<server>_" prefix it carries; server names are DNS
// labels and cannot contain an underscore, so the attribution is unambiguous. An item that
// carries no listed server's prefix is dropped: nothing vouches for it. Every listed server gets
// a part, even an empty one, because "this server now lists nothing" is a change worth hashing.
func partition(items []map[string]any, servers []string, prefixed bool) []listPart {
	if !prefixed {
		if len(servers) != 1 {
			return nil
		}
		return []listPart{{server: servers[0], items: items}}
	}
	parts := make([]listPart, len(servers))
	index := map[string]int{}
	for i, s := range servers {
		parts[i] = listPart{server: s, items: []map[string]any{}}
		index[s] = i
	}
	for _, it := range items {
		if server, ok := mcpjson.PrefixOwner(mcpjson.ItemKey(it), servers); ok {
			parts[index[server]].items = append(parts[index[server]].items, it)
		}
	}
	return parts
}

func stash(c pipeline.Caller, server, tool, decision string) *structpb.Struct {
	m := map[string]any{MetaConversationID: c.ConversationID, StashDecision: decision}
	if server != "" {
		m[MetaServer] = server
	}
	if tool != "" {
		m[MetaTool] = tool
	}
	s, _ := structpb.NewStruct(m)
	return s
}

// subjectFrom reads the verified claims. There is deliberately no other source; see the package
// comment.
func subjectFrom(meta map[string]any) (identity.Subject, bool) {
	claims, ok := meta[MetaJWT].(map[string]any)
	if !ok || str(claims["sub"]) == "" {
		return identity.Subject{}, false
	}
	return identity.FromClaims(claims), true
}

// conversationID prefers what the request phase resolved, then the explicit header, then the
// W3C baggage member, from metadata first and forwarded headers second.
func conversationID(meta map[string]any, headers map[string]string) string {
	for _, v := range []string{str(meta[MetaConversationID]), str(meta[MetaConversation]), headers["x-eacp-conversation-id"]} {
		if v != "" {
			return v
		}
	}
	for _, src := range []string{str(meta[MetaBaggage]), headers["baggage"]} {
		for _, kv := range strings.Split(src, ",") {
			if k, v, ok := strings.Cut(strings.TrimSpace(kv), "="); ok && k == "conversation_id" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// carrier assembles the trace context to continue. The gateway rewrites the traceparent it
// exposes in CEL so its span id is the gateway's own request span, which makes the adapter's
// spans children of it. If the gateway also propagates context on the gRPC call itself, that
// more specific parent wins.
func carrier(ctx context.Context, headers map[string]string, meta map[string]any) map[string]string {
	out := map[string]string{}
	for _, k := range []string{"traceparent", "tracestate", "baggage"} {
		if v := headers[k]; v != "" {
			out[k] = v
		}
	}
	if v := str(meta[MetaTraceparent]); v != "" {
		out["traceparent"] = v
	}
	if v := str(meta[MetaBaggage]); v != "" {
		out["baggage"] = v
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for _, k := range []string{"traceparent", "tracestate", "baggage"} {
			if v := md.Get(k); len(v) > 0 && v[0] != "" {
				out[k] = v[0]
			}
		}
	}
	return out
}

func headerMap(hs []*gen.McpHeader) map[string]string {
	out := map[string]string{}
	for _, h := range hs {
		k := strings.ToLower(h.GetKey())
		if _, dup := out[k]; dup {
			out[k] += "," + string(h.GetValue())
			continue
		}
		out[k] = string(h.GetValue())
	}
	return out
}

func structMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	return s.AsMap()
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// debug logs the shape of a hook call, never its content: header and metadata names only.
func (s *Server) debug(ctx context.Context, phase, method string, services []string, headers map[string]string, meta map[string]any) {
	if !s.Log.Enabled(ctx, slog.LevelDebug) {
		return
	}
	hk := make([]string, 0, len(headers))
	for k := range headers {
		hk = append(hk, k)
	}
	mk := make([]string, 0, len(meta))
	for k := range meta {
		mk = append(mk, k)
	}
	sort.Strings(hk)
	sort.Strings(mk)
	s.Log.Debug("hook", "phase", phase, "method", method, "services", services, "headers", hk, "metadata", mk)
}

func denyRequest(code gen.AuthorizationError_Code, eacpCode, reason string, data map[string]any, toolCall bool) *gen.McpRequestResult {
	return &gen.McpRequestResult{Result: &gen.McpRequestResult_Error{Error: authzError(code, eacpCode, reason, data, toolCall)}}
}

func denyResponse(eacpCode, reason string, data map[string]any, toolCall bool) *gen.McpResponseResult {
	return &gen.McpResponseResult{Result: &gen.McpResponseResult_Error{Error: authzError(gen.AuthorizationError_PERMISSION_DENIED, eacpCode, reason, data, toolCall)}}
}

// authzError builds the denial. What the client receives depends on the method, because the
// gateway renders the two cases differently (agentgateway 1.7, checked by test/pathway):
//
//   - A denied tools/call comes back as an ordinary tool result with isError true whose only
//     content is Reason as text. McpError is dropped. The model reads that text, and so do
//     clients and the scenario tests, so Reason is a small JSON object: a sentence under
//     "message" and the machine-readable part under "data".
//   - Any other denial (a listing, for instance) becomes a JSON-RPC error: the code derives from
//     Code, the message is Reason, and McpError, parsed as JSON, is the error's `data` member.
//     There Reason stays a sentence and McpError is the data object itself, not an envelope.
//
// Either way the data carries eacp_code, the stable identifier of why.
func authzError(code gen.AuthorizationError_Code, eacpCode, reason string, data map[string]any, toolCall bool) *gen.AuthorizationError {
	d := map[string]any{"eacp_code": eacpCode}
	for k, v := range data {
		d[k] = v
	}
	sentence := "eacp policy: " + reason
	b, _ := json.Marshal(d)
	if !toolCall {
		return &gen.AuthorizationError{Code: code, Reason: sentence, McpError: b}
	}
	text, _ := json.Marshal(map[string]any{"error": "eacp_policy", "message": sentence, "data": d})
	return &gen.AuthorizationError{Code: code, Reason: string(text), McpError: b}
}
