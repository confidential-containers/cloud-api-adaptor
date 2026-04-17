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
GCP Workload Identity Federation: render the external_account ConfigMap + token
mount when enabled. Only meaningful for provider=gcp; short-circuits if the
block is missing or disabled.
*/}}
{{- define "peerpods.gcpWorkloadIdentityEnabled" -}}
{{- if and (eq .Values.provider "gcp") .Values.gcpWorkloadIdentity .Values.gcpWorkloadIdentity.enabled -}}
true
{{- end -}}
{{- end -}}

{{/*
GCP WIF: resolve the projected-token audience. Defaults to the `audience` value
with the leading "//" stripped (the STS audience format takes "//...", but
Kubernetes projected tokens take the https:// URL form).
*/}}
{{- define "peerpods.gcpWorkloadIdentityTokenAudience" -}}
{{- if .Values.gcpWorkloadIdentity.tokenAudience -}}
{{- .Values.gcpWorkloadIdentity.tokenAudience -}}
{{- else -}}
{{- $aud := .Values.gcpWorkloadIdentity.audience | default "" -}}
{{- if hasPrefix "//" $aud -}}
https:{{ $aud }}
{{- else -}}
{{- $aud -}}
{{- end -}}
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
