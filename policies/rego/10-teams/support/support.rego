# Team layer for support. Owned by the support team leads (see CODEOWNERS). Can only add
# denials on top of the global rules.
package eacp.teampolicy.support

# Support engineers fetch documentation and status pages; anything else is a policy question.
allowed_fetch_prefixes := {
	"https://docs.example.com/",
	"https://status.example.com/",
	"https://example.com/",
}

deny contains {"code": "support_fetch_host_not_allowed", "msg": sprintf("support may only fetch from %v", [allowed_fetch_prefixes])} if {
	input.phase == "request"
	input.tool.server == "web-fetch"
	url := input.arguments.url
	not prefix_allowed(url)
}

prefix_allowed(url) if {
	some p in allowed_fetch_prefixes
	startswith(url, p)
}
