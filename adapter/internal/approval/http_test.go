package approval

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/audit"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/config"
)

// The policy hook must start whether or not the identity provider is reachable; only the
// approval page depends on it, and it says so when asked.
func TestApprovalPageDoesNotNeedTheIdentityProviderAtStartup(t *testing.T) {
	cfg := config.Config{OIDCIssuer: "http://127.0.0.1:9/realms/eacp", OIDCClientID: "eacp-adapter", PublicURL: "http://gw.test"}
	w, err := NewWeb(cfg, NewMemory(), audit.NewStdout(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("construction must not contact the identity provider: %v", err)
	}
	mux := http.NewServeMux()
	w.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/approvals/abc", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unreachable identity provider should give 503 on the page, got %d", rec.Code)
	}
}
