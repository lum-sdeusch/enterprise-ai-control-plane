// Package policy is the client for the OPA sidecar. The adapter never decides anything itself;
// it builds the input document described in policies/README.md and applies the decision.
package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/guardrails"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/identity"
)

type ToolRef struct {
	Server string `json:"server"`
	Name   string `json:"name"`
}

type ApprovalInput struct {
	Approved       bool     `json:"approved"`
	Approver       string   `json:"approver,omitempty"`
	ApproverGroups []string `json:"approver_groups"`
}

type LLMInput struct {
	RequestedModel string `json:"requested_model,omitempty"`
	TargetClass    string `json:"target_class,omitempty"`
}

// Input is the document sent as `input` to data.eacp.decision.
type Input struct {
	Phase          string              `json:"phase"`
	Subject        identity.Subject    `json:"subject"`
	Agent          string              `json:"agent"`
	Tool           *ToolRef            `json:"tool,omitempty"`
	Arguments      map[string]any      `json:"arguments,omitempty"`
	Findings       guardrails.Findings `json:"findings"`
	Taint          map[string]bool     `json:"taint"`
	Pinning        map[string]string   `json:"pinning,omitempty"`
	Approval       *ApprovalInput      `json:"approval,omitempty"`
	LLM            *LLMInput           `json:"llm,omitempty"`
	ConversationID string              `json:"conversation_id"`
	TraceID        string              `json:"trace_id"`
}

type Reason struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

type ApprovalSpec struct {
	ApproverGroup string `json:"approver_group"`
	AllowSelf     bool   `json:"allow_self"`
	Reason        string `json:"reason"`
}

// Decision mirrors the output contract in policies/README.md.
type Decision struct {
	Allow           bool                 `json:"allow"`
	Reasons         []Reason             `json:"reasons"`
	Redactions      []guardrails.Finding `json:"redactions"`
	RequireApproval *ApprovalSpec        `json:"require_approval"`
	RouteLabel      string               `json:"route_label"`
	RouteTarget     string               `json:"route_target"`
	TaintUpdates    map[string]bool      `json:"taint_updates"`
	AuditTags       []string             `json:"audit_tags"`
	PolicyRevision  string               `json:"policy_revision"`
}

func (d *Decision) ReasonCodes() []string {
	out := make([]string, 0, len(d.Reasons))
	for _, r := range d.Reasons {
		out = append(out, r.Code)
	}
	return out
}

type Client interface {
	Decide(ctx context.Context, in Input) (*Decision, error)
	VisibleTools(ctx context.Context, in Input) ([]string, error)
	PinnedHash(ctx context.Context, server, kind string) (string, error)
	AgentClients(ctx context.Context) (map[string]string, error)
}

var ErrUndefined = errors.New("policy: decision undefined")

type HTTPClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{BaseURL: baseURL, HTTP: &http.Client{Timeout: timeout}}
}

func (c *HTTPClient) query(ctx context.Context, path string, input any, out any) error {
	var body []byte
	var err error
	method := http.MethodGet
	if input != nil {
		method = http.MethodPost
		body, err = json.Marshal(map[string]any{"input": input})
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/v1/data/"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("policy: status %d", resp.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("policy: decode: %w", err)
	}
	if len(envelope.Result) == 0 {
		return ErrUndefined
	}
	return json.Unmarshal(envelope.Result, out)
}

func (c *HTTPClient) Decide(ctx context.Context, in Input) (*Decision, error) {
	var d Decision
	if err := c.query(ctx, "eacp/decision", in, &d); err != nil {
		return nil, err
	}
	if d.TaintUpdates == nil {
		d.TaintUpdates = map[string]bool{}
	}
	return &d, nil
}

func (c *HTTPClient) VisibleTools(ctx context.Context, in Input) ([]string, error) {
	var tools []string
	if err := c.query(ctx, "eacp/entitlement/visible_tools", in, &tools); err != nil {
		if errors.Is(err, ErrUndefined) {
			return []string{}, nil
		}
		return nil, err
	}
	return tools, nil
}

func (c *HTTPClient) PinnedHash(ctx context.Context, server, kind string) (string, error) {
	var hash string
	path := fmt.Sprintf("eacp/tools/%s/pins/%s", url.PathEscape(server), url.PathEscape(kind))
	if err := c.query(ctx, path, nil, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (c *HTTPClient) AgentClients(ctx context.Context) (map[string]string, error) {
	var m map[string]string
	if err := c.query(ctx, "eacp/agent_clients", nil, &m); err != nil {
		return nil, err
	}
	return m, nil
}
