# Turns probabilistic findings from the guardrail service into deterministic outcomes.
# Secrets are never allowed through in any direction. Injection above the block threshold
# stops a tool result or a prompt. PII is redacted, not blocked, on the paths where it would
# leave the organisation's control.
package eacp.guardrails

tool := data.eacp.tools[input.tool.server].tools[input.tool.name]

thresholds := data.eacp.config.thresholds

injection_score := s if {
	s := input.findings.injection.score
} else := 0

secrets := s if {
	s := input.findings.secrets
} else := []

pii := p if {
	p := input.findings.pii
} else := []

deny contains {"code": "secret_in_arguments", "msg": sprintf("%d secret(s) detected in tool arguments", [count(secrets)])} if {
	input.phase == "request"
	count(secrets) > 0
}

deny contains {"code": "secret_in_prompt", "msg": sprintf("%d secret(s) detected in prompt", [count(secrets)])} if {
	input.phase == "llm_request"
	count(secrets) > 0
}

deny contains {"code": "injection_in_tool_result", "msg": sprintf("injection score %.2f is at or above block threshold %.2f", [injection_score, thresholds.injection_block])} if {
	input.phase == "response"
	injection_score >= thresholds.injection_block
}

deny contains {"code": "injection_in_prompt", "msg": sprintf("injection score %.2f is at or above block threshold %.2f", [injection_score, thresholds.injection_block])} if {
	input.phase == "llm_request"
	injection_score >= thresholds.injection_block
}

# PII in the arguments of an exfiltration-capable tool is masked before the call proceeds.
redact contains f if {
	input.phase == "request"
	tool.capability == "exfil-capable"
	some f in pii
}

# PII bound for an external model is masked; the in-house model may see it.
redact contains f if {
	input.phase == "llm_request"
	input.llm.target_class == "external"
	some f in pii
}

# Secrets that appear in a tool result or a completion are masked and the event is audited.
redact contains f if {
	input.phase in {"response", "llm_response"}
	some f in secrets
}
