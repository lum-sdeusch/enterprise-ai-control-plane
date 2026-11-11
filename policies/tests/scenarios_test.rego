# Unit tests for data.eacp.decision. One fixture per demo scenario under tests/fixtures,
# loaded at data.tests.fixtures.<name>. Variants are built with object.union so each test
# states only what differs from the fixture.
package eacp.tests.scenarios

import data.eacp
import data.tests.fixtures as fx

codes(d) := {r.code | some r in d.reasons}

# Fixtures say "MANIFEST_PIN" where the live listing is meant to match what the manifest pins.
# It is resolved after a test's overrides, against the server the final input names, so tests do
# not change when a tool is re-pinned and never pass or fail on a stale hash.
pinned(inp) := object.union(inp, {"pinning": {"tools": data.eacp.tools[inp.tool.server].pins.tools}}) if {
	inp.pinning.tools == "MANIFEST_PIN"
} else := inp

# --- baseline ---------------------------------------------------------------------------------

test_baseline_allows_entitled_call if {
	d := eacp.decision with input as pinned(fx.t0_baseline)
	d.allow
	count(d.reasons) == 0
	count(d.redactions) == 0
	d.require_approval == null
	d.policy_revision == "unbundled"
	"phase:request" in d.audit_tags
	"tool:tickets/list_tickets" in d.audit_tags
}

# --- scenario 1: rug-pull quarantine ----------------------------------------------------------

test_t1_rugpull_quarantines_server if {
	d := eacp.decision with input as pinned(fx.t1_rugpull)
	not d.allow
	"server_quarantined" in codes(d)
}

test_t1_rugpull_control_matching_pin_allows if {
	d := eacp.decision with input as pinned(object.union(fx.t1_rugpull, {"pinning": {"tools": "MANIFEST_PIN"}}))
	d.allow
}

test_t1_missing_pin_observation_fails_closed if {
	d := eacp.decision with input as pinned(object.remove(fx.t0_baseline, ["pinning"]))
	not d.allow
	"pin_unverified" in codes(d)
}

# --- scenario 2: injection in a tool result ---------------------------------------------------

test_t2_injection_in_result_is_blocked_and_taints if {
	d := eacp.decision with input as pinned(fx.t2_injection_result)
	not d.allow
	"injection_in_tool_result" in codes(d)
	d.taint_updates.touched_untrusted_content == true
}

test_t2_low_score_untrusted_source_passes_but_taints if {
	d := eacp.decision with input as pinned(object.union(fx.t2_injection_result, {"findings": {"injection": {"score": 0.1, "label": "safe", "categories": []}}}))
	d.allow
	d.taint_updates.touched_untrusted_content == true
}

test_t2_mid_score_taints_without_blocking if {
	d := eacp.decision with input as pinned(object.union(fx.t2_injection_result, {"findings": {"injection": {"score": 0.6, "label": "controversial", "categories": []}}}))
	d.allow
	d.taint_updates.touched_untrusted_content == true
}

# --- scenario 3: session-taint exfiltration block --------------------------------------------

test_t3_exfil_after_restricted_is_denied if {
	d := eacp.decision with input as pinned(fx.t3_taint_exfil)
	not d.allow
	"exfil_after_restricted" in codes(d)
}

test_t3_control_without_restricted_taint_allows_with_pii_redacted if {
	d := eacp.decision with input as pinned(object.union(fx.t3_taint_exfil, {"taint": {"touched_restricted": false}}))
	d.allow
	count(d.redactions) == 1
	d.redactions[0].type == "EMAIL_ADDRESS"
}

test_t3_reading_restricted_record_sets_taint if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {
		"phase": "response",
		"tool": {"server": "customer-db", "name": "execute_sql"},
	}))
	d.allow
	d.taint_updates.touched_restricted == true
}

# --- scenario 4: classification-aware routing -------------------------------------------------

test_t4_restricted_taint_routes_in_house if {
	d := eacp.decision with input as pinned(fx.t4_routing_restricted)
	d.allow
	d.route_label == "restricted"
	d.route_target == "eacp-inhouse"
}

test_t4_public_prompt_routes_external if {
	d := eacp.decision with input as pinned(fx.t4_routing_public)
	d.allow
	d.route_label == "public"
	d.route_target == "eacp-external"
}

test_t4_pii_finding_makes_prompt_confidential if {
	d := eacp.decision with input as pinned(object.union(fx.t4_routing_public, {"findings": {"pii": [{"type": "PHONE_NUMBER", "start": 0, "end": 12, "score": 0.9}]}}))
	d.route_label == "confidential"
	d.route_target == "eacp-inhouse"
}

test_t4_explicit_token_label_is_honoured if {
	d := eacp.decision with input as pinned(object.union(fx.t4_routing_public, {"subject": {"data_label": "restricted"}}))
	d.route_label == "restricted"
	d.route_target == "eacp-inhouse"
}

test_t4_scheduled_agent_never_leaves_in_house if {
	d := eacp.decision with input as pinned(fx.t4_routing_scheduled_agent)
	d.allow
	d.route_label == "public"
	d.route_target == "eacp-inhouse"
}

# --- scenario 5: one-human approval -----------------------------------------------------------

test_t5_high_risk_tool_requires_approval if {
	d := eacp.decision with input as pinned(fx.t5_approval_email)
	not d.allow
	count(d.reasons) == 0
	d.require_approval.approver_group == "support-approvers"
	d.require_approval.allow_self == false
}

test_t5_approval_by_group_member_allows if {
	d := eacp.decision with input as pinned(object.union(fx.t5_approval_email, {"approval": {"approved": true, "approver": "bob", "approver_groups": ["support", "support-approvers"]}}))
	d.allow
	d.require_approval == null
}

test_t5_self_approval_is_rejected if {
	d := eacp.decision with input as pinned(object.union(fx.t5_approval_email, {"approval": {"approved": true, "approver": "alice", "approver_groups": ["support", "support-approvers"]}}))
	not d.allow
	d.require_approval.approver_group == "support-approvers"
}

test_t5_approver_outside_group_is_rejected if {
	d := eacp.decision with input as pinned(object.union(fx.t5_approval_email, {"approval": {"approved": true, "approver": "carol", "approver_groups": ["support"]}}))
	not d.allow
}

test_t5_pii_in_exfil_arguments_is_redacted_even_when_approved if {
	d := eacp.decision with input as pinned(object.union(fx.t5_approval_email, {"approval": {"approved": true, "approver": "bob", "approver_groups": ["support-approvers"]}}))
	d.allow
	count(d.redactions) == 1
}

test_t5_destructive_after_untrusted_content_requires_approval_dynamically if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {
		"tool": {"server": "tickets", "name": "close_ticket"},
		"taint": {"touched_untrusted_content": true},
	}))
	not d.allow
	d.require_approval.reason == "destructive action after untrusted content"
	d.require_approval.approver_group == "support-approvers"
}

# --- scenario 6: model class versus label (budgets are enforced in the gateway) ---------------

test_t6_restricted_data_may_not_reach_external_model if {
	d := eacp.decision with input as pinned(fx.t6_prompt_to_external_with_restricted_taint)
	not d.allow
	"model_class_not_allowed_for_label" in codes(d)
}

test_t6_same_prompt_to_in_house_is_fine if {
	d := eacp.decision with input as pinned(object.union(fx.t6_prompt_to_external_with_restricted_taint, {"llm": {"target_class": "inhouse"}}))
	d.allow
}

# --- guardrails: secrets ----------------------------------------------------------------------

test_secret_in_arguments_is_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {"findings": {"secrets": [{"type": "aws_access_key_id", "start": 0, "end": 20}]}}))
	not d.allow
	"secret_in_arguments" in codes(d)
}

test_secret_in_tool_result_is_redacted_not_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {"phase": "response", "findings": {"secrets": [{"type": "github_pat", "start": 5, "end": 45}]}}))
	d.allow
	count(d.redactions) == 1
}

test_pii_to_external_model_is_redacted if {
	d := eacp.decision with input as pinned(object.union(fx.t4_routing_public, {
		"phase": "llm_request",
		"llm": {"target_class": "external"},
		"findings": {"pii": [{"type": "EMAIL_ADDRESS", "start": 0, "end": 10, "score": 0.9}]},
	}))

	# PII makes the label confidential, which the external class may not see at all.
	not d.allow
	"model_class_not_allowed_for_label" in codes(d)
	count(d.redactions) == 1
}

# --- entitlement ------------------------------------------------------------------------------

test_caller_outside_required_groups_is_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {"subject": {"groups": ["marketing"]}}))
	not d.allow
	"group_not_entitled" in codes(d)
}

test_agent_manifest_limits_tools if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {
		"agent": "nightly-triage",
		"subject": {"sub": "service-account-agent-nightly-triage", "clearance": "confidential"},
		"tool": {"server": "customer-db", "name": "search_objects"},
	}))
	not d.allow
	"agent_not_entitled" in codes(d)
}

test_clearance_below_output_classification_is_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {
		"subject": {"sub": "bob", "clearance": "confidential"},
		"tool": {"server": "customer-db", "name": "execute_sql"},
	}))
	not d.allow
	"clearance_insufficient" in codes(d)
}

test_missing_clearance_falls_back_to_team_default if {
	d := eacp.decision with input as pinned(object.union(object.remove(fx.t0_baseline, ["subject"]), {
		"subject": {"sub": "dave", "groups": ["support"], "team": "support"},
		"tool": {"server": "tickets", "name": "get_ticket"},
	}))

	# team default is internal; get_ticket outputs confidential
	not d.allow
	"clearance_insufficient" in codes(d)
}

test_undeclared_tool_is_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {"tool": {"server": "tickets", "name": "delete_everything"}}))
	not d.allow
	"unknown_tool" in codes(d)
}

test_unknown_server_is_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {"tool": {"server": "shadow-it", "name": "run"}}))
	not d.allow
	"unknown_server" in codes(d)
}

test_unknown_agent_is_denied if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {"agent": "rogue"}))
	not d.allow
	"unknown_agent" in codes(d)
}

test_visible_tools_for_interactive_agent if {
	v := eacp.entitlement.visible_tools with input as pinned(object.union(fx.t0_baseline, {"phase": "list"}))
	"tickets/list_tickets" in v
	"customer-db/execute_sql" in v
	"email-send/send_email" in v
	"web-fetch/fetch" in v
}

test_visible_tools_respects_clearance if {
	v := eacp.entitlement.visible_tools with input as pinned(object.union(fx.t0_baseline, {"phase": "list", "subject": {"sub": "bob", "clearance": "confidential"}}))
	"tickets/get_ticket" in v
	not "customer-db/execute_sql" in v
}

test_visible_tools_for_scheduled_agent if {
	v := eacp.entitlement.visible_tools with input as pinned(object.union(fx.t0_baseline, {"phase": "list", "agent": "nightly-triage", "subject": {"groups": ["support"], "clearance": "confidential"}}))
	v == {"tickets/list_tickets", "tickets/get_ticket", "tickets/create_ticket", "tickets/close_ticket"}
}

# --- layers -----------------------------------------------------------------------------------

test_team_layer_restricts_fetch_hosts if {
	d := eacp.decision with input as pinned(object.union(fx.t3_taint_exfil, {
		"taint": {"touched_restricted": false},
		"findings": {"pii": []},
		"arguments": {"url": "https://pastebin.example.net/raw/abc"},
	}))
	not d.allow
	"support_fetch_host_not_allowed" in codes(d)
}

test_tool_layer_restricts_recipient_domain if {
	d := eacp.decision with input as pinned(object.union(fx.t5_approval_email, {
		"arguments": {"to": "someone@gmail.com"},
		"approval": {"approved": true, "approver": "bob", "approver_groups": ["support-approvers"]},
	}))
	not d.allow
	"email_recipient_domain_not_allowed" in codes(d)
}

test_agent_layer_blocks_destructive_for_scheduled_agent if {
	d := eacp.decision with input as pinned(object.union(fx.t0_baseline, {
		"agent": "nightly-triage",
		"subject": {"sub": "service-account-agent-nightly-triage", "clearance": "confidential"},
		"tool": {"server": "tickets", "name": "close_ticket"},
	}))
	not d.allow
	"scheduled_agent_capability_blocked" in codes(d)
}

test_layers_cannot_grant_only_deny if {
	# A layer package exists for support, but a call the global rules deny stays denied.
	d := eacp.decision with input as pinned(object.union(fx.t3_taint_exfil, {"arguments": {"url": "https://docs.example.com/ok"}}))
	not d.allow
	"exfil_after_restricted" in codes(d)
}
