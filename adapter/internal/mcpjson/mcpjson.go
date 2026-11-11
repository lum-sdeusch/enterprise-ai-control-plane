// Package mcpjson parses and re-serialises the JSON-RPC params and results the hook sees,
// preserving fields it does not understand so a pass-through never changes bytes it did not
// mean to change.
package mcpjson

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ToolCallParams is the `params` of tools/call.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Meta      map[string]any `json:"_meta,omitempty"`
}

func ParseToolCall(raw []byte) (ToolCallParams, error) {
	var p ToolCallParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("tools/call params: %w", err)
	}
	if p.Name == "" {
		return p, fmt.Errorf("tools/call params: missing name")
	}
	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}
	return p, nil
}

func (p ToolCallParams) Marshal() ([]byte, error) { return json.Marshal(p) }

// ToolResult is the `result` of tools/call. Content blocks stay generic maps so image, audio
// and resource blocks survive untouched; only text blocks are inspected and rewritten.
type ToolResult struct {
	Content []map[string]any
	IsError bool
	rest    map[string]json.RawMessage
}

func ParseToolResult(raw []byte) (ToolResult, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return ToolResult{}, fmt.Errorf("tools/call result: %w", err)
	}
	r := ToolResult{rest: map[string]json.RawMessage{}}
	for k, v := range all {
		switch k {
		case "content":
			if err := json.Unmarshal(v, &r.Content); err != nil {
				return ToolResult{}, fmt.Errorf("tools/call result content: %w", err)
			}
		case "isError":
			_ = json.Unmarshal(v, &r.IsError)
		default:
			r.rest[k] = v
		}
	}
	return r, nil
}

func (r ToolResult) Marshal() ([]byte, error) {
	out := map[string]any{}
	for k, v := range r.rest {
		out[k] = v
	}
	out["content"] = r.Content
	if r.IsError {
		out["isError"] = true
	}
	return json.Marshal(out)
}

// Texts returns every text block keyed by its path, e.g. "content.0".
func (r ToolResult) Texts() map[string]string {
	out := map[string]string{}
	for i, block := range r.Content {
		if block["type"] == "text" {
			if s, ok := block["text"].(string); ok {
				out["content."+strconv.Itoa(i)] = s
			}
		}
	}
	return out
}

// SetText replaces the text of the block at "content.<i>".
func (r *ToolResult) SetText(path, text string) {
	i, err := strconv.Atoi(strings.TrimPrefix(path, "content."))
	if err != nil || i < 0 || i >= len(r.Content) {
		return
	}
	r.Content[i]["text"] = text
}

// ParseList splits a */list result into its items and the remaining fields (nextCursor, _meta).
func ParseList(raw []byte, key string) ([]map[string]any, map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, nil, fmt.Errorf("%s list result: %w", key, err)
	}
	items := []map[string]any{}
	rest := map[string]json.RawMessage{}
	for k, v := range all {
		if k == key {
			if err := json.Unmarshal(v, &items); err != nil {
				return nil, nil, fmt.Errorf("%s list items: %w", key, err)
			}
			continue
		}
		rest[k] = v
	}
	return items, rest, nil
}

func MarshalList(key string, items []map[string]any, rest map[string]json.RawMessage) ([]byte, error) {
	out := map[string]any{}
	for k, v := range rest {
		out[k] = v
	}
	if items == nil {
		items = []map[string]any{}
	}
	out[key] = items
	return json.Marshal(out)
}

// StringLeaves flattens the string values of a JSON object into path -> value, with paths like
// "arguments.sql" or "arguments.items.0.name". Only strings are analysed; numbers and booleans
// cannot carry PII or instructions.
func StringLeaves(prefix string, v any) map[string]string {
	out := map[string]string{}
	walkLeaves(prefix, v, out)
	return out
}

func walkLeaves(path string, v any, out map[string]string) {
	switch t := v.(type) {
	case string:
		out[path] = t
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkLeaves(path+"."+k, t[k], out)
		}
	case []any:
		for i, x := range t {
			walkLeaves(path+"."+strconv.Itoa(i), x, out)
		}
	}
}

// SetLeaf writes a string at a path produced by StringLeaves. Unknown paths are ignored.
func SetLeaf(root map[string]any, prefix, path, value string) {
	if !strings.HasPrefix(path, prefix+".") {
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix+"."), ".")
	var cur any = root
	for i, p := range parts {
		last := i == len(parts)-1
		switch c := cur.(type) {
		case map[string]any:
			if last {
				c[p] = value
				return
			}
			cur = c[p]
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= len(c) {
				return
			}
			if last {
				c[idx] = value
				return
			}
			cur = c[idx]
		default:
			return
		}
	}
}

// SplitName resolves the server and bare tool name for a tools/call. The gateway names the
// backing server in service_names and has already removed the aggregate's "<server>_" prefix
// from the name by the time the hook sees it (agentgateway 1.7); the prefix is still stripped
// here if present, so both behaviours are handled. Without a service name there is no server.
func SplitName(name string, serviceNames []string) (server, tool string) {
	if len(serviceNames) == 0 {
		return "", name
	}
	server = serviceNames[0]
	return server, strings.TrimPrefix(name, server+"_")
}

// BareName strips the aggregate's prefix from a listed name.
func BareName(name, server string) string {
	return strings.TrimPrefix(name, server+"_")
}

// ItemKey is the identifying string of a listing item: its name, or its uri for a resource
// without one.
func ItemKey(item map[string]any) string {
	if s, ok := item["name"].(string); ok && s != "" {
		return s
	}
	s, _ := item["uri"].(string)
	return s
}

// PrefixOwner returns the server whose "<server>_" prefix key carries. Server names are DNS
// labels, which cannot contain an underscore, so at most one server matches.
func PrefixOwner(key string, servers []string) (string, bool) {
	for _, s := range servers {
		if strings.HasPrefix(key, s+"_") {
			return s, true
		}
	}
	return "", false
}

// Unprefixed returns copies of the items with the aggregate's "<server>_" prefix removed from
// their names, which is the form the server itself advertises and the form that is pinned.
func Unprefixed(items []map[string]any, server string) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, it := range items {
		cp := make(map[string]any, len(it))
		for k, v := range it {
			cp[k] = v
		}
		if name, ok := cp["name"].(string); ok {
			cp["name"] = BareName(name, server)
		}
		out[i] = cp
	}
	return out
}
