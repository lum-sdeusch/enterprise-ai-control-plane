// Package approval holds pending human approvals. A request is identified by a hash of who is
// asking, which tool, and the exact arguments, so the agent's retry of the same call finds the
// approval without carrying a token it cannot carry.
package approval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"
)

var ErrNotFound = errors.New("approval: not found")

type Request struct {
	ID             string    `json:"id"`
	RequestHash    string    `json:"request_hash"`
	Subject        string    `json:"subject"`
	Team           string    `json:"team"`
	Agent          string    `json:"agent"`
	Server         string    `json:"server"`
	Tool           string    `json:"tool"`
	Summary        string    `json:"summary"`
	ApproverGroup  string    `json:"approver_group"`
	AllowSelf      bool      `json:"allow_self"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	Approver       string    `json:"approver,omitempty"`
	ApproverGroups []string  `json:"approver_groups,omitempty"`
	ConversationID string    `json:"conversation_id"`
	TraceID        string    `json:"trace_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	DecidedAt      time.Time `json:"decided_at,omitempty"`
}

func (r *Request) Expired(now time.Time) bool { return now.After(r.ExpiresAt) }

type Store interface {
	Create(ctx context.Context, r *Request) error
	Get(ctx context.Context, id string) (*Request, error)
	FindByHash(ctx context.Context, hash string) (*Request, error)
	Decide(ctx context.Context, id, approver string, approverGroups []string, approve bool, now time.Time) (*Request, error)
}

// RequestHash binds an approval to one caller, one tool and one exact argument set.
func RequestHash(subject, server, tool string, args map[string]any) string {
	b, _ := json.Marshal(args) // sorted keys, canonical enough for equality
	sum := sha256.Sum256([]byte(subject + "\x00" + server + "\x00" + tool + "\x00" + string(b)))
	return hex.EncodeToString(sum[:])
}

func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// --- in-memory ---------------------------------------------------------------------------------

type Memory struct {
	mu     sync.Mutex
	byID   map[string]*Request
	byHash map[string]string
}

func NewMemory() *Memory { return &Memory{byID: map[string]*Request{}, byHash: map[string]string{}} }

func (m *Memory) Create(_ context.Context, r *Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.byID[r.ID] = &cp
	m.byHash[r.RequestHash] = r.ID
	return nil
}

func (m *Memory) Get(_ context.Context, id string) (*Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (m *Memory) FindByHash(ctx context.Context, hash string) (*Request, error) {
	m.mu.Lock()
	id, ok := m.byHash[hash]
	m.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	return m.Get(ctx, id)
}

func (m *Memory) Decide(_ context.Context, id, approver string, groups []string, approve bool, now time.Time) (*Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	apply(r, approver, groups, approve, now)
	cp := *r
	return &cp, nil
}

func apply(r *Request, approver string, groups []string, approve bool, now time.Time) {
	r.Approver = approver
	r.ApproverGroups = groups
	r.DecidedAt = now
	if approve {
		r.Status = StatusApproved
	} else {
		r.Status = StatusDenied
	}
}

// --- Valkey ------------------------------------------------------------------------------------

type Valkey struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewValkey(addr string, ttl time.Duration) *Valkey {
	return &Valkey{rdb: redis.NewClient(&redis.Options{Addr: addr}), ttl: ttl}
}

func (v *Valkey) Create(ctx context.Context, r *Request) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	pipe := v.rdb.TxPipeline()
	pipe.Set(ctx, "approval:"+r.ID, b, v.ttl)
	pipe.Set(ctx, "approval:hash:"+r.RequestHash, r.ID, v.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (v *Valkey) Get(ctx context.Context, id string) (*Request, error) {
	b, err := v.rdb.Get(ctx, "approval:"+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var r Request
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (v *Valkey) FindByHash(ctx context.Context, hash string) (*Request, error) {
	id, err := v.rdb.Get(ctx, "approval:hash:"+hash).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return v.Get(ctx, id)
}

func (v *Valkey) Decide(ctx context.Context, id, approver string, groups []string, approve bool, now time.Time) (*Request, error) {
	r, err := v.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	apply(r, approver, groups, approve, now)
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	// Keep the record until the original TTL so the retry window is the one the requester saw.
	if err := v.rdb.Set(ctx, "approval:"+id, b, redis.KeepTTL).Err(); err != nil {
		return nil, err
	}
	return r, nil
}
