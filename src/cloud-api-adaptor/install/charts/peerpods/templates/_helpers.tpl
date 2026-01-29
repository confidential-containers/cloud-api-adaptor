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
Return the SSH key secret name for libvirt:
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
