# Classification-aware model routing. The label is the maximum over every source: the
# guardrail sensitivity finding, PII findings, the conversation's taint, and an explicit label
# claim in the token. The routing table in config maps a label to the model classes allowed to
# see it; the team and agent manifests narrow that further. The first class that survives wins,
# and the fallback is always in-house.
package eacp.routing

import data.eacp.lib

cfg := data.eacp.config

llm_phases := {"llm_route", "llm_request", "llm_response"}

labels contains l if {
	l := input.findings.sensitivity.label
}

labels contains "confidential" if {
	count(input.findings.pii) > 0
}

labels contains "restricted" if {
	input.taint.touched_restricted == true
}

labels contains "confidential" if {
	input.taint.touched_pii == true
}

labels contains l if {
	l := input.subject.data_label
	l != null
}

label := lib.max_label(labels)

team_classes := c if {
	c := data.eacp.teams[input.subject.team].allowed_model_classes
} else := ["inhouse"]

agent_classes := c if {
	c := data.eacp.agents[input.agent].allowed_model_classes
} else := ["inhouse"]

allowed_classes := [c |
	some c in cfg.routing[label]
	c in team_classes
	c in agent_classes
]

default target_class := "inhouse"

target_class := allowed_classes[0] if {
	count(allowed_classes) > 0
}

target := cfg.models[target_class]

deny contains {"code": "no_model_class_allowed", "msg": sprintf("no model class may handle %s data for this team and agent", [label])} if {
	input.phase in llm_phases
	count(allowed_classes) == 0
}

# A prompt already bound for a concrete model class the label forbids is stopped at the prompt
# guard, in case the client bypassed the virtual model and named a model directly.
deny contains {"code": "model_class_not_allowed_for_label", "msg": sprintf("%s data may not go to the %s model class", [label, input.llm.target_class])} if {
	input.phase == "llm_request"
	not input.llm.target_class in cfg.routing[label]
}
