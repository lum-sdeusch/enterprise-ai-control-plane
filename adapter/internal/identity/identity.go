// Package identity turns what the gateway tells us about the caller into the subject the policy
// reasons about. The gateway has already verified the token's signature, issuer and audience;
// this package only reads claims, it never trusts them from an unverified source.
package identity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type Actor struct {
	ClientID string `json:"client_id,omitempty"`
	Sub      string `json:"sub,omitempty"`
}

type Subject struct {
	Sub       string   `json:"sub"`
	Groups    []string `json:"groups"`
	Team      string   `json:"team"`
	Clearance string   `json:"clearance,omitempty"`
	DataLabel string   `json:"data_label,omitempty"`
	Act       *Actor   `json:"act,omitempty"`
	// ClientID is the token's authorized party (azp); not sent to policy, used to resolve the
	// agent for tokens without an actor chain (scheduled agents, client credentials).
	ClientID string `json:"-"`
}

// FromClaims builds a Subject from decoded JWT claims.
func FromClaims(claims map[string]any) Subject {
	s := Subject{
		Sub:       str(claims["sub"]),
		Groups:    strs(claims["groups"]),
		Team:      str(claims["team"]),
		Clearance: str(claims["clearance"]),
		DataLabel: str(claims["data_label"]),
		ClientID:  str(claims["azp"]),
	}
	if act, ok := claims["act"].(map[string]any); ok {
		s.Act = &Actor{ClientID: str(act["client_id"]), Sub: str(act["sub"])}
		if s.Act.ClientID == "" {
			// Keycloak delegation puts the actor's client id in azp of the nested claim set.
			s.Act.ClientID = str(act["azp"])
		}
	}
	if s.Groups == nil {
		s.Groups = []string{}
	}
	return s
}

// ClaimsFromBearer decodes the payload of a JWT from an Authorization header WITHOUT verifying
// it. It is only valid to call this for tokens the gateway has already verified (the hook runs
// behind jwtAuthentication in Strict mode). Never use it on anything a client sent directly.
func ClaimsFromBearer(authorization string) (map[string]any, error) {
	const prefix = "bearer "
	if len(authorization) < len(prefix) || !strings.EqualFold(authorization[:len(prefix)], prefix) {
		return nil, errors.New("not a bearer token")
	}
	parts := strings.Split(strings.TrimSpace(authorization[len(prefix):]), ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// FromHeaders builds a Subject from the x-eacp-* headers the gateway's CEL config attaches to
// LLM webhook and callout requests (platform/charts/eacp-gateway).
func FromHeaders(get func(string) string) Subject {
	s := Subject{
		Sub:       get("x-eacp-sub"),
		Groups:    splitCSV(get("x-eacp-groups")),
		Team:      get("x-eacp-team"),
		Clearance: get("x-eacp-clearance"),
		DataLabel: get("x-eacp-data-label"),
	}
	if a := get("x-eacp-agent"); a != "" {
		s.Act = &Actor{ClientID: a}
	}
	return s
}

// AgentOf resolves the agent manifest name for a subject: the actor's client id first (an
// interactive agent acting for a user), then the token's own client id (a scheduled agent).
func AgentOf(s Subject, agentClients map[string]string) string {
	if s.Act != nil && s.Act.ClientID != "" {
		if a, ok := agentClients[s.Act.ClientID]; ok {
			return a
		}
	}
	if a, ok := agentClients[s.ClientID]; ok {
		return a
	}
	return ""
}

// ActorChain renders "user -> agent" for audit records.
func ActorChain(s Subject) string {
	if s.Act == nil || s.Act.ClientID == "" {
		return s.Sub
	}
	return s.Sub + " -> " + s.Act.ClientID
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strs(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(t)
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
