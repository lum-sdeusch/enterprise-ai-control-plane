# Agent layer for nightly-triage. A scheduled agent has no human watching, so it never gets an
# exfiltration-capable or destructive tool, whatever its manifest or group entitlement says.
package eacp.agentpolicy.nightly_triage

blocked_capabilities := {"exfil-capable", "destructive"}

deny contains {"code": "scheduled_agent_capability_blocked", "msg": sprintf("scheduled agents may not use %s tools", [cap])} if {
	input.phase == "request"
	input.agent == "nightly-triage"
	cap := data.eacp.tools[input.tool.server].tools[input.tool.name].capability
	cap in blocked_capabilities
}
