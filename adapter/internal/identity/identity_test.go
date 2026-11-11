package identity

import "testing"

func TestAgentOf(t *testing.T) {
	clients := map[string]string{
		"agent-support-assistant": "support-assistant",
		"librechat":               "support-assistant",
		"agent-nightly-digest":    "nightly-digest",
	}
	cases := []struct {
		name string
		s    Subject
		want string
	}{
		{"actor claim names the agent", Subject{ClientID: "librechat", Act: &Actor{ClientID: "agent-support-assistant"}}, "support-assistant"},
		{"user-facing client without an actor claim", Subject{ClientID: "librechat"}, "support-assistant"},
		{"scheduled agent's own client", Subject{ClientID: "agent-nightly-digest"}, "nightly-digest"},
		// The token came through a client an agent acts through, but names another actor. The
		// client's agent must not stand in for it.
		{"unregistered actor does not fall back to the client", Subject{ClientID: "librechat", Act: &Actor{ClientID: "agent-nobody-registered"}}, ""},
		{"unknown client", Subject{ClientID: "something-else"}, ""},
	}
	for _, c := range cases {
		if got := AgentOf(c.s, clients); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
