package approval

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/audit"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/config"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/identity"
)

// Web serves the minimal approval page: the approver signs in through Keycloak, sees who asked
// for what, and approves or denies. Both outcomes are audited with the approver's identity.
// The spec-native flow (elicitation plus the Tasks extension) is a separate, flagged path
// (ADR 0031).
type Web struct {
	Store    Store
	Cfg      config.Config
	Audit    audit.Writer
	Log      *slog.Logger
	Now      func() time.Time
	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
	key      []byte
}

type session struct {
	Sub    string   `json:"sub"`
	Groups []string `json:"groups"`
	Exp    int64    `json:"exp"`
}

func NewWeb(cfg config.Config, store Store, aud audit.Writer, log *slog.Logger) (*Web, error) {
	w := &Web{Store: store, Cfg: cfg, Audit: aud, Log: log, Now: time.Now}
	if cfg.CookieSecret != "" {
		w.key = []byte(cfg.CookieSecret)
	} else {
		w.key = make([]byte, 32)
		if _, err := rand.Read(w.key); err != nil {
			return nil, err
		}
	}
	if cfg.OIDCIssuer == "" {
		log.Warn("approvals: EACP_OIDC_ISSUER not set; the approval page will refuse to serve")
	}
	return w, nil
}

// ensure connects to the identity provider on first use. Not at start-up: the policy hook must
// come up, and stay up, whether or not Keycloak is reachable; only the approval page needs it.
func (w *Web) ensure() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.oauth != nil {
		return nil
	}
	cfg := w.Cfg
	if cfg.OIDCIssuer == "" {
		return errors.New("approvals are not configured (EACP_OIDC_ISSUER)")
	}
	// The provider keeps this context for fetching signing keys later, so it must outlive any
	// one request. Discovery goes to the in-cluster Keycloak URL while the issuer stays the
	// public one.
	ctx := context.Background()
	discovery := cfg.OIDCIssuer
	if cfg.OIDCInternalURL != "" {
		u, err := url.Parse(cfg.OIDCIssuer)
		if err != nil {
			return err
		}
		discovery = strings.TrimRight(cfg.OIDCInternalURL, "/") + u.Path
		ctx = oidc.InsecureIssuerURLContext(ctx, cfg.OIDCIssuer)
	}
	provider, err := oidc.NewProvider(ctx, discovery)
	if err != nil {
		return fmt.Errorf("approvals: oidc discovery: %w", err)
	}
	endpoint := provider.Endpoint()
	// The browser must reach the public authorization endpoint.
	endpoint.AuthURL = strings.TrimRight(cfg.OIDCIssuer, "/") + "/protocol/openid-connect/auth"
	w.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
	w.oauth = &oauth2.Config{
		ClientID: cfg.OIDCClientID, ClientSecret: cfg.OIDCClientSecret, Endpoint: endpoint,
		RedirectURL: strings.TrimRight(cfg.PublicURL, "/") + "/approvals/callback",
		Scopes:      []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return nil
}

func (w *Web) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /approvals/callback", w.callback)
	mux.HandleFunc("GET /approvals/{id}", w.show)
	mux.HandleFunc("POST /approvals/{id}", w.decide)
}

func (w *Web) show(rw http.ResponseWriter, r *http.Request) {
	s, ok := w.session(r)
	if !ok {
		w.login(rw, r, r.URL.Path)
		return
	}
	req, err := w.Store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		http.Error(rw, "no such approval request", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(rw, "approval store unavailable", http.StatusServiceUnavailable)
		return
	}
	w.render(rw, req, s, "")
}

func (w *Web) decide(rw http.ResponseWriter, r *http.Request) {
	s, ok := w.session(r)
	if !ok {
		http.Error(rw, "sign in first", http.StatusUnauthorized)
		return
	}
	req, err := w.Store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(rw, "no such approval request", http.StatusNotFound)
		return
	}
	now := w.Now()
	if req.Status != StatusPending || req.Expired(now) {
		w.render(rw, req, s, "this request is no longer pending")
		return
	}
	if !contains(s.Groups, req.ApproverGroup) {
		w.render(rw, req, s, "you are not in the approver group "+req.ApproverGroup)
		return
	}
	if s.Sub == req.Subject && !req.AllowSelf {
		w.render(rw, req, s, "the requester may not approve their own call")
		return
	}
	approve := r.FormValue("decision") == "approve"
	updated, err := w.Store.Decide(r.Context(), req.ID, s.Sub, s.Groups, approve, now)
	if err != nil {
		http.Error(rw, "approval store unavailable", http.StatusServiceUnavailable)
		return
	}
	ev := audit.Event{
		TS: audit.Timestamp(now), EventID: NewID(), TraceID: req.TraceID, ConversationID: req.ConversationID,
		Phase: "approval", Subject: req.Subject, Team: req.Team, Groups: []string{}, Agent: req.Agent,
		ActorChain: identity.ActorChain(identity.Subject{Sub: req.Subject}), Server: req.Server, Tool: req.Tool,
		Decision: updated.Status, Reasons: []string{req.Reason}, Findings: "{}", Approver: s.Sub, AdapterVersion: config.Version,
	}
	if err := w.Audit.Write(r.Context(), ev); err != nil && w.Cfg.AuditRequired {
		http.Error(rw, "audit store unavailable; decision not recorded", http.StatusServiceUnavailable)
		return
	}
	w.render(rw, updated, s, "")
}

// --- OIDC login -----------------------------------------------------------------------------------

func (w *Web) login(rw http.ResponseWriter, r *http.Request, returnTo string) {
	if err := w.ensure(); err != nil {
		w.Log.Warn("approval page unavailable", "err", err)
		http.Error(rw, "the approval page cannot reach the identity provider; try again shortly", http.StatusServiceUnavailable)
		return
	}
	state := base64.RawURLEncoding.EncodeToString([]byte(returnTo))
	http.SetCookie(rw, &http.Cookie{Name: "eacp_state", Value: w.sign(state), Path: "/approvals", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(rw, r, w.oauth.AuthCodeURL(state), http.StatusFound)
}

func (w *Web) callback(rw http.ResponseWriter, r *http.Request) {
	if err := w.ensure(); err != nil {
		http.Error(rw, "the approval page cannot reach the identity provider; try again shortly", http.StatusServiceUnavailable)
		return
	}
	state := r.URL.Query().Get("state")
	c, err := r.Cookie("eacp_state")
	if err != nil || !w.verify(c.Value, state) {
		http.Error(rw, "invalid state", http.StatusBadRequest)
		return
	}
	tok, err := w.oauth.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(rw, "code exchange failed", http.StatusBadGateway)
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	idt, err := w.verifier.Verify(r.Context(), raw)
	if err != nil {
		http.Error(rw, "id token verification failed", http.StatusUnauthorized)
		return
	}
	var claims map[string]any
	if err := idt.Claims(&claims); err != nil {
		http.Error(rw, "claims", http.StatusUnauthorized)
		return
	}
	sub := identity.FromClaims(claims)
	s := session{Sub: sub.Sub, Groups: sub.Groups, Exp: w.Now().Add(15 * time.Minute).Unix()}
	b, _ := json.Marshal(s)
	http.SetCookie(rw, &http.Cookie{Name: "eacp_session", Value: w.sign(base64.RawURLEncoding.EncodeToString(b)), Path: "/approvals", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 900})
	returnTo, _ := base64.RawURLEncoding.DecodeString(state)
	target := string(returnTo)
	if !strings.HasPrefix(target, "/approvals/") {
		target = "/approvals/"
	}
	http.Redirect(rw, r, target, http.StatusFound)
}

func (w *Web) session(r *http.Request) (session, bool) {
	c, err := r.Cookie("eacp_session")
	if err != nil {
		return session{}, false
	}
	payload, ok := w.open(c.Value)
	if !ok {
		return session{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return session{}, false
	}
	var s session
	if err := json.Unmarshal(b, &s); err != nil || w.Now().Unix() > s.Exp {
		return session{}, false
	}
	return s, true
}

func (w *Web) sign(v string) string {
	mac := hmac.New(sha256.New, w.key)
	mac.Write([]byte(v))
	return v + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (w *Web) open(signed string) (string, bool) {
	i := strings.LastIndex(signed, ".")
	if i < 0 {
		return "", false
	}
	v, sig := signed[:i], signed[i+1:]
	mac := hmac.New(sha256.New, w.key)
	mac.Write([]byte(v))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}
	return v, true
}

func (w *Web) verify(signed, want string) bool {
	v, ok := w.open(signed)
	return ok && v == want
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// --- page ------------------------------------------------------------------------------------------

var page = template.Must(template.New("approval").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>eacp approval</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
body{font-family:system-ui,sans-serif;max-width:640px;margin:2rem auto;padding:0 1rem;color:#111}
dl{display:grid;grid-template-columns:max-content 1fr;gap:.4rem 1rem}dt{font-weight:600}
.status{padding:.5rem .75rem;border-radius:.4rem;background:#eee;display:inline-block}
.pending{background:#fff3cd}.approved{background:#d1e7dd}.denied{background:#f8d7da}
button{font:inherit;padding:.5rem 1rem;margin-right:.5rem}.err{color:#b02a37}
</style></head><body>
<h1>Approval request</h1>
<p>Signed in as <strong>{{.Session.Sub}}</strong></p>
<p class="status {{.Req.Status}}">Status: {{.Req.Status}}</p>
<dl>
<dt>Requested by</dt><dd>{{.Req.Subject}} (team {{.Req.Team}}) through agent {{.Req.Agent}}</dd>
<dt>Tool</dt><dd>{{.Req.Server}}/{{.Req.Tool}}</dd>
<dt>Arguments</dt><dd>{{.Req.Summary}}</dd>
<dt>Why approval</dt><dd>{{.Req.Reason}}</dd>
<dt>Approver group</dt><dd>{{.Req.ApproverGroup}}</dd>
<dt>Requested at</dt><dd>{{.Req.CreatedAt.Format "2006-01-02 15:04:05 MST"}}</dd>
<dt>Expires at</dt><dd>{{.Req.ExpiresAt.Format "2006-01-02 15:04:05 MST"}}</dd>
<dt>Conversation</dt><dd>{{.Req.ConversationID}}</dd>
<dt>Trace</dt><dd>{{.Req.TraceID}}</dd>
{{if .Req.Approver}}<dt>Decided by</dt><dd>{{.Req.Approver}} at {{.Req.DecidedAt.Format "2006-01-02 15:04:05 MST"}}</dd>{{end}}
</dl>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
{{if eq .Req.Status "pending"}}
<form method="post">
<button name="decision" value="approve">Approve</button>
<button name="decision" value="deny">Deny</button>
</form>
<p>Approving lets the requester's agent retry exactly this call, with these arguments, before the expiry above.</p>
{{end}}
</body></html>`))

func (w *Web) render(rw http.ResponseWriter, req *Request, s session, errMsg string) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Cache-Control", "no-store")
	_ = page.Execute(rw, map[string]any{"Req": req, "Session": s, "Error": errMsg})
}
