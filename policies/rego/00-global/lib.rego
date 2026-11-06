# Shared helpers. Pure functions over data.eacp.config; no input access.
package eacp.lib

# Ordinal of a classification label. Undefined for unknown labels, which makes any rule that
# compares an unknown label fail closed.
rank(label) := i if {
	some i, x in data.eacp.config.classification_order
	x == label
}

# True when the two collections share at least one element.
intersects(a, b) if {
	some x in a
	x in b
}

# allowed_tools entries are "server/tool" or "server/*".
tool_allowed(patterns, server, tool) if {
	sprintf("%s/%s", [server, tool]) in patterns
}

tool_allowed(patterns, server, _) if {
	sprintf("%s/*", [server]) in patterns
}

# Highest classification among a collection of labels; "public" when the collection is empty.
max_label(labels) := l if {
	ranks := {rank(x) | some x in labels}
	count(ranks) > 0
	l := data.eacp.config.classification_order[max(ranks)]
}

max_label(labels) := "public" if {
	count({x | some x in labels}) == 0
}
