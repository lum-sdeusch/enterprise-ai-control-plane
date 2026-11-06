// Package guardrails is the client for the Python guardrail service. The service returns
// findings; it never decides. Everything here is data for the policy.
package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Finding struct {
	Path  string  `json:"path,omitempty"`
	Type  string  `json:"type"`
	Start int     `json:"start"`
	End   int     `json:"end"`
	Score float64 `json:"score,omitempty"`
}

type Injection struct {
	Score      float64  `json:"score"`
	Label      string   `json:"label"`
	Categories []string `json:"categories"`
}

type Sensitivity struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

type Findings struct {
	PII         []Finding    `json:"pii"`
	Secrets     []Finding    `json:"secrets"`
	Injection   *Injection   `json:"injection,omitempty"`
	Sensitivity *Sensitivity `json:"sensitivity,omitempty"`
}

// Empty returns findings with empty (not nil) slices so the policy sees arrays, not null.
func Empty() Findings {
	return Findings{PII: []Finding{}, Secrets: []Finding{}}
}

func (f *Findings) normalize() {
	if f.PII == nil {
		f.PII = []Finding{}
	}
	if f.Secrets == nil {
		f.Secrets = []Finding{}
	}
	if f.Injection != nil && f.Injection.Categories == nil {
		f.Injection.Categories = []string{}
	}
}

type AnalyzeRequest struct {
	Phase   string            `json:"phase"`
	Texts   map[string]string `json:"texts"`
	Checks  []string          `json:"checks,omitempty"`
	Context map[string]any    `json:"context,omitempty"`
}

type AnalyzeResponse struct {
	Findings      Findings           `json:"findings"`
	Redacted      map[string]string  `json:"redacted"`
	ModelVersions map[string]string  `json:"model_versions"`
	LatencyMS     map[string]float64 `json:"latency_ms"`
}

type Client interface {
	Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error)
}

type HTTPClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{BaseURL: baseURL, HTTP: &http.Client{Timeout: timeout}}
}

func (c *HTTPClient) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	if len(req.Texts) == 0 {
		return &AnalyzeResponse{Findings: Empty(), Redacted: map[string]string{}}, nil
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/analyze", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("guardrails: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("guardrails: status %d", resp.StatusCode)
	}
	var out AnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("guardrails: decode: %w", err)
	}
	out.Findings.normalize()
	if out.Redacted == nil {
		out.Redacted = map[string]string{}
	}
	return &out, nil
}
