// Package extmcp implements agentgateway's ExtMCP gRPC hook on top of the pipeline.
package extmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	gen "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/gen/extmcp"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/identity"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/mcpjson"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/pipeline"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/telemetry"
)

// Metadata keys the adapter stashes on the request phase (McpRequestResult.metadata, exposed to
// CEL as extMcp.<key>) and expects back in McpResponse.metadata_context on the response phase.
// TODO(verify): the AgentgatewayPolicy field that fills metadata_context from CEL, e.g.
//   metadata: { jwt: 'jwt', eacp_tool: 'extMcp.eacp_tool', eacp_conversation_id: '...' }
const (
	MetaTool           = "eacp_tool"
	MetaServer         = "eacp_server"
	MetaConversationID = "eacp_conversation_id"
	MetaJWT            = "jwt"
	MetaTraceparent    = "traceparent"
	MetaBaggage        = "baggage"
)

type Server struct {
	gen.UnimplementedExtMcpServer
	P   *pipeline.Pipeline
	Log *slog.Logger
}

func (s *Server) CheckRequest(ctx context.Context, req *gen.McpRequest) (*gen.McpRequestResult, error) {
	headers := headerMap(req.GetHeaders())
	meta := structMap(req.GetMetadataContext())
	ctx = telemetry.Extract(ctx, carrier(headers, meta))

	subject, ok := subjectFrom(meta, headers)
	if !ok {
		return denyRequest(gen.AuthorizationError_PERMISSION_DENIED, "unauthenticated", "caller identity unavailable to the policy hook", nil), nil
	}
	caller := s.P.Resolve(ctx, subject, conversationID(meta, headers))

	switch req.GetMethod() {
	case "tools/call":
		params, err := mcpjson.ParseToolCall(req.GetMcpRequest())
		if err != nil {
			return denyRequest(gen.AuthorizationError_INVALID, "invalid_params", err.Error(), nil), nil
		}
		server, tool := mcpjson.SplitName(params.Name, req.GetServiceNames())
		out := s.P.ToolCallRequest(ctx, caller, server, tool, params)
		stash, _ := structpb.NewStruct(map[string]any{
			MetaTool: tool, MetaServer: server, MetaConversationID: caller.ConversationID,
			"eacp_decision": out.Action.String(),
		})
		switch out.Action {
		case pipeline.Pass:
			return &gen.McpRequestResult{Result: &gen.McpRequestResult_Pass{Pass: &gen.Pass{}}, Metadata: stash}, nil
		case pipeline.Mutate:
			return &gen.McpRequestResult{Result: &gen.McpRequestResult_Mutated{Mutated: out.Mutated}, Metadata: stash}, nil
		case pipeline.ApprovalRequired:
			data := map[string]any{
				"approval_id": out.Approval.ID, "approval_url": s.P.ApprovalURL(out.Approval),
				"approver_group": out.Approval.ApproverGroup, "expires_at": out.Approval.ExpiresAt,
				"conversation_id": caller.ConversationID,
			}
			return denyRequest(gen.AuthorizationError_PERMISSION_DENIED, out.Code, out.Reason, data), nil
		default:
			return denyRequest(gen.AuthorizationError_PERMISSION_DENIED, out.Code, out.Reason, map[string]any{"conversation_id": caller.ConversationID}), nil
		}
	default:
		// Listings are judged on the response phase; everything else passes through the
		// gateway's own CEL rules.
		stash, _ := structpb.NewStruct(map[string]any{MetaConversationID: caller.ConversationID})
		return &gen.McpRequestResult{Result: &gen.McpRequestResult_Pass{Pass: &gen.Pass{}}, Metadata: stash}, nil
	}
}

func (s *Server) CheckResponse(ctx context.Context, req *gen.McpResponse) (*gen.McpResponseResult, error) {
	meta := structMap(req.GetMetadataContext())
	ctx = telemetry.Extract(ctx, carrier(nil, meta))

	subject, ok := subjectFrom(meta, nil)
	if !ok {
		return denyResponse("unauthenticated", "caller identity unavailable to the response hook; configure metadata_context"), nil
	}
	caller := s.P.Resolve(ctx, subject, conversationID(meta, nil))

	switch req.GetMethod() {
	case "tools/call":
		server, _ := meta[MetaServer].(string)
		tool, _ := meta[MetaTool].(string)
		if server == "" && len(req.GetServiceNames()) > 0 {
			server = req.GetServiceNames()[0]
		}
		if tool == "" {
			return denyResponse("response_context_unavailable", "tool name missing from metadata_context; see extmcp.MetaTool"), nil
		}
		result, err := mcpjson.ParseToolResult(req.GetMcpResponse())
		if err != nil {
			return denyResponse("invalid_result", err.Error()), nil
		}
		out := s.P.ToolCallResponse(ctx, caller, server, tool, result)
		switch out.Action {
		case pipeline.Pass:
			return &gen.McpResponseResult{Result: &gen.McpResponseResult_Pass{Pass: &gen.Pass{}}}, nil
		case pipeline.Mutate:
			return &gen.McpResponseResult{Result: &gen.McpResponseResult_Mutated{Mutated: out.Mutated}}, nil
		default:
			return denyResponse(out.Code, out.Reason), nil
		}
	case "tools/list", "prompts/list", "resources/list":
		kind := strings.TrimSuffix(req.GetMethod(), "/list")
		items, rest, err := mcpjson.ParseList(req.GetMcpResponse(), kind)
		if err != nil {
			return denyResponse("invalid_result", err.Error()), nil
		}
		servers := req.GetServiceNames()
		var kept []map[string]any
		for _, server := range servers {
			mine := itemsFor(items, server, len(servers) > 1)
			filtered, out := s.P.Listing(ctx, caller, server, kind, mine)
			if out.Action == pipeline.Deny {
				return denyResponse(out.Code, out.Reason), nil
			}
			kept = append(kept, filtered...)
		}
		b, err := mcpjson.MarshalList(kind, kept, rest)
		if err != nil {
			return denyResponse("internal", err.Error()), nil
		}
		return &gen.McpResponseResult{Result: &gen.McpResponseResult_Mutated{Mutated: b}}, nil
	default:
		return &gen.McpResponseResult{Result: &gen.McpResponseResult_Pass{Pass: &gen.Pass{}}}, nil
	}
}

// --- helpers --------------------------------------------------------------------------------------

func itemsFor(items []map[string]any, server string, fanout bool) []map[string]any {
	if !fanout {
		return items
	}
	out := []map[string]any{}
	for _, it := range items {
		name, _ := it["name"].(string)
		if strings.HasPrefix(name, server+"_") {
			out = append(out, it)
		}
	}
	return out
}

func subjectFrom(meta map[string]any, headers map[string]string) (identity.Subject, bool) {
	if claims, ok := meta[MetaJWT].(map[string]any); ok && claims["sub"] != nil {
		return identity.FromClaims(claims), true
	}
	if authz, ok := headers["authorization"]; ok {
		if claims, err := identity.ClaimsFromBearer(authz); err == nil && claims["sub"] != nil {
			return identity.FromClaims(claims), true
		}
	}
	return identity.Subject{}, false
}

func conversationID(meta map[string]any, headers map[string]string) string {
	if v, ok := meta[MetaConversationID].(string); ok && v != "" {
		return v
	}
	if v := headers["x-eacp-conversation-id"]; v != "" {
		return v
	}
	for _, src := range []string{headers["baggage"], str(meta[MetaBaggage])} {
		for _, kv := range strings.Split(src, ",") {
			if k, v, ok := strings.Cut(strings.TrimSpace(kv), "="); ok && k == "conversation_id" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func carrier(headers map[string]string, meta map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range headers {
		out[k] = v
	}
	if v := str(meta[MetaTraceparent]); v != "" {
		out["traceparent"] = v
	}
	if v := str(meta[MetaBaggage]); v != "" {
		out["baggage"] = v
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

func denyRequest(code gen.AuthorizationError_Code, eacpCode, reason string, data map[string]any) *gen.McpRequestResult {
	return &gen.McpRequestResult{Result: &gen.McpRequestResult_Error{Error: authzError(code, eacpCode, reason, data)}}
}

func denyResponse(eacpCode, reason string) *gen.McpResponseResult {
	return &gen.McpResponseResult{Result: &gen.McpResponseResult_Error{Error: authzError(gen.AuthorizationError_PERMISSION_DENIED, eacpCode, reason, nil)}}
}

func authzError(code gen.AuthorizationError_Code, eacpCode, reason string, data map[string]any) *gen.AuthorizationError {
	payload := map[string]any{"code": -32001, "message": fmt.Sprintf("eacp policy: %s", reason)}
	d := map[string]any{"eacp_code": eacpCode}
	for k, v := range data {
		d[k] = v
	}
	payload["data"] = d
	b, _ := json.Marshal(payload)
	return &gen.AuthorizationError{Code: code, Reason: reason, McpError: b}
}
