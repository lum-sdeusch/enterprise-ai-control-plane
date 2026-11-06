// Package llm serves the gateway's LLM-side hooks: the virtual-model routing callout and the
// prompt-guard webhook (request and response phases). Wire formats follow the agentgateway
// guardrail webhook OpenAPI (PassAction / MaskAction / RejectAction) and the callout contract
// ({"model": "<target>"}).
package llm

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/identity"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/pipeline"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/telemetry"
)

type Handler struct {
	P   *pipeline.Pipeline
	Log *slog.Logger
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/route", h.route)
	mux.HandleFunc("POST /v1/llm/request", h.request)
	mux.HandleFunc("POST /v1/llm/response", h.response)
}

type routeRequest struct {
	Model    string             `json:"model"`
	Messages []pipeline.Message `json:"messages"`
}

type promptRequest struct {
	Body struct {
		Messages []pipeline.Message `json:"messages"`
	} `json:"body"`
}

type responseRequest struct {
	Body struct {
		Choices []pipeline.Choice `json:"choices"`
	} `json:"body"`
}

func (h *Handler) caller(r *http.Request) pipeline.Caller {
	ctx := telemetry.Extract(r.Context(), map[string]string{"traceparent": r.Header.Get("traceparent"), "baggage": r.Header.Get("baggage")})
	subject := identity.FromHeaders(r.Header.Get)
	return h.P.Resolve(ctx, subject, r.Header.Get("x-eacp-conversation-id"))
}

func (h *Handler) route(w http.ResponseWriter, r *http.Request) {
	var req routeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		// A malformed callout body still gets a safe answer: in-house.
		writeJSON(w, http.StatusOK, map[string]string{"model": h.P.InHouseModel()})
		return
	}
	c := h.caller(r)
	model, _ := h.P.RouteLLM(r.Context(), c, req.Model, req.Messages)
	writeJSON(w, http.StatusOK, map[string]string{"model": model})
}

func (h *Handler) request(w http.ResponseWriter, r *http.Request) {
	var req promptRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{"status_code": 400, "body": "malformed prompt", "reason": "decode"}})
		return
	}
	c := h.caller(r)
	act := h.P.GuardLLMRequest(r.Context(), c, r.Header.Get("x-eacp-model"), req.Body.Messages)
	switch act.Type {
	case "mask":
		msgs := make([]map[string]any, len(act.Messages))
		for i, m := range act.Messages {
			msgs[i] = map[string]any{"role": m.Role, "content": m.Content}
		}
		writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{"body": map[string]any{"messages": msgs}, "reason": act.Reason}})
	case "reject":
		writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{"status_code": act.Status, "body": act.Body, "reason": act.Reason}})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{}})
	}
}

func (h *Handler) response(w http.ResponseWriter, r *http.Request) {
	var req responseRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{"body": map[string]any{"choices": []any{}}, "reason": "decode"}})
		return
	}
	c := h.caller(r)
	act := h.P.GuardLLMResponse(r.Context(), c, r.Header.Get("x-eacp-model"), req.Body.Choices)
	if act.Type == "mask" {
		choices := make([]map[string]any, len(act.Choices))
		for i, ch := range act.Choices {
			choices[i] = map[string]any{"message": map[string]any{"role": ch.Message.Role, "content": ch.Message.Content}}
		}
		writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{"body": map[string]any{"choices": choices}, "reason": act.Reason}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": map[string]any{}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
