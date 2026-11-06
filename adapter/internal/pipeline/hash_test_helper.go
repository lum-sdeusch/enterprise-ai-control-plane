package pipeline

import "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/pinning"

// hashItems exists so tests in this package can compute pinned hashes without importing
// pinning themselves; it is the same function the Listing path uses.
func hashItems(items []map[string]any) (string, error) { return pinning.Hash(items) }
