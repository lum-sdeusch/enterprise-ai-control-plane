// Package pipeline is the enforcement orchestrator. For every phase it runs the guardrails,
// loads the conversation's taint, asks OPA for one decision, applies redactions, handles
// approvals, records the audit event, and fails closed when any of those is unavailable.
//
// It knows nothing about gRPC or HTTP; internal/extmcp and internal/llm translate the wire
// formats into these calls.
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/approval"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/audit"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/config"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/guardrails"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/identity"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/mcpjson"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/pinning"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/policy"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/taint"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/telemetry"
)

type Deps struct {
	Cfg          config.Config
	Policy       policy.Client
	Guardrails   guardrails.Client
	Taint        taint.Store
	Approvals    approval.Store
	Audit        audit.Writer
	Now          func() time.Time
	AgentClients func() map[string]string
}

type Pipeline struct {
	d     Deps
	cache *analysisCache
}

func New(d Deps) *Pipeline {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.AgentClients == nil {
		d.AgentClients = func() map[string]string { return map[string]string{} }
	}
	return &Pipeline{d: d, cache: newAnalysisCache(d.Cfg.RouteCacheTTL, 2048)}
}

// Caller is the resolved identity for one call.
type Caller struct {
	Subject        identity.Subject
	Agent          string
	ConversationID string
	TraceID        string
}

// Resolve derives the agent from the actor chain and settles the conversation id, falling back
// to the subject-and-agent pair when the client sent none (ADR 0027).
func (p *Pipeline) Resolve(ctx context.Context, subject identity.Subject, conversationID string) Caller {
	agent := identity.AgentOf(subject, p.d.AgentClients())
	if conversationID == "" {
		conversationID = "fallback:" + subject.Sub + "|" + agent
	}
	traceID, _ := telemetry.IDs(ctx)
	return Caller{Subject: subject, Agent: agent, ConversationID: conversationID, TraceID: traceID}
}

type Action int

const (
	Pass Action = iota
	Mutate
	Deny
	ApprovalRequired
)

func (a Action) String() string {
	switch a {
	case Pass:
		return "pass"
	case Mutate:
		return "mutate"
	case Deny:
		return "deny"
	case ApprovalRequired:
		return "approval_required"
	}
	return "unknown"
}

type Outcome struct {
	Action   Action
	Mutated  []byte
	Code     string
	Reason   string
	Approval *approval.Request
	Decision *policy.Decision
}

// --- tool call: request phase -------------------------------------------------------------------

func (p *Pipeline) ToolCallRequest(ctx context.Context, c Caller, server, tool string, params mcpjson.ToolCallParams) Outcome {
	start := p.d.Now()
	ctx, span := telemetry.Tracer().Start(ctx, "eacp.tool.request", trace.WithAttributes(
		attribute.String("mcp.method.name", "tools/call"),
		attribute.String("gen_ai.tool.name", tool),
		attribute.String("eacp.server", server),
	))
	defer span.End()
	ev := p.baseEvent(ctx, c, "request")
	ev.Server, ev.Tool = server, tool

	if reason, quarantined, err := p.d.Taint.Quarantined(ctx, server); err != nil {
		return p.failClosed(ctx, span, ev, start, "state store unavailable", err)
	} else if quarantined {
		ev.Decision, ev.PinStatus, ev.Reasons = "quarantine", "quarantined", []string{"server_quarantined"}
		return p.finish(ctx, span, ev, start, Outcome{Action: Deny, Code: "server_quarantined", Reason: "server quarantined: " + reason})
	}

	leaves := mcpjson.StringLeaves("arguments", params.Arguments)
	an, err := p.d.Guardrails.Analyze(ctx, guardrails.AnalyzeRequest{
		Phase: "request", Texts: leaves,
		Context: map[string]any{"server": server, "tool": tool, "conversation_id": c.ConversationID},
	})
	if err != nil {
		return p.failClosed(ctx, span, ev, start, "guardrails unavailable", err)
	}
	ev.Findings = summarize(an)

	labels, err := p.d.Taint.Labels(ctx, c.ConversationID)
	if err != nil {
		return p.failClosed(ctx, span, ev, start, "state store unavailable", err)
	}
	ev.TaintBefore, ev.TaintAfter = encodeLabels(labels), encodeLabels(labels)

	pin := map[string]string{}
	if h, ok, err := p.d.Taint.ObservedPin(ctx, server, "tools"); err != nil {
		return p.failClosed(ctx, span, ev, start, "state store unavailable", err)
	} else if ok {
		pin["tools"] = h
		ev.PinStatus = "observed"
	} else {
		pin = nil
		ev.PinStatus = "unobserved"
	}

	hash := approval.RequestHash(c.Subject.Sub, server, tool, params.Arguments)
	existing, err := p.d.Approvals.FindByHash(ctx, hash)
	if err != nil && !errors.Is(err, approval.ErrNotFound) {
		return p.failClosed(ctx, span, ev, start, "approval store unavailable", err)
	}
	var apprIn *policy.ApprovalInput
	if existing != nil && existing.Status == approval.StatusApproved && !existing.Expired(start) {
		apprIn = &policy.ApprovalInput{Approved: true, Approver: existing.Approver, ApproverGroups: existing.ApproverGroups}
		ev.Approver = existing.Approver
	}

	in := policy.Input{
		Phase: "request", Subject: c.Subject, Agent: c.Agent,
		Tool: &policy.ToolRef{Server: server, Name: tool}, Arguments: params.Arguments,
		Findings: an.Findings, Taint: labels, Pinning: pin, Approval: apprIn,
		ConversationID: c.ConversationID, TraceID: c.TraceID,
	}
	dec, err := p.d.Policy.Decide(ctx, in)
	if err != nil {
		return p.failClosed(ctx, span, ev, start, "policy unavailable", err)
	}
	applyDecision(ev, dec)
	span.SetAttributes(attribute.String("eacp.policy.revision", dec.PolicyRevision), attribute.Bool("eacp.policy.allow", dec.Allow))

	if !dec.Allow && dec.RequireApproval != nil && len(dec.Reasons) == 0 {
		req := existing
		if req == nil || req.Status != approval.StatusPending || req.Expired(start) {
			req = &approval.Request{
				ID: approval.NewID(), RequestHash: hash,
				Subject: c.Subject.Sub, Team: c.Subject.Team, Agent: c.Agent, Server: server, Tool: tool,
				Summary: summarizeArgs(params.Arguments), ApproverGroup: dec.RequireApproval.ApproverGroup,
				AllowSelf: dec.RequireApproval.AllowSelf, Reason: dec.RequireApproval.Reason,
				Status: approval.StatusPending, ConversationID: c.ConversationID, TraceID: c.TraceID,
				CreatedAt: start, ExpiresAt: start.Add(p.d.Cfg.ApprovalTTL),
			}
			if err := p.d.Approvals.Create(ctx, req); err != nil {
				return p.failClosed(ctx, span, ev, start, "approval store unavailable", err)
			}
		}
		ev.Decision = "approval_required"
		return p.finish(ctx, span, ev, start, Outcome{
			Action: ApprovalRequired, Code: "approval_required", Approval: req, Decision: dec,
			Reason: fmt.Sprintf("approval required from %s: %s", req.ApproverGroup, p.ApprovalURL(req)),
		})
	}
	if !dec.Allow {
		ev.Decision = "deny"
		code, msg := firstReason(dec)
		return p.finish(ctx, span, ev, start, Outcome{Action: Deny, Code: code, Reason: msg, Decision: dec})
	}
	ev.Decision = "allow"
	if apprIn != nil {
		ev.Decision = "approved"
	}
	if len(dec.Redactions) > 0 {
		for _, f := range dec.Redactions {
			if red, ok := an.Redacted[f.Path]; ok {
				mcpjson.SetLeaf(params.Arguments, "arguments", f.Path, red)
			}
		}
		ev.Redactions = uint16(len(dec.Redactions))
		b, err := params.Marshal()
		if err != nil {
			return p.failClosed(ctx, span, ev, start, "cannot serialise redacted arguments", err)
		}
		p.captureContent(span, "gen_ai.tool.call.arguments", mcpjson.StringLeaves("arguments", params.Arguments))
		return p.finish(ctx, span, ev, start, Outcome{Action: Mutate, Mutated: b, Decision: dec})
	}
	p.captureContent(span, "gen_ai.tool.call.arguments", leaves)
	return p.finish(ctx, span, ev, start, Outcome{Action: Pass, Decision: dec})
}

// --- tool call: response phase ------------------------------------------------------------------

func (p *Pipeline) ToolCallResponse(ctx context.Context, c Caller, server, tool string, result mcpjson.ToolResult) Outcome {
	start := p.d.Now()
	ctx, span := telemetry.Tracer().Start(ctx, "eacp.tool.response", trace.WithAttributes(
		attribute.String("mcp.method.name", "tools/call"),
		attribute.String("gen_ai.tool.name", tool),
		attribute.String("eacp.server", server),
	))
	defer span.End()
	ev := p.baseEvent(ctx, c, "response")
	ev.Server, ev.Tool = server, tool

	texts := result.Texts()
	an, err := p.d.Guardrails.Analyze(ctx, guardrails.AnalyzeRequest{
		Phase: "response", Texts: texts,
		Context: map[string]any{"server": server, "tool": tool, "conversation_id": c.ConversationID},
	})
	if err != nil {
		return p.failClosed(ctx, span, ev, start, "guardrails unavailable", err)
	}
	ev.Findings = summarize(an)

	labels, err := p.d.Taint.Labels(ctx, c.ConversationID)
	if err != nil {
		return p.failClosed(ctx, span, ev, start, "state store unavailable", err)
	}
	ev.TaintBefore = encodeLabels(labels)

	pin := map[string]string{}
	if h, ok, err := p.d.Taint.ObservedPin(ctx, server, "tools"); err != nil {
		return p.failClosed(ctx, span, ev, start, "state store unavailable", err)
	} else if ok {
		pin["tools"] = h
		ev.PinStatus = "observed"
	} else {
		pin = nil
		ev.PinStatus = "unobserved"
	}

	dec, err := p.d.Policy.Decide(ctx, policy.Input{
		Phase: "response", Subject: c.Subject, Agent: c.Agent,
		Tool: &policy.ToolRef{Server: server, Name: tool},
		Findings: an.Findings, Taint: labels, Pinning: pin,
		ConversationID: c.ConversationID, TraceID: c.TraceID,
	})
	if err != nil {
		return p.failClosed(ctx, span, ev, start, "policy unavailable", err)
	}
	applyDecision(ev, dec)

	// Taint is updated even when the result is blocked: untrusted content entered the
	// conversation whether or not the model got to see it.
	if len(dec.TaintUpdates) > 0 {
		if err := p.d.Taint.Merge(ctx, c.ConversationID, dec.TaintUpdates); err != nil {
			return p.failClosed(ctx, span, ev, start, "state store unavailable", err)
		}
		for k, v := range dec.TaintUpdates {
			if v {
				labels[k] = true
			}
		}
	}
	ev.TaintAfter = encodeLabels(labels)

	if !dec.Allow {
		ev.Decision = "deny"
		code, msg := firstReason(dec)
		return p.finish(ctx, span, ev, start, Outcome{Action: Deny, Code: code, Reason: msg, Decision: dec})
	}
	ev.Decision = "allow"
	if len(dec.Redactions) > 0 {
		for _, f := range dec.Redactions {
			if red, ok := an.Redacted[f.Path]; ok {
				result.SetText(f.Path, red)
			}
		}
		ev.Redactions = uint16(len(dec.Redactions))
		b, err := result.Marshal()
		if err != nil {
			return p.failClosed(ctx, span, ev, start, "cannot serialise redacted result", err)
		}
		p.captureContent(span, "gen_ai.tool.call.result", result.Texts())
		return p.finish(ctx, span, ev, start, Outcome{Action: Mutate, Mutated: b, Decision: dec})
	}
	p.captureContent(span, "gen_ai.tool.call.result", texts)
	return p.finish(ctx, span, ev, start, Outcome{Action: Pass, Decision: dec})
}

// --- listings: pin check and entitlement filter -----------------------------------------------

// Listing verifies the pinned hash for a */list result of one server and, for tools, filters the
// items to what this caller may see. A hash mismatch quarantines the server and empties the
// listing so a poisoned description never reaches the model.
func (p *Pipeline) Listing(ctx context.Context, c Caller, server, kind string, items []map[string]any) ([]map[string]any, Outcome) {
	start := p.d.Now()
	ctx, span := telemetry.Tracer().Start(ctx, "eacp.listing", trace.WithAttributes(
		attribute.String("mcp.method.name", kind+"/list"),
		attribute.String("eacp.server", server),
	))
	defer span.End()
	ev := p.baseEvent(ctx, c, "list")
	ev.Server = server

	hash, err := pinning.Hash(items)
	if err != nil {
		return nil, p.failClosed(ctx, span, ev, start, "cannot hash listing", err)
	}
	if err := p.d.Taint.SetObservedPin(ctx, server, kind, hash); err != nil {
		return nil, p.failClosed(ctx, span, ev, start, "state store unavailable", err)
	}
	pinned, err := p.d.Policy.PinnedHash(ctx, server, kind)
	if errors.Is(err, policy.ErrUndefined) {
		// A server that is not onboarded is invisible.
		ev.Decision, ev.PinStatus, ev.Reasons = "deny", "unknown_server", []string{"unknown_server"}
		return []map[string]any{}, p.finish(ctx, span, ev, start, Outcome{Action: Mutate, Code: "unknown_server"})
	}
	if err != nil {
		return nil, p.failClosed(ctx, span, ev, start, "policy unavailable", err)
	}
	if pinned != hash {
		reason := fmt.Sprintf("%s/list hash %s does not match pinned %s", kind, hash, pinned)
		if err := p.d.Taint.Quarantine(ctx, server, reason); err != nil {
			return nil, p.failClosed(ctx, span, ev, start, "state store unavailable", err)
		}
		ev.Decision, ev.PinStatus, ev.Reasons = "quarantine", "mismatch", []string{"server_quarantined"}
		span.SetAttributes(attribute.String("eacp.pin.status", "mismatch"))
		return []map[string]any{}, p.finish(ctx, span, ev, start, Outcome{Action: Mutate, Code: "server_quarantined", Reason: reason})
	}
	ev.PinStatus = "match"
	if _, quarantined, err := p.d.Taint.Quarantined(ctx, server); err == nil && quarantined {
		// The listing matches the manifest again: a new manifest was merged. Lift.
		_ = p.d.Taint.Lift(ctx, server)
		ev.PinStatus = "lifted"
	}

	if kind != "tools" {
		ev.Decision = "allow"
		return items, p.finish(ctx, span, ev, start, Outcome{Action: Pass})
	}
	visible, err := p.d.Policy.VisibleTools(ctx, policy.Input{
		Phase: "list", Subject: c.Subject, Agent: c.Agent, Findings: guardrails.Empty(),
		Taint: map[string]bool{}, ConversationID: c.ConversationID, TraceID: c.TraceID,
	})
	if err != nil {
		return nil, p.failClosed(ctx, span, ev, start, "policy unavailable", err)
	}
	allowed := map[string]bool{}
	for _, v := range visible {
		allowed[v] = true
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, it := range items {
		name, _ := it["name"].(string)
		if allowed[server+"/"+mcpjson.BareName(name, server)] {
			filtered = append(filtered, it)
		}
	}
	ev.Decision = "allow"
	ev.Findings = fmt.Sprintf(`{"advertised":%d,"visible":%d}`, len(items), len(filtered))
	return filtered, p.finish(ctx, span, ev, start, Outcome{Action: Mutate})
}

// --- LLM path ---------------------------------------------------------------------------------

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Choice struct {
	Message Message `json:"message"`
}

type LLMAction struct {
	Type     string // pass | mask | reject
	Messages []Message
	Choices  []Choice
	Status   int
	Body     string
	Reason   string
}

// RouteLLM answers the gateway's virtual-model callout: which model class may see this prompt.
// Any failure routes in-house, never outward.
func (p *Pipeline) RouteLLM(ctx context.Context, c Caller, requestedModel string, messages []Message) (string, *policy.Decision) {
	start := p.d.Now()
	ctx, span := telemetry.Tracer().Start(ctx, "eacp.llm.route")
	defer span.End()
	ev := p.baseEvent(ctx, c, "llm_route")
	ev.ModelRequested = requestedModel

	texts := messageTexts(messages)
	an, hash, err := p.analyzeCached(ctx, "llm_route", texts, c.ConversationID)
	if err != nil {
		ev.ModelSelected = p.d.Cfg.RouteModelInhouse
		p.failClosed(ctx, span, ev, start, "guardrails unavailable", err)
		return p.d.Cfg.RouteModelInhouse, nil
	}
	ev.Findings = summarize(an)
	labels, err := p.d.Taint.Labels(ctx, c.ConversationID)
	if err != nil {
		ev.ModelSelected = p.d.Cfg.RouteModelInhouse
		p.failClosed(ctx, span, ev, start, "state store unavailable", err)
		return p.d.Cfg.RouteModelInhouse, nil
	}
	ev.TaintBefore, ev.TaintAfter = encodeLabels(labels), encodeLabels(labels)
	dec, err := p.d.Policy.Decide(ctx, policy.Input{
		Phase: "llm_route", Subject: c.Subject, Agent: c.Agent, Findings: an.Findings, Taint: labels,
		LLM: &policy.LLMInput{RequestedModel: requestedModel}, ConversationID: c.ConversationID, TraceID: c.TraceID,
	})
	if err != nil {
		ev.ModelSelected = p.d.Cfg.RouteModelInhouse
		p.failClosed(ctx, span, ev, start, "policy unavailable", err)
		return p.d.Cfg.RouteModelInhouse, nil
	}
	applyDecision(ev, dec)
	model, class := p.modelFor(dec)
	if !dec.Allow {
		// No class may see this prompt for this team and agent; the prompt guard rejects it.
		// Route in-house so nothing leaves while that happens.
		model, class = p.d.Cfg.RouteModelInhouse, "inhouse"
		ev.Decision = "deny"
	} else {
		ev.Decision = "allow"
	}
	ev.ModelSelected = model
	_ = p.d.Taint.SetRoute(ctx, c.ConversationID, hash, class)
	span.SetAttributes(attribute.String("eacp.classification", dec.RouteLabel), attribute.String("eacp.route.model", model))
	p.finish(ctx, span, ev, start, Outcome{Action: Pass, Decision: dec})
	return model, dec
}

// GuardLLMRequest is the prompt-guard request phase: pass, mask PII bound for an external
// model, or reject secrets, injection and class violations.
func (p *Pipeline) GuardLLMRequest(ctx context.Context, c Caller, requestedModel string, messages []Message) LLMAction {
	start := p.d.Now()
	ctx, span := telemetry.Tracer().Start(ctx, "eacp.llm.request")
	defer span.End()
	ev := p.baseEvent(ctx, c, "llm_request")
	ev.ModelRequested = requestedModel

	texts := messageTexts(messages)
	an, hash, err := p.analyzeCached(ctx, "llm_request", texts, c.ConversationID)
	if err != nil {
		p.failClosed(ctx, span, ev, start, "guardrails unavailable", err)
		return rejectAction("guardrails unavailable")
	}
	ev.Findings = summarize(an)
	labels, err := p.d.Taint.Labels(ctx, c.ConversationID)
	if err != nil {
		p.failClosed(ctx, span, ev, start, "state store unavailable", err)
		return rejectAction("state store unavailable")
	}
	ev.TaintBefore, ev.TaintAfter = encodeLabels(labels), encodeLabels(labels)

	class, ok, _ := p.d.Taint.Route(ctx, c.ConversationID, hash)
	if !ok {
		class = p.classOfModel(requestedModel)
		if class == "" {
			// The prompt guard ran before the routing callout: decide the route now.
			routeDec, err := p.d.Policy.Decide(ctx, policy.Input{
				Phase: "llm_route", Subject: c.Subject, Agent: c.Agent, Findings: an.Findings, Taint: labels,
				LLM: &policy.LLMInput{RequestedModel: requestedModel}, ConversationID: c.ConversationID, TraceID: c.TraceID,
			})
			if err != nil {
				p.failClosed(ctx, span, ev, start, "policy unavailable", err)
				return rejectAction("policy unavailable")
			}
			_, class = p.modelFor(routeDec)
		}
	}
	dec, err := p.d.Policy.Decide(ctx, policy.Input{
		Phase: "llm_request", Subject: c.Subject, Agent: c.Agent, Findings: an.Findings, Taint: labels,
		LLM: &policy.LLMInput{RequestedModel: requestedModel, TargetClass: class}, ConversationID: c.ConversationID, TraceID: c.TraceID,
	})
	if err != nil {
		p.failClosed(ctx, span, ev, start, "policy unavailable", err)
		return rejectAction("policy unavailable")
	}
	applyDecision(ev, dec)
	ev.ModelSelected = class
	if !dec.Allow {
		ev.Decision = "deny"
		code, msg := firstReason(dec)
		p.finish(ctx, span, ev, start, Outcome{Action: Deny, Code: code, Reason: msg, Decision: dec})
		return LLMAction{Type: "reject", Status: 403, Reason: msg, Body: errorBody(code, msg, c.ConversationID)}
	}
	ev.Decision = "allow"
	if len(dec.Redactions) > 0 {
		masked := cloneMessages(messages)
		for _, f := range dec.Redactions {
			if red, ok := an.Redacted[f.Path]; ok {
				setMessageText(masked, f.Path, red)
			}
		}
		ev.Redactions = uint16(len(dec.Redactions))
		p.captureContent(span, "gen_ai.input.messages", messageTexts(masked))
		p.finish(ctx, span, ev, start, Outcome{Action: Mutate, Decision: dec})
		return LLMAction{Type: "mask", Messages: masked, Reason: "pii redacted before external model"}
	}
	p.captureContent(span, "gen_ai.input.messages", texts)
	p.finish(ctx, span, ev, start, Outcome{Action: Pass, Decision: dec})
	return LLMAction{Type: "pass"}
}

// GuardLLMResponse is the prompt-guard response phase: pass or mask. The webhook protocol has
// no reject here, so a denied completion is masked to empty content.
func (p *Pipeline) GuardLLMResponse(ctx context.Context, c Caller, requestedModel string, choices []Choice) LLMAction {
	start := p.d.Now()
	ctx, span := telemetry.Tracer().Start(ctx, "eacp.llm.response")
	defer span.End()
	ev := p.baseEvent(ctx, c, "llm_response")
	ev.ModelRequested = requestedModel

	texts := map[string]string{}
	for i, ch := range choices {
		for k, v := range mcpjson.StringLeaves(fmt.Sprintf("choices.%d.message.content", i), ch.Message.Content) {
			texts[k] = v
		}
		if s, ok := ch.Message.Content.(string); ok {
			texts[fmt.Sprintf("choices.%d.message.content", i)] = s
		}
	}
	an, err := p.d.Guardrails.Analyze(ctx, guardrails.AnalyzeRequest{Phase: "llm_response", Texts: texts, Context: map[string]any{"conversation_id": c.ConversationID}})
	if err != nil {
		p.failClosed(ctx, span, ev, start, "guardrails unavailable", err)
		return LLMAction{Type: "mask", Choices: blankChoices(choices), Reason: "guardrails unavailable"}
	}
	ev.Findings = summarize(an)
	labels, err := p.d.Taint.Labels(ctx, c.ConversationID)
	if err != nil {
		p.failClosed(ctx, span, ev, start, "state store unavailable", err)
		return LLMAction{Type: "mask", Choices: blankChoices(choices), Reason: "state store unavailable"}
	}
	dec, err := p.d.Policy.Decide(ctx, policy.Input{
		Phase: "llm_response", Subject: c.Subject, Agent: c.Agent, Findings: an.Findings, Taint: labels,
		LLM: &policy.LLMInput{RequestedModel: requestedModel}, ConversationID: c.ConversationID, TraceID: c.TraceID,
	})
	if err != nil {
		p.failClosed(ctx, span, ev, start, "policy unavailable", err)
		return LLMAction{Type: "mask", Choices: blankChoices(choices), Reason: "policy unavailable"}
	}
	applyDecision(ev, dec)
	if !dec.Allow {
		ev.Decision = "deny"
		_, msg := firstReason(dec)
		p.finish(ctx, span, ev, start, Outcome{Action: Deny, Decision: dec})
		return LLMAction{Type: "mask", Choices: blankChoices(choices), Reason: msg}
	}
	ev.Decision = "allow"
	if len(dec.Redactions) > 0 {
		masked := make([]Choice, len(choices))
		for i, ch := range choices {
			masked[i] = Choice{Message: Message{Role: ch.Message.Role, Content: ch.Message.Content}}
		}
		for _, f := range dec.Redactions {
			if red, ok := an.Redacted[f.Path]; ok {
				var idx int
				if _, err := fmt.Sscanf(f.Path, "choices.%d.", &idx); err == nil && idx < len(masked) {
					masked[idx].Message.Content = red
				}
			}
		}
		ev.Redactions = uint16(len(dec.Redactions))
		p.finish(ctx, span, ev, start, Outcome{Action: Mutate, Decision: dec})
		return LLMAction{Type: "mask", Choices: masked, Reason: "secrets redacted from completion"}
	}
	p.captureContent(span, "gen_ai.output.messages", texts)
	p.finish(ctx, span, ev, start, Outcome{Action: Pass, Decision: dec})
	return LLMAction{Type: "pass"}
}

// ApprovalURL is the link a requester receives in the deny message.
func (p *Pipeline) ApprovalURL(r *approval.Request) string {
	return strings.TrimRight(p.d.Cfg.PublicURL, "/") + "/approvals/" + r.ID
}

// --- helpers ------------------------------------------------------------------------------------

func (p *Pipeline) modelFor(dec *policy.Decision) (model, class string) {
	switch dec.RouteTarget {
	case "eacp-inhouse", "":
		return p.d.Cfg.RouteModelInhouse, "inhouse"
	case "eacp-external":
		return p.d.Cfg.RouteModelExternal, "external"
	default:
		return dec.RouteTarget, "external"
	}
}

func (p *Pipeline) classOfModel(model string) string {
	switch model {
	case p.d.Cfg.RouteModelInhouse:
		return "inhouse"
	case p.d.Cfg.RouteModelExternal:
		return "external"
	}
	return ""
}

func (p *Pipeline) analyzeCached(ctx context.Context, phase string, texts map[string]string, conversationID string) (*guardrails.AnalyzeResponse, string, error) {
	hash := textsHash(texts)
	if an, ok := p.cache.get(hash); ok {
		return an, hash, nil
	}
	an, err := p.d.Guardrails.Analyze(ctx, guardrails.AnalyzeRequest{Phase: phase, Texts: texts, Context: map[string]any{"conversation_id": conversationID}})
	if err != nil {
		return nil, hash, err
	}
	p.cache.put(hash, an)
	return an, hash, nil
}

func (p *Pipeline) baseEvent(ctx context.Context, c Caller, phase string) *audit.Event {
	traceID, spanID := telemetry.IDs(ctx)
	if traceID == "" {
		traceID = c.TraceID
	}
	return &audit.Event{
		TS: audit.Timestamp(p.d.Now()), EventID: approval.NewID(), TraceID: traceID, SpanID: spanID,
		ConversationID: c.ConversationID, Phase: phase, Subject: c.Subject.Sub, Team: c.Subject.Team,
		Groups: c.Subject.Groups, Agent: c.Agent, ActorChain: identity.ActorChain(c.Subject),
		Reasons: []string{}, AdapterVersion: config.Version, Findings: "{}",
	}
}

// finish writes the audit record. If the record cannot be written and audit is required, the
// outcome becomes a denial: a decision without a record is not made (ADR 0009, 0010).
func (p *Pipeline) finish(ctx context.Context, span trace.Span, ev *audit.Event, start time.Time, out Outcome) Outcome {
	ev.LatencyMS = uint32(p.d.Now().Sub(start).Milliseconds())
	span.SetAttributes(
		attribute.String("eacp.policy.decision", ev.Decision),
		attribute.StringSlice("eacp.policy.reasons", ev.Reasons),
		attribute.String("eacp.conversation_id", ev.ConversationID),
	)
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.d.Cfg.AuditTimeout)
	defer cancel()
	if err := p.d.Audit.Write(actx, *ev); err != nil {
		span.RecordError(err)
		if p.d.Cfg.AuditRequired {
			return Outcome{Action: Deny, Code: "audit_unavailable", Reason: "audit store unavailable; failing closed", Decision: out.Decision}
		}
	}
	return out
}

func (p *Pipeline) failClosed(ctx context.Context, span trace.Span, ev *audit.Event, start time.Time, why string, err error) Outcome {
	span.RecordError(err)
	ev.Decision = "fail_closed"
	ev.Reasons = []string{why}
	ev.LatencyMS = uint32(p.d.Now().Sub(start).Milliseconds())
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.d.Cfg.AuditTimeout)
	defer cancel()
	_ = p.d.Audit.Write(actx, *ev)
	return Outcome{Action: Deny, Code: "fail_closed", Reason: why + "; failing closed"}
}

func (p *Pipeline) captureContent(span trace.Span, attr string, texts map[string]string) {
	if !p.d.Cfg.ContentCapture || len(texts) == 0 {
		return
	}
	keys := make([]string, 0, len(texts))
	for k := range texts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+texts[k])
	}
	span.SetAttributes(attribute.String(attr, strings.Join(parts, "\n")))
}

func firstReason(dec *policy.Decision) (string, string) {
	if len(dec.Reasons) == 0 {
		return "denied", "denied by policy"
	}
	return dec.Reasons[0].Code, dec.Reasons[0].Msg
}

func rejectAction(reason string) LLMAction {
	return LLMAction{Type: "reject", Status: 503, Reason: reason, Body: errorBody("fail_closed", reason+"; failing closed", "")}
}

func errorBody(code, msg, conversationID string) string {
	b, _ := json.Marshal(map[string]any{"error": map[string]any{"type": "eacp_policy", "code": code, "message": msg, "conversation_id": conversationID}})
	return string(b)
}

func messageTexts(messages []Message) map[string]string {
	out := map[string]string{}
	for i, m := range messages {
		base := fmt.Sprintf("messages.%d.content", i)
		if s, ok := m.Content.(string); ok {
			out[base] = s
			continue
		}
		for k, v := range mcpjson.StringLeaves(base, m.Content) {
			if strings.HasSuffix(k, ".text") {
				out[k] = v
			}
		}
	}
	return out
}

func cloneMessages(in []Message) []Message {
	out := make([]Message, len(in))
	for i, m := range in {
		out[i] = Message{Role: m.Role, Content: deepCopy(m.Content)}
	}
	return out
}

func deepCopy(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func setMessageText(messages []Message, path, value string) {
	var idx int
	rest := ""
	if _, err := fmt.Sscanf(path, "messages.%d.%s", &idx, &rest); err != nil || idx >= len(messages) {
		return
	}
	if rest == "content" {
		messages[idx].Content = value
		return
	}
	if blocks, ok := messages[idx].Content.([]any); ok {
		wrapper := map[string]any{"content": blocks}
		mcpjson.SetLeaf(wrapper, "messages."+fmt.Sprint(idx), path, value)
		messages[idx].Content = wrapper["content"]
	}
}

func blankChoices(in []Choice) []Choice {
	out := make([]Choice, len(in))
	for i, ch := range in {
		out[i] = Choice{Message: Message{Role: ch.Message.Role, Content: ""}}
	}
	return out
}

func encodeLabels(labels map[string]bool) string {
	b, _ := json.Marshal(labels)
	return string(b)
}

func summarize(an *guardrails.AnalyzeResponse) string {
	s := map[string]any{"pii": len(an.Findings.PII), "secrets": len(an.Findings.Secrets)}
	if an.Findings.Injection != nil {
		s["injection"] = an.Findings.Injection.Score
	}
	if an.Findings.Sensitivity != nil {
		s["sensitivity"] = an.Findings.Sensitivity.Label
	}
	if len(an.ModelVersions) > 0 {
		s["models"] = an.ModelVersions
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// summarizeArgs renders the argument keys and value lengths for the approval page; never the
// values themselves, which may be the PII the approval is about.
func summarizeArgs(args map[string]any) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		switch v := args[k].(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s (%d chars)", k, len(v)))
		default:
			parts = append(parts, k)
		}
	}
	return strings.Join(parts, ", ")
}

func textsHash(texts map[string]string) string {
	keys := make([]string, 0, len(texts))
	for k := range texts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(texts[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// applyDecision copies the decision's audit-relevant fields onto the event.
func applyDecision(e *audit.Event, dec *policy.Decision) {
	e.Reasons = dec.ReasonCodes()
	e.RouteLabel = dec.RouteLabel
	e.PolicyRevision = dec.PolicyRevision
}

// InHouseModel is the model name every failure path routes to.
func (p *Pipeline) InHouseModel() string { return p.d.Cfg.RouteModelInhouse }

// --- analysis cache -----------------------------------------------------------------------------

type analysisCache struct {
	mu   sync.Mutex
	ttl  time.Duration
	max  int
	data map[string]cacheEntry
}

type cacheEntry struct {
	an     *guardrails.AnalyzeResponse
	expiry time.Time
}

func newAnalysisCache(ttl time.Duration, max int) *analysisCache {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &analysisCache{ttl: ttl, max: max, data: map[string]cacheEntry{}}
}

func (c *analysisCache) get(key string) (*guardrails.AnalyzeResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok || time.Now().After(e.expiry) {
		delete(c.data, key)
		return nil, false
	}
	return e.an, true
}

func (c *analysisCache) put(key string, an *guardrails.AnalyzeResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.data) >= c.max {
		for k := range c.data { // drop an arbitrary entry; the cache is a latency optimisation only
			delete(c.data, k)
			break
		}
	}
	c.data[key] = cacheEntry{an: an, expiry: time.Now().Add(c.ttl)}
}
