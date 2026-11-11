package extmcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"google.golang.org/grpc/metadata"

	gen "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/gen/extmcp"
)

func names(items []map[string]any) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it["name"].(string))
	}
	return out
}

func TestIdentityComesOnlyFromVerifiedClaims(t *testing.T) {
	if _, ok := subjectFrom(map[string]any{}); ok {
		t.Fatal("no jwt in metadata must mean no identity")
	}
	if _, ok := subjectFrom(map[string]any{"jwt": map[string]any{"groups": []any{"support"}}}); ok {
		t.Fatal("claims without a subject are not an identity")
	}
	s, ok := subjectFrom(map[string]any{"jwt": map[string]any{
		"sub": "alice", "groups": []any{"support"}, "team": "support", "clearance": "restricted",
		"azp": "librechat", "act": map[string]any{"client_id": "agent-support-assistant"},
	}})
	if !ok || s.Sub != "alice" || s.Team != "support" || s.Act == nil || s.Act.ClientID != "agent-support-assistant" || !reflect.DeepEqual(s.Groups, []string{"support"}) {
		t.Fatalf("subject = %+v", s)
	}
}

func TestPartitionOfAMergedListing(t *testing.T) {
	items := []map[string]any{
		{"name": "tickets_list_tickets"}, {"name": "notes_read_note"}, {"name": "tickets_close_ticket"},
		{"name": "stray_tool"}, // carries no listed server's prefix
	}
	parts := partition(items, []string{"tickets", "notes", "silent"}, true)
	if len(parts) != 3 {
		t.Fatalf("every listed server gets a part, got %d", len(parts))
	}
	if got := names(parts[0].items); !reflect.DeepEqual(got, []string{"tickets_list_tickets", "tickets_close_ticket"}) {
		t.Fatalf("tickets = %v", got)
	}
	if got := names(parts[1].items); !reflect.DeepEqual(got, []string{"notes_read_note"}) {
		t.Fatalf("notes = %v", got)
	}
	if parts[2].server != "silent" || len(parts[2].items) != 0 {
		t.Fatalf("a server that lists nothing still gets an (empty) part: %+v", parts[2])
	}
}

func TestPartitionOfAPerServerListing(t *testing.T) {
	items := []map[string]any{{"name": "list_tickets"}, {"name": "tickets_like_name"}}
	parts := partition(items, []string{"tickets"}, false)
	if len(parts) != 1 || len(parts[0].items) != 2 {
		t.Fatalf("unprefixed listing belongs wholly to its one server: %+v", parts)
	}
	if got := partition(items, []string{"a", "b"}, false); got != nil {
		t.Fatalf("unprefixed items of several servers cannot be attributed: %+v", got)
	}
}

func TestConversationIDPrecedence(t *testing.T) {
	meta := map[string]any{"eacp_conversation_id": "from-request-phase", "conversation_id": "from-header", "baggage": "a=b, conversation_id=from-baggage"}
	if got := conversationID(meta, nil); got != "from-request-phase" {
		t.Fatalf("got %q", got)
	}
	delete(meta, "eacp_conversation_id")
	if got := conversationID(meta, nil); got != "from-header" {
		t.Fatalf("got %q", got)
	}
	delete(meta, "conversation_id")
	if got := conversationID(meta, nil); got != "from-baggage" {
		t.Fatalf("got %q", got)
	}
	if got := conversationID(map[string]any{}, map[string]string{"x-eacp-conversation-id": "hdr"}); got != "hdr" {
		t.Fatalf("got %q", got)
	}
	if got := conversationID(map[string]any{}, nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTraceCarrierPrefersTheGrpcCallContext(t *testing.T) {
	meta := map[string]any{"traceparent": "00-aaaa-bbbb-01", "baggage": "conversation_id=c1"}
	c := carrier(context.Background(), nil, meta)
	if c["traceparent"] != "00-aaaa-bbbb-01" || c["baggage"] != "conversation_id=c1" {
		t.Fatalf("carrier = %v", c)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("traceparent", "00-aaaa-cccc-01"))
	if c := carrier(ctx, nil, meta); c["traceparent"] != "00-aaaa-cccc-01" {
		t.Fatalf("the gRPC call's own context should win: %v", c)
	}
}

func TestDenialOfAListingIsAJSONRPCErrorWithData(t *testing.T) {
	e := denyResponse("fail_closed", "policy unavailable", map[string]any{"conversation_id": "c1"}, false).GetError()
	if e.GetCode() != gen.AuthorizationError_PERMISSION_DENIED || e.GetReason() != "eacp policy: policy unavailable" {
		t.Fatalf("error = %+v", e)
	}
	// The gateway uses mcp_error as the JSON-RPC error's `data`, so it must be the data object.
	var data map[string]any
	if err := json.Unmarshal(e.GetMcpError(), &data); err != nil {
		t.Fatal(err)
	}
	if data["eacp_code"] != "fail_closed" || data["conversation_id"] != "c1" || data["code"] != nil {
		t.Fatalf("data = %v", data)
	}
}

func TestDenialOfAToolCallPutsTheCodeInTheText(t *testing.T) {
	// The gateway renders a denied tools/call as a tool result whose text is the reason and drops
	// mcp_error, so the reason itself must be parseable.
	e := denyRequest(gen.AuthorizationError_PERMISSION_DENIED, "exfil_after_restricted", "no", map[string]any{"conversation_id": "c1", "eacp_codes": []string{"exfil_after_restricted"}}, true).GetError()
	var text struct {
		Error   string         `json:"error"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(e.GetReason()), &text); err != nil {
		t.Fatalf("reason is not JSON: %v: %s", err, e.GetReason())
	}
	if text.Error != "eacp_policy" || text.Message != "eacp policy: no" || text.Data["eacp_code"] != "exfil_after_restricted" || text.Data["conversation_id"] != "c1" {
		t.Fatalf("text = %+v", text)
	}
}
