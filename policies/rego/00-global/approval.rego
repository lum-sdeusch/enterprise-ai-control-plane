# Human in the loop: one approval by one human from the tool owner's approver group.
# The adapter validates the approval record (request hash, TTL, approver identity and groups)
# and reports the verified facts in input.approval; this module decides whether they satisfy
# the tool's requirement.
package eacp.approval

server := data.eacp.tools[input.tool.server]

tool := server.tools[input.tool.name]

# Static requirement from the manifest.
required if {
	input.phase == "request"
	tool.approval.mode == "required"
}

# Dynamic requirement: a destructive action after untrusted content entered the conversation
# needs a human even when the manifest does not demand one. This is the prompt-injection
# backstop for write paths.
required if {
	input.phase == "request"
	tool.capability == "destructive"
	input.taint.touched_untrusted_content == true
}

approver_group := g if {
	g := tool.approval.approvers
	g != null
} else := g if {
	g := data.eacp.teams[server.team].approver_groups[0]
} else := "unknown"

default allow_self := false

allow_self := tool.approval.allow_self

satisfied if {
	input.approval.approved == true
	approver_group in input.approval.approver_groups
	input.approval.approver != input.subject.sub
}

satisfied if {
	input.approval.approved == true
	approver_group in input.approval.approver_groups
	allow_self
}

pending if {
	required
	not satisfied
}

reason := "manifest requires approval" if {
	tool.approval.mode == "required"
} else := "destructive action after untrusted content"

spec := {
	"approver_group": approver_group,
	"allow_self": allow_self,
	"reason": reason,
}
