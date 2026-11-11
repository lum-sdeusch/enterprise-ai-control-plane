{{/*
Naming and label helpers. The tool name is the Kubernetes name everywhere so that a reader can
go from a manifest to every object it produced with one grep.
*/}}
{{- define "mcp-tool.name" -}}
{{- .Values.metadata.name -}}
{{- end -}}

{{- define "mcp-tool.namespace" -}}
{{- default (printf "team-%s" .Values.metadata.team) .Values.platform.namespaceOverride -}}
{{- end -}}

{{- define "mcp-tool.labels" -}}
app.kubernetes.io/name: {{ include "mcp-tool.name" . }}
app.kubernetes.io/part-of: eacp
app.kubernetes.io/managed-by: {{ .Release.Service }}
eacp.dev/team: {{ .Values.metadata.team }}
eacp.dev/mcp-server: "true"
eacp.dev/transport: {{ .Values.spec.transport }}
{{- end -}}

{{- define "mcp-tool.selectorLabels" -}}
{{- if eq .Values.spec.transport "stdio" -}}
{{ .Values.platform.kmcp.podLabelKey }}: {{ include "mcp-tool.name" . }}
{{- else -}}
app.kubernetes.io/name: {{ include "mcp-tool.name" . }}
{{- end -}}
{{- end -}}

{{/* Container port the MCP endpoint listens on. stdio servers are fronted by kmcp's adapter. */}}
{{- define "mcp-tool.targetPort" -}}
{{- if eq .Values.spec.transport "stdio" -}}
{{- .Values.platform.kmcp.port -}}
{{- else -}}
{{- .Values.spec.http.port -}}
{{- end -}}
{{- end -}}

{{- define "mcp-tool.mcpPath" -}}
{{- if eq .Values.spec.transport "stdio" -}}
{{- .Values.platform.kmcp.path -}}
{{- else -}}
{{- default "/mcp" .Values.spec.http.path -}}
{{- end -}}
{{- end -}}

{{/* The MCP resource identifier (RFC 8707 audience) for this server. */}}
{{- define "mcp-tool.resourceUrl" -}}
{{- printf "%s/mcp/%s" .Values.platform.gateway.publicUrl (include "mcp-tool.name" .) -}}
{{- end -}}

{{/*
The image the workload runs: a laptop-only local build (platform.localImages, filled by
scripts/up.sh from scripts/local-images.sh) wins over the manifest's image.
*/}}
{{- define "mcp-tool.image" -}}
{{- $local := .Values.platform.localImages | default dict -}}
{{- index $local (include "mcp-tool.name" .) | default .Values.spec.image -}}
{{- end -}}

{{/*
"true" when this server can run. With platform.skipPlaceholderImages (set by the Argo CD
ApplicationSet) a manifest still carrying the all-zero placeholder digest renders nothing: the
server is not onboarded until its image is pinned (ticket 05), and a Service with no pods behind
it would make the fail-closed aggregate endpoint fail every session.
*/}}
{{- define "mcp-tool.deployable" -}}
{{- $placeholder := hasSuffix "@sha256:0000000000000000000000000000000000000000000000000000000000000000" (include "mcp-tool.image" .) -}}
{{- if not (and .Values.platform.skipPlaceholderImages $placeholder) -}}true{{- end -}}
{{- end -}}

{{- define "mcp-tool.credentialsSecret" -}}
{{- printf "%s-credentials" (include "mcp-tool.name" .) -}}
{{- end -}}

{{/* CEL authorization expression from requiredGroups. Empty list: any authenticated caller. */}}
{{- define "mcp-tool.groupsCel" -}}
{{- if .Values.spec.requiredGroups -}}
{{- $quoted := list -}}
{{- range .Values.spec.requiredGroups -}}{{- $quoted = append $quoted (printf "%q" .) -}}{{- end -}}
has(jwt.groups) && jwt.groups.exists(g, g in [{{ join ", " $quoted }}])
{{- else -}}
has(jwt.sub)
{{- end -}}
{{- end -}}

{{- define "mcp-tool.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: {{ .Values.platform.podSecurity.runAsUser }}
runAsGroup: {{ .Values.platform.podSecurity.runAsUser }}
fsGroup: {{ .Values.platform.podSecurity.runAsUser }}
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "mcp-tool.containerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: false
capabilities:
  drop: ["ALL"]
{{- end -}}
