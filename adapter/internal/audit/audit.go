// Package audit writes one record per decision phase to the append-only audit store. The record
// shape mirrors eacp_audit.events in platform/telemetry/clickhouse.yaml. It carries summaries,
// never content: counts and labels of findings, the taint before and after, the reasons, the
// policy revision and the trace id that leads to the full trace.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type Event struct {
	TS             string   `json:"ts"`
	EventID        string   `json:"event_id"`
	TraceID        string   `json:"trace_id"`
	SpanID         string   `json:"span_id"`
	ConversationID string   `json:"conversation_id"`
	Phase          string   `json:"phase"`
	Subject        string   `json:"subject"`
	Team           string   `json:"team"`
	Groups         []string `json:"groups"`
	Agent          string   `json:"agent"`
	ActorChain     string   `json:"actor_chain"`
	Server         string   `json:"server"`
	Tool           string   `json:"tool"`
	ModelRequested string   `json:"model_requested"`
	ModelSelected  string   `json:"model_selected"`
	RouteLabel     string   `json:"route_label"`
	Decision       string   `json:"decision"`
	Reasons        []string `json:"reasons"`
	Redactions     uint16   `json:"redactions"`
	Findings       string   `json:"findings"`
	TaintBefore    string   `json:"taint_before"`
	TaintAfter     string   `json:"taint_after"`
	PinStatus      string   `json:"pin_status"`
	PolicyRevision string   `json:"policy_revision"`
	Approver       string   `json:"approver"`
	LatencyMS      uint32   `json:"latency_ms"`
	AdapterVersion string   `json:"adapter_version"`
}

// Timestamp formats a time the way ClickHouse DateTime64(3) parses it.
func Timestamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05.000") }

type Writer interface {
	Write(ctx context.Context, e Event) error
}

// Stdout writes JSON lines; the laptop profile and tests use it.
type Stdout struct {
	mu  sync.Mutex
	out io.Writer
}

func NewStdout() *Stdout { return &Stdout{out: os.Stdout} }

func (s *Stdout) Write(_ context.Context, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(struct {
		Kind string `json:"kind"`
		Event
	}{"eacp.audit", e})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(s.out, string(b))
	return err
}

// ClickHouse inserts synchronously over the HTTP interface with an insert-only user. The write
// is on the critical path on purpose: a decision without a record is not a decision this
// platform makes (EACP_AUDIT_REQUIRED).
type ClickHouse struct {
	URL      string
	User     string
	Password string
	HTTP     *http.Client
}

func NewClickHouse(baseURL, user, password string, timeout time.Duration) *ClickHouse {
	return &ClickHouse{URL: baseURL, User: user, Password: password, HTTP: &http.Client{Timeout: timeout}}
}

func (c *ClickHouse) Write(ctx context.Context, e Event) error {
	row, err := json.Marshal(e)
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("query", "INSERT INTO eacp_audit.events FORMAT JSONEachRow")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/?"+q.Encode(), bytes.NewReader(append(row, '\n')))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("audit: clickhouse status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Multi fans out to several writers and returns the first error; used to keep stdout alongside
// ClickHouse in the laptop profile.
type Multi []Writer

func (m Multi) Write(ctx context.Context, e Event) error {
	var first error
	for _, w := range m {
		if err := w.Write(ctx, e); err != nil && first == nil {
			first = err
		}
	}
	return first
}
