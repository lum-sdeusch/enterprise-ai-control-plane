package pinning

import "testing"

// The same items and expected hash live in scripts/pin-tools.py (VECTOR_ITEMS / VECTOR_HASH).
// The pin written at onboarding by the Python script must equal the hash the Go adapter computes
// at runtime, or every server would be quarantined on its first listing.
func TestCrossLanguageVector(t *testing.T) {
	items := []map[string]any{
		{"name": "fetch", "description": "Fetch a URL & return <markdown>", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string"}}, "required": []any{"url"}}},
		{"name": "ping", "description": "Ping ü"},
	}
	got, err := Hash(items)
	if err != nil {
		t.Fatal(err)
	}
	const want = "sha256:313593aef9970794928f9d855ce03b662b615ea1d3d342dddc2f0b30b6d5ff3f"
	if got != want {
		t.Fatalf("Go and Python canonical forms diverged:\n got %s\nwant %s", got, want)
	}
}
