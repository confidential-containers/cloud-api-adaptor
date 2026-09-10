{{/*
Helper templates for peerpods chart
*/}}

{{/*
Return the appropriate secret name based on secrets.mode:
- "create": Use the chart-managed secret (peer-pods-secret)
- "reference": Use the user-provided existing secret name (validated)
*/}}
{{- define "peerpods.secretName" -}}
{{- if eq .Values.secrets.mode "reference" -}}
{{- .Values.secrets.existingSecretName -}}
{{- else -}}
peer-pods-secret
{{- end -}}
{{- end -}}

{{/*
Return the SSH key secret name for providers that use SSH (libvirt, byom):
- "create": Use the chart-managed secret (ssh-key-secret)
- "reference": Use the user-provided existing secret name (validated)
*/}}
{{- define "peerpods.sshKeySecretName" -}}
{{- if eq .Values.secrets.mode "reference" -}}
{{- .Values.secrets.existingSshKeySecretName -}}
{{- else -}}
ssh-key-secret
{{- end -}}
{{- end -}}

{{/*
Return the TLS secret name for custom certificates:
- "create": Use the chart-managed secret (certs-for-tls)
- "reference": Use the user-provided existing secret name (validated)
*/}}
{{- define "peerpods.tlsSecretName" -}}
{{- if eq .Values.secrets.mode "reference" -}}
{{- .Values.secrets.existingTlsSecretName -}}
{{- else -}}
certs-for-tls
{{- end -}}
{{- end -}}

{{/*
Alibaba Cloud RRSA: mount projected service account token when enabled.
Uses chained `and` (short-circuit) so missing .Values.alibabacloud / .rrsa is safe.
Returns non-empty "true" when the RRSA volume should be rendered.
*/}}
{{- define "peerpods.alibabacloudRrsaEnabled" -}}
{{- if and (eq .Values.provider "alibabacloud") .Values.alibabacloud .Values.alibabacloud.rrsa .Values.alibabacloud.rrsa.enable -}}
true
{{- end -}}
{{- end -}}

{{/*
Check if custom TLS certificates are configured.
Returns "true" when CACERT_FILE is set in providerConfigs for the active
provider AND a TLS secret name is available (either chart-managed or external).
*/}}
{{- define "peerpods.hasTlsCerts" -}}
{{- $config := dict }}
{{- if .Values.providerConfigs }}
{{- $config = index .Values.providerConfigs .Values.provider | default dict }}
{{- end }}
{{- if and (index $config "CACERT_FILE") (include "peerpods.tlsSecretName" .) -}}
true
{{- end -}}
{{- end -}}

{{/*
Return the BYOM SSH host key secret name:
- "create": Use the chart-managed secret (byom-ssh-host-keys), created from
            providerSecrets.byom.hostKeys entries.
- "reference": Use the user-provided existing secret name.
Only meaningful when provider is "byom".
*/}}
{{- define "peerpods.byomHostKeySecretName" -}}
{{- if eq .Values.secrets.mode "reference" -}}
{{- .Values.secrets.existingByomHostKeySecretName -}}
{{- else -}}
byom-ssh-host-keys
{{- end -}}
{{- end -}}

{{/*
Check if BYOM SSH host key allowlist is configured.
Returns "true" when provider is "byom" AND either:
- create mode: providerSecrets.byom.hostKeys is non-empty
- reference mode: existingByomHostKeySecretName is set
When true, the chart mounts the Secret at /etc/byom/ssh-host-keys and
automatically injects SSH_HOST_KEY_ALLOWLIST_DIR pointing to that path.
*/}}
{{- define "peerpods.hasByomHostKeys" -}}
{{- if eq .Values.provider "byom" -}}
  {{- if eq .Values.secrets.mode "reference" -}}
    {{- if .Values.secrets.existingByomHostKeySecretName -}}
true
    {{- end -}}
  {{- else -}}
    {{- $secrets := dict -}}
    {{- if .Values.providerSecrets -}}
      {{- $secrets = index .Values.providerSecrets .Values.provider | default dict -}}
    {{- end -}}
    {{- if index $secrets "hostKeys" -}}
true
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
