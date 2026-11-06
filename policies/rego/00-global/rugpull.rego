# Rug-pull protection. The manifest pins the SHA-256 of the server's tools/list at onboarding;
# the adapter hashes every live listing and reports the current value. A mismatch quarantines
# the server: every call is denied until a new manifest with the new hash is merged.
package eacp.rugpull

tool_phases := {"request", "response"}

pinned := data.eacp.tools[input.tool.server].pins.tools

deny contains {"code": "server_quarantined", "msg": sprintf("live tools/list hash %s does not match pinned %s", [input.pinning.tools, pinned])} if {
	input.phase in tool_phases
	input.pinning.tools != pinned
}

# No observed listing yet means the adapter cannot vouch for the server. Fail closed; the
# adapter refreshes the listing on start-up and on every tools/list it proxies.
deny contains {"code": "pin_unverified", "msg": "no tools/list has been observed for this server"} if {
	input.phase in tool_phases
	data.eacp.tools[input.tool.server]
	not input.pinning.tools
}
