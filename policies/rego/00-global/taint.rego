# Session taint. Each conversation carries three boolean labels that the adapter stores in
# Valkey and passes back on every call. Response phases propose updates; request phases are
# judged against the labels. This is what catches the "read a record, then fetch a URL" chain
# that no per-call check can see.
package eacp.taint

server := data.eacp.tools[input.tool.server]

tool := server.tools[input.tool.name]

thresholds := data.eacp.config.thresholds

touched(label) if input.taint[label] == true

deny contains {"code": "exfil_after_restricted", "msg": "an exfiltration-capable tool may not be used after restricted data was read in this conversation"} if {
	input.phase == "request"
	tool.capability == "exfil-capable"
	touched("touched_restricted")
}

update_set contains "touched_restricted" if {
	input.phase == "response"
	tool.output_classification == "restricted"
}

update_set contains "touched_pii" if {
	input.phase == "response"
	count(input.findings.pii) > 0
}

update_set contains "touched_untrusted_content" if {
	input.phase == "response"
	server.output_trust == "untrusted"
}

update_set contains "touched_untrusted_content" if {
	input.phase == "response"
	input.findings.injection.score >= thresholds.injection_taint
}

updates := {k: true | some k in update_set}
