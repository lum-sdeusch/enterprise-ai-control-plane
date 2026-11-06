// Package pinning hashes MCP listings so a server that changes what it advertises after
// onboarding is detected. The hash is over canonical JSON (sorted keys, compact encoding) of the
// items sorted by name, so field order and whitespace never matter and a changed description,
// schema or annotation always does.
package pinning

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Hash returns "sha256:<hex>" of the canonical form of items. An empty listing hashes too, so a
// server that suddenly advertises nothing is also a change.
func Hash(items []map[string]any) (string, error) {
	sorted := make([]map[string]any, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		return name(sorted[i]) < name(sorted[j])
	})
	// encoding/json sorts map keys at every level and emits compact output, which is the
	// canonical form we need; round-tripping through `any` normalises numbers too. HTML escaping
	// is off so the bytes match Python's json.dumps(sort_keys=True, separators=(",", ":"),
	// ensure_ascii=False) in scripts/pin-tools.py; both sides share the test vector in
	// pinning_test.go and scripts/pin-tools.py.
	b, err := json.Marshal(sorted)
	if err != nil {
		return "", err
	}
	var round any
	if err := json.Unmarshal(b, &round); err != nil {
		return "", err
	}
	canon, err := canonical(round)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func name(m map[string]any) string {
	if s, ok := m["name"].(string); ok {
		return s
	}
	if s, ok := m["uri"].(string); ok {
		return s
	}
	return ""
}
