# The single decision document the adapter asks for: data.eacp.decision.
#
# Global modules each contribute a `deny` set. Team, tool and agent layers contribute their own
# `deny` sets and are looked up by the team, server and agent in the input, so adding a layer
# never requires touching this file. A layer can only add denials, never remove one.
package eacp

import data.eacp.approval
import data.eacp.entitlement
import data.eacp.guardrails
import data.eacp.routing
import data.eacp.rugpull
import data.eacp.taint

default policy_revision := "unbundled"

# OPA exposes every loaded bundle's manifest under data.system.bundles; the audit record carries
# the revision so a decision can be reproduced against the exact policy that made it.
policy_revision := concat(",", sort([rev | some name, _ in data.system.bundles; rev := data.system.bundles[name].manifest.revision])) if {
	count(data.system.bundles) > 0
}

core_denies := (((entitlement.deny | rugpull.deny) | guardrails.deny) | taint.deny) | routing.deny

team_denies := d if {
	d := data.eacp.teampolicy[input.subject.team].deny
} else := set()

tool_denies := d if {
	d := data.eacp.toolpolicy[replace(input.tool.server, "-", "_")].deny
} else := set()

agent_denies := d if {
	d := data.eacp.agentpolicy[replace(input.agent, "-", "_")].deny
} else := set()

denies := ((core_denies | team_denies) | tool_denies) | agent_denies

default allow := false

allow if {
	count(denies) == 0
	not approval.pending
}

require_approval := approval.spec if {
	approval.pending
} else := null

audit_tags contains sprintf("phase:%s", [input.phase])

audit_tags contains sprintf("tool:%s/%s", [input.tool.server, input.tool.name])

audit_tags contains sprintf("agent:%s", [input.agent])

audit_tags contains sprintf("team:%s", [input.subject.team])

audit_tags contains sprintf("label:%s", [routing.label])

decision := {
	"allow": allow,
	"reasons": [r | some r in denies],
	"redactions": [r | some r in guardrails.redact],
	"require_approval": require_approval,
	"route_label": routing.label,
	"route_target": routing.target,
	"taint_updates": taint.updates,
	"audit_tags": [t | some t in audit_tags],
	"policy_revision": policy_revision,
}
