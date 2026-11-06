// Package taint keeps the per-conversation labels, the last observed listing hash per server,
// the quarantine flags, and the routing decision cache. One store interface, two backends:
// in-memory for tests and single-process demos, Valkey for the cluster.
package taint

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	TouchedPII       = "touched_pii"
	TouchedRestrict  = "touched_restricted"
	TouchedUntrusted = "touched_untrusted_content"
)

// Known returns every label name; missing labels are sent to the policy as false so the input
// is always complete.
func Known() []string { return []string{TouchedPII, TouchedRestrict, TouchedUntrusted} }

type Store interface {
	Labels(ctx context.Context, conversation string) (map[string]bool, error)
	Merge(ctx context.Context, conversation string, updates map[string]bool) error

	ObservedPin(ctx context.Context, server, kind string) (string, bool, error)
	SetObservedPin(ctx context.Context, server, kind, hash string) error

	Quarantine(ctx context.Context, server, reason string) error
	Quarantined(ctx context.Context, server string) (string, bool, error)
	Lift(ctx context.Context, server string) error

	SetRoute(ctx context.Context, conversation, contentHash, targetClass string) error
	Route(ctx context.Context, conversation, contentHash string) (string, bool, error)
}

// --- in-memory ---------------------------------------------------------------------------------

type Memory struct {
	mu     sync.Mutex
	labels map[string]map[string]bool
	pins   map[string]string
	quar   map[string]string
	routes map[string]string
}

func NewMemory() *Memory {
	return &Memory{labels: map[string]map[string]bool{}, pins: map[string]string{}, quar: map[string]string{}, routes: map[string]string{}}
}

func (m *Memory) Labels(_ context.Context, conv string) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return complete(m.labels[conv]), nil
}

func (m *Memory) Merge(_ context.Context, conv string, updates map[string]bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.labels[conv]
	if cur == nil {
		cur = map[string]bool{}
		m.labels[conv] = cur
	}
	for k, v := range updates {
		if v {
			cur[k] = true
		}
	}
	return nil
}

func (m *Memory) ObservedPin(_ context.Context, server, kind string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.pins[server+"/"+kind]
	return h, ok, nil
}

func (m *Memory) SetObservedPin(_ context.Context, server, kind, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pins[server+"/"+kind] = hash
	return nil
}

func (m *Memory) Quarantine(_ context.Context, server, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.quar[server] = reason
	return nil
}

func (m *Memory) Quarantined(_ context.Context, server string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.quar[server]
	return r, ok, nil
}

func (m *Memory) Lift(_ context.Context, server string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.quar, server)
	return nil
}

func (m *Memory) SetRoute(_ context.Context, conv, hash, class string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[conv+"/"+hash] = class
	return nil
}

func (m *Memory) Route(_ context.Context, conv, hash string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.routes[conv+"/"+hash]
	return c, ok, nil
}

// --- Valkey ------------------------------------------------------------------------------------

type Valkey struct {
	rdb      *redis.Client
	taintTTL time.Duration
	routeTTL time.Duration
}

func NewValkey(addr string, taintTTL, routeTTL time.Duration) *Valkey {
	return &Valkey{rdb: redis.NewClient(&redis.Options{Addr: addr}), taintTTL: taintTTL, routeTTL: routeTTL}
}

func (v *Valkey) Ping(ctx context.Context) error { return v.rdb.Ping(ctx).Err() }

func (v *Valkey) Labels(ctx context.Context, conv string) (map[string]bool, error) {
	raw, err := v.rdb.HGetAll(ctx, "taint:"+conv).Result()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for k, s := range raw {
		b, _ := strconv.ParseBool(s)
		out[k] = b
	}
	return complete(out), nil
}

func (v *Valkey) Merge(ctx context.Context, conv string, updates map[string]bool) error {
	fields := map[string]any{}
	for k, val := range updates {
		if val {
			fields[k] = "true"
		}
	}
	if len(fields) == 0 {
		return nil
	}
	key := "taint:" + conv
	pipe := v.rdb.TxPipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, v.taintTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (v *Valkey) ObservedPin(ctx context.Context, server, kind string) (string, bool, error) {
	h, err := v.rdb.Get(ctx, "pin:"+server+":"+kind).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	return h, err == nil, err
}

func (v *Valkey) SetObservedPin(ctx context.Context, server, kind, hash string) error {
	return v.rdb.Set(ctx, "pin:"+server+":"+kind, hash, 0).Err()
}

func (v *Valkey) Quarantine(ctx context.Context, server, reason string) error {
	return v.rdb.Set(ctx, "quarantine:"+server, reason, 0).Err()
}

func (v *Valkey) Quarantined(ctx context.Context, server string) (string, bool, error) {
	r, err := v.rdb.Get(ctx, "quarantine:"+server).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	return r, err == nil, err
}

func (v *Valkey) Lift(ctx context.Context, server string) error {
	return v.rdb.Del(ctx, "quarantine:"+server).Err()
}

func (v *Valkey) SetRoute(ctx context.Context, conv, hash, class string) error {
	return v.rdb.Set(ctx, "route:"+conv+":"+hash, class, v.routeTTL).Err()
}

func (v *Valkey) Route(ctx context.Context, conv, hash string) (string, bool, error) {
	c, err := v.rdb.Get(ctx, "route:"+conv+":"+hash).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	return c, err == nil, err
}

func complete(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, k := range Known() {
		out[k] = in[k]
	}
	return out
}
