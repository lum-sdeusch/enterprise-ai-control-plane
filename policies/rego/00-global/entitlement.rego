# Who may see and call which tool. Four independent gates, all of which must pass:
# group entitlement on the server, the agent manifest's allowed tools, the caller's clearance
# against the tool's output classification, and the agent's classification ceiling.
package eacp.entitlement

import data.eacp.lib

tool_phases := {"request", "response"}

server := data.eacp.tools[input.tool.server]

tool := server.tools[input.tool.name]

agent := data.eacp.agents[input.agent]

# Clearance comes from the token; a missing claim falls back to the team default, then to public.
subject_clearance := c if {
	c := input.subject.clearance
} else := c if {
	c := data.eacp.teams[input.subject.team].default_clearance
} else := "public"

deny contains {"code": "unknown_server", "msg": sprintf("server %q is not onboarded", [input.tool.server])} if {
	input.phase in tool_phases
	not data.eacp.tools[input.tool.server]
}

# Allow-list semantics: a tool the manifest does not declare is denied even if the server
# advertises it. This is also what makes a rug-pull that adds a tool fail closed.
deny contains {"code": "unknown_tool", "msg": sprintf("tool %q is not declared by server %q", [input.tool.name, input.tool.server])} if {
	input.phase in tool_phases
	data.eacp.tools[input.tool.server]
	not data.eacp.tools[input.tool.server].tools[input.tool.name]
}

deny contains {"code": "unknown_agent", "msg": sprintf("agent %q has no manifest", [input.agent])} if {
	input.phase in tool_phases
	not data.eacp.agents[input.agent]
}

deny contains {"code": "group_not_entitled", "msg": sprintf("caller is in none of %v", [server.required_groups])} if {
	input.phase in tool_phases
	count(server.required_groups) > 0
	not lib.intersects(input.subject.groups, server.required_groups)
}

deny contains {"code": "agent_not_entitled", "msg": sprintf("agent %q may not use %s/%s", [input.agent, input.tool.server, input.tool.name])} if {
	input.phase in tool_phases
	agent
	not lib.tool_allowed(agent.allowed_tools, input.tool.server, input.tool.name)
}

deny contains {"code": "clearance_insufficient", "msg": sprintf("tool output is %s, caller clearance is %s", [tool.output_classification, subject_clearance])} if {
	input.phase in tool_phases
	lib.rank(tool.output_classification) > lib.rank(subject_clearance)
}

deny contains {"code": "agent_ceiling_exceeded", "msg": sprintf("tool output is %s, agent ceiling is %s", [tool.output_classification, agent.max_classification])} if {
	input.phase in tool_phases
	lib.rank(tool.output_classification) > lib.rank(agent.max_classification)
}

# The set the adapter uses to filter tools/list. Same four gates, evaluated over every onboarded
# tool instead of the one being called.
visible_tools contains sprintf("%s/%s", [s, t]) if {
	some s, srv in data.eacp.tools
	some t, td in srv.tools
	group_ok(srv)
	lib.tool_allowed(agent.allowed_tools, s, t)
	lib.rank(td.output_classification) <= lib.rank(subject_clearance)
	lib.rank(td.output_classification) <= lib.rank(agent.max_classification)
}

group_ok(srv) if count(srv.required_groups) == 0

group_ok(srv) if lib.intersects(input.subject.groups, srv.required_groups)
