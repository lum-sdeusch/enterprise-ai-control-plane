# Tool layer for email-send. Owned by the tool's team. Package name is the server name with
# dashes replaced by underscores; the aggregator looks it up that way.
package eacp.toolpolicy.email_send

allowed_recipient_domains := {"example.com", "customer.example"}

deny contains {"code": "email_recipient_domain_not_allowed", "msg": sprintf("recipient domain must be one of %v", [allowed_recipient_domains])} if {
	input.phase == "request"
	input.tool.server == "email-send"
	to := lower(input.arguments.to)
	not domain_allowed(to)
}

domain_allowed(address) if {
	some d in allowed_recipient_domains
	endswith(address, concat("", ["@", d]))
}
