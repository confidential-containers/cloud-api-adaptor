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
{{- required "secrets.existingSecretName is required when secrets.mode is 'reference'" .Values.secrets.existingSecretName -}}
{{- else -}}
peer-pods-secret
{{- end -}}
{{- end -}}
